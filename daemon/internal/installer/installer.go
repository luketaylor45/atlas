package installer

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/luketaylor45/atlas/daemon/internal/serverfs"
)

type Installer struct {
	Client *client.Client
}

func New(cli *client.Client) *Installer {
	return &Installer{Client: cli}
}

// Request describes one installer run.
type Request struct {
	UUID   string
	Image  string
	Script string
	// Entry is the interpreter the egg asks for ("bash", "ash", ...). Empty
	// means it is derived from the script's shebang.
	Entry string
	Env   []string
	// ScriptDir is where the daemon writes the script, and HostScriptDir is
	// the same directory as the Docker host sees it.
	ScriptDir     string
	HostScriptDir string
	// Output receives the complete installer transcript.
	Output   io.Writer
	Reporter ProgressReporter
}

// ResolveEntry picks the interpreter for an install script. The egg's choice
// wins; otherwise the shebang decides, falling back to POSIX sh.
func ResolveEntry(entry string, script string) string {
	if entry = strings.TrimSpace(entry); entry != "" {
		return entry
	}
	firstLine, _, _ := strings.Cut(strings.TrimLeft(script, "\ufeff \t\r\n"), "\n")
	firstLine = strings.TrimSpace(firstLine)
	if strings.HasPrefix(firstLine, "#!") {
		fields := strings.Fields(strings.TrimPrefix(firstLine, "#!"))
		if len(fields) > 0 {
			// "#!/usr/bin/env bash" names the interpreter as an argument.
			if filepath.Base(fields[0]) == "env" && len(fields) > 1 {
				return fields[1]
			}
			return filepath.Base(fields[0])
		}
	}
	return "sh"
}

// NormalizeScript converts Windows line endings, which make shells fail with
// confusing "not found" errors on otherwise valid scripts.
func NormalizeScript(script string) string {
	script = strings.TrimPrefix(script, "\ufeff")
	script = strings.ReplaceAll(script, "\r\n", "\n")
	script = strings.ReplaceAll(script, "\r", "\n")
	if !strings.HasSuffix(script, "\n") {
		script += "\n"
	}
	return script
}

// Install pulls the installer image and runs the egg's install script in a
// transient container with the service directory mounted at /mnt/server.
// Cancelling ctx stops and removes the container.
func (i *Installer) Install(ctx context.Context, req Request) error {
	log.Printf("[Installer] Starting installation for %s using %s", req.UUID, req.Image)
	report := func(stage string, progress int) {
		if req.Reporter != nil {
			req.Reporter(ProgressUpdate{Stage: stage, Progress: progress})
		}
	}
	output := req.Output
	if output == nil {
		output = io.Discard
	}

	// 1. Pull the installer image.
	report("Pulling installer image", 0)
	fmt.Fprintf(output, "[Atlas] Pulling installer image %s\n", req.Image)
	reader, err := i.Client.ImagePull(ctx, req.Image, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull installer image: %w", err)
	}
	pullErr := ConsumeImagePull(reader, func(progress int) {
		report(fmt.Sprintf("Pulling installer image: %d%%", progress), progress*8/100)
	})
	reader.Close()
	if pullErr != nil {
		return pullErr
	}

	// 2. Prepare the service directory.
	report("Preparing server storage", 8)
	localDataDir := serverfs.LocalServicePath(req.UUID)
	if err := serverfs.EnsureServiceDirectory(localDataDir); err != nil {
		return fmt.Errorf("prepare server data directory: %w", err)
	}
	// The installer runs as root; whatever it leaves behind is handed to the
	// game user, whether or not it succeeded.
	defer func() {
		if err := serverfs.NormalizeTree(localDataDir); err != nil {
			log.Printf("[Installer] Warning: failed to normalize permissions for %s: %v", req.UUID, err)
		}
	}()

	// 3. Write the script to a file, as Pterodactyl-style eggs expect. Passing
	// it as a command-line string breaks on large scripts and on quoting.
	if err := os.MkdirAll(req.ScriptDir, 0700); err != nil {
		return fmt.Errorf("prepare install script directory: %w", err)
	}
	defer os.RemoveAll(req.ScriptDir)
	// The directory is bind-mounted, so it must be traversable by the
	// container's root regardless of the daemon's umask.
	if err := os.Chmod(req.ScriptDir, 0755); err != nil {
		return fmt.Errorf("prepare install script directory: %w", err)
	}
	scriptPath := filepath.Join(req.ScriptDir, "install.sh")
	if err := os.WriteFile(scriptPath, []byte(NormalizeScript(req.Script)), 0755); err != nil {
		return fmt.Errorf("write install script: %w", err)
	}
	if err := os.Chmod(scriptPath, 0755); err != nil {
		return fmt.Errorf("write install script: %w", err)
	}

	env := append([]string{}, req.Env...)
	env = append(env, "SERVER_UUID="+req.UUID, "HOME=/mnt/server")

	entry := ResolveEntry(req.Entry, req.Script)
	containerConfig := &container.Config{
		Image:      req.Image,
		Cmd:        []string{entry, "/mnt/install/install.sh"},
		Entrypoint: []string{},
		Env:        env,
		Tty:        true,
		WorkingDir: "/mnt/server",
		User:       "0:0",
	}
	hostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode("bridge"),
		Mounts: []mount.Mount{
			{Type: mount.TypeBind, Source: serverfs.HostServicePath(req.UUID), Target: "/mnt/server"},
			{Type: mount.TypeBind, Source: req.HostScriptDir, Target: "/mnt/install", ReadOnly: true},
		},
	}

	// 4. Run it.
	report("Starting installer", 10)
	fmt.Fprintf(output, "[Atlas] Running install script with %s\n", entry)
	containerName := ContainerName(req.UUID)
	_ = i.Client.ContainerRemove(ctx, containerName, container.RemoveOptions{Force: true})
	resp, err := i.Client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, containerName)
	if err != nil {
		return fmt.Errorf("failed to create installer container: %w", err)
	}
	// A fresh context: the container must be removed even when ctx was cancelled.
	defer i.Client.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})

	if err := i.Client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start installer container: %w", err)
	}

	statusCh, errCh := i.Client.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)

	logs, err := i.Client.ContainerLogs(ctx, resp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err != nil {
		return fmt.Errorf("failed to stream installer logs: %w", err)
	}
	recentOutput := newLineTailWriter(8)
	streamErr := streamScriptOutput(logs, io.MultiWriter(output, recentOutput), req.Reporter)
	logs.Close()
	if ctx.Err() != nil {
		return context.Cause(ctx)
	}
	if streamErr != nil {
		return fmt.Errorf("failed to read installer logs: %w", streamErr)
	}

	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("error waiting for installer: %w", err)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			if details := recentOutput.String(); details != "" {
				return fmt.Errorf("installer failed with exit code %d; recent output: %s", status.StatusCode, details)
			}
			return fmt.Errorf("installer failed with exit code %d", status.StatusCode)
		}
	}

	if err := serverfs.NormalizeTree(localDataDir); err != nil {
		return fmt.Errorf("normalize installed files: %w", err)
	}
	report("Installation complete", 100)
	return nil
}

const containerPrefix = "install-"

// ContainerName is the name of the transient installer container for a service.
func ContainerName(uuid string) string {
	return containerPrefix + uuid
}

// IsInstallerContainer reports whether a container name belongs to an installer.
func IsInstallerContainer(name string) bool {
	return strings.HasPrefix(strings.TrimPrefix(name, "/"), containerPrefix)
}
