package api

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/luketaylor45/atlas/daemon/internal/docker"
	"github.com/luketaylor45/atlas/daemon/internal/eggconfig"
	"github.com/luketaylor45/atlas/daemon/internal/serverfs"
)

// serverRuntime is what the daemon needs each time a server starts. Core
// sends it with every create, rebuild and power action; the daemon keeps the
// latest copy so a server that restarts on its own (or is started while Core
// is unreachable) still behaves the same way.
type serverRuntime struct {
	// ConfigFiles is the egg's config-file rules, a JSON object.
	ConfigFiles string `json:"config_files"`
	// StartupDone lists console strings that mean startup has finished.
	StartupDone []string `json:"startup_done"`
	StopCommand string   `json:"stop_command"`

	Port        int    `json:"port"`
	Memory      int64  `json:"memory"`
	Environment string `json:"environment"` // JSON object
}

// serverBindAddress is the address games are told to listen on. Inside a
// container that is every interface; Docker publishes the chosen ports.
const serverBindAddress = "0.0.0.0"

func runtimePath(uuid string) string {
	return atlasDir("servers", uuid+".json")
}

func saveRuntime(uuid string, runtime serverRuntime) {
	path := runtimePath(uuid)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		log.Printf("[Daemon] Could not store start settings for %s: %v", uuid, err)
		return
	}
	encoded, err := json.Marshal(runtime)
	if err == nil {
		// Written to a temporary file first so a crash never leaves half a file.
		temporary := path + ".tmp"
		if err = os.WriteFile(temporary, encoded, 0600); err == nil {
			err = os.Rename(temporary, path)
		}
	}
	if err != nil {
		log.Printf("[Daemon] Could not store start settings for %s: %v", uuid, err)
	}
}

func loadRuntime(uuid string) (serverRuntime, bool) {
	var runtime serverRuntime
	data, err := os.ReadFile(runtimePath(uuid))
	if err != nil {
		return runtime, false
	}
	if err := json.Unmarshal(data, &runtime); err != nil {
		return runtime, false
	}
	return runtime, true
}

func removeRuntime(uuid string) {
	_ = os.Remove(runtimePath(uuid))
	readiness.forget(uuid)
}

// dockerInterface is the address a game container reaches its host on, which
// is what {{config.docker.interface}} stands for in egg config files.
func dockerInterface(ctx context.Context) string {
	if docker.Client != nil {
		inspect, err := docker.Client.NetworkInspect(ctx, defaultServerNetwork, network.InspectOptions{})
		if err == nil {
			for _, config := range inspect.IPAM.Config {
				if config.Gateway != "" {
					return config.Gateway
				}
			}
		}
	}
	// The gateway of the default Docker bridge.
	return "172.17.0.1"
}

// applyConfigFiles rewrites the egg's config files with the server's current
// port and variables. It runs before every start, so a changed port or
// variable reaches the game's own files. A file that cannot be rewritten is
// logged and skipped: the server should still start.
func applyConfigFiles(ctx context.Context, uuid string, runtime serverRuntime) {
	files, err := eggconfig.Parse(runtime.ConfigFiles)
	if err != nil {
		log.Printf("[Daemon] Config file rules for %s are invalid and were skipped: %v", uuid, err)
		return
	}
	if len(files) == 0 {
		return
	}

	environment := map[string]string{}
	if strings.TrimSpace(runtime.Environment) != "" {
		_ = json.Unmarshal([]byte(runtime.Environment), &environment)
	}
	// The same values the container itself is given.
	environment["SERVER_PORT"] = strconv.Itoa(runtime.Port)
	environment["SERVER_MEMORY"] = strconv.FormatInt(runtime.Memory, 10)
	environment["SERVER_IP"] = serverBindAddress
	environment["SERVER_UUID"] = uuid

	root := serverfs.LocalServicePath(uuid)
	failures := eggconfig.Apply(root, files, eggconfig.Values{
		Port:      runtime.Port,
		IP:        serverBindAddress,
		Memory:    runtime.Memory,
		UUID:      uuid,
		Interface: dockerInterface(ctx),
		Env:       environment,
	})
	for _, failure := range failures {
		log.Printf("[Daemon] Config file for %s was not updated: %v", uuid, failure)
	}

	// Rewritten files belong to the daemon's user until they are handed back
	// to the game's.
	for _, file := range files {
		handBackPath(root, file.Path)
	}
}

// handBackPath gives a file inside a server directory, and any directories
// leading to it, the ownership and mode the game container needs.
func handBackPath(root string, relative string) {
	clean := filepath.Clean(filepath.FromSlash(relative))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return
	}
	current := root
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return
		}
		if err := serverfs.NormalizePath(current); err != nil {
			log.Printf("[Daemon] Could not set permissions on %s: %v", current, err)
			return
		}
	}
}

// startupLimit is how long a server may stay "starting" before it is shown
// as running anyway. Without it, an egg with a wrong done-string would leave
// its servers starting forever.
const startupLimit = 15 * time.Minute

// readinessTracker follows servers between "process started" and "ready for
// players". Only servers whose egg defines done-strings are tracked.
type readinessTracker struct {
	mu       sync.Mutex
	starting map[string]context.CancelFunc
}

var readiness = &readinessTracker{starting: map[string]context.CancelFunc{}}

// isStarting reports whether a running server is still waiting for its
// done-string.
func (t *readinessTracker) isStarting(uuid string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, ok := t.starting[uuid]
	return ok
}

// forget stops following a server, for example because it stopped.
func (t *readinessTracker) forget(uuid string) {
	t.mu.Lock()
	cancel, ok := t.starting[uuid]
	delete(t.starting, uuid)
	t.mu.Unlock()
	if ok {
		cancel()
	}
}

// begin registers a server as starting. A previous watcher for the same
// server is stopped first.
func (t *readinessTracker) begin(uuid string) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), startupLimit)
	t.mu.Lock()
	previous, had := t.starting[uuid]
	t.starting[uuid] = cancel
	t.mu.Unlock()
	if had {
		previous()
	}
	return ctx
}

// finish marks a server as ready. It returns false when the watcher was
// replaced or the server stopped in the meantime, in which case nothing
// should be reported.
func (t *readinessTracker) finish(uuid string, ctx context.Context) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	cancel, ok := t.starting[uuid]
	if !ok || ctx.Err() == context.Canceled {
		return false
	}
	delete(t.starting, uuid)
	cancel()
	return true
}

// serverStarted is called when a game container's process starts. It reports
// "running" straight away for eggs without done-strings and otherwise
// "starting" until one of them appears in the console.
func serverStarted(uuid string) {
	runtime, _ := loadRuntime(uuid)
	matcher := newDoneMatcher(runtime.StartupDone)
	if matcher == nil {
		NotifyStatus(uuid, "running")
		return
	}

	NotifyStatus(uuid, "starting")
	ctx := readiness.begin(uuid)
	go func() {
		matched := watchForDone(ctx, uuid, matcher)
		if !readiness.finish(uuid, ctx) {
			return
		}
		if !matched {
			// The limit passed or the log stream ended. If the server is
			// still up, showing it as running is the useful answer.
			inspect, err := docker.Client.ContainerInspect(context.Background(), uuid)
			if err != nil || inspect.State == nil || !inspect.State.Running {
				return
			}
			log.Printf("[Daemon] %s did not print its startup message within %s; showing it as running", uuid, startupLimit)
		}
		NotifyStatus(uuid, "running")
	}()
}

// serverStopped is called when a game container's process ends.
func serverStopped(uuid string) {
	readiness.forget(uuid)
	NotifyStatus(uuid, "offline")
}

// doneMatcher decides whether a console line means startup has finished.
type doneMatcher struct {
	plain    []string
	patterns []*regexp.Regexp
}

// newDoneMatcher returns nil when there is nothing to match.
func newDoneMatcher(done []string) *doneMatcher {
	matcher := &doneMatcher{}
	for _, entry := range done {
		if pattern, isRegex := strings.CutPrefix(entry, "regex:"); isRegex {
			compiled, err := regexp.Compile(strings.TrimSpace(pattern))
			if err != nil {
				log.Printf("[Daemon] Ignoring invalid startup pattern %q: %v", pattern, err)
				continue
			}
			matcher.patterns = append(matcher.patterns, compiled)
			continue
		}
		if strings.TrimSpace(entry) != "" {
			matcher.plain = append(matcher.plain, entry)
		}
	}
	if len(matcher.plain) == 0 && len(matcher.patterns) == 0 {
		return nil
	}
	return matcher
}

// ansiSequence matches terminal colour and cursor codes, which would
// otherwise sit in the middle of the text being looked for.
var ansiSequence = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b[@-Z\\-_]`)

func (m *doneMatcher) matches(line string) bool {
	line = ansiSequence.ReplaceAllString(line, "")
	for _, text := range m.plain {
		if strings.Contains(line, text) {
			return true
		}
	}
	for _, pattern := range m.patterns {
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
}

// watchForDone follows a server's console from its latest start until a line
// matches. It returns false when the context ends or the output stops first.
func watchForDone(ctx context.Context, uuid string, matcher *doneMatcher) bool {
	inspect, err := docker.Client.ContainerInspect(ctx, uuid)
	if err != nil || inspect.State == nil {
		return false
	}

	reader, err := docker.Client.ContainerLogs(ctx, uuid, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		// Only output of this run: an earlier run's "Done" must not count.
		Since: inspect.State.StartedAt,
	})
	if err != nil {
		return false
	}
	defer reader.Close()

	var stream io.Reader = reader
	if inspect.Config == nil || !inspect.Config.Tty {
		// Without a TTY, Docker interleaves stdout and stderr with headers.
		pipeReader, pipeWriter := io.Pipe()
		go func() {
			_, copyErr := stdcopy.StdCopy(pipeWriter, pipeWriter, reader)
			pipeWriter.CloseWithError(copyErr)
		}()
		defer pipeReader.Close()
		stream = pipeReader
	}

	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	// Progress output redraws a line with carriage returns; each redraw is
	// treated as a line of its own.
	scanner.Split(scanLinesOrReturns)
	for scanner.Scan() {
		if matcher.matches(scanner.Text()) {
			return true
		}
	}
	return false
}

// scanLinesOrReturns splits on "\n" and on "\r".
func scanLinesOrReturns(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i, b := range data {
		if b == '\n' || b == '\r' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}
