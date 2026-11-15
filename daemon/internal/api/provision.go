package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/errdefs"
	"github.com/docker/go-connections/nat"
	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/daemon/internal/docker"
	"github.com/luketaylor45/atlas/daemon/internal/installer"
	"github.com/luketaylor45/atlas/daemon/internal/serverfs"
)

const (
	installedMarker     = ".atlas_installed"
	installFailedMarker = ".atlas_install_failed"
	permissionsMarker   = ".atlas_permissions_v1"
	startScriptName     = "start.sh"

	defaultInstallImage = "ghcr.io/pterodactyl/installers:alpine"

	// gameTmpfsOptions sizes the in-memory /tmp of game containers. It counts
	// towards the container's memory limit only as far as it is used.
	gameTmpfsOptions = "rw,exec,nosuid,size=100m"
)

// PortMapping is an additional port published for a service besides its
// primary one (for example a query or RCON port).
type PortMapping struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"` // "tcp", "udp" or "both" (default)
}

// ServerSpec is everything the daemon needs to build a service's container.
type ServerSpec struct {
	UUID             string        `json:"uuid"`
	Memory           int64         `json:"memory"`
	Disk             int64         `json:"disk"`
	Cpu              int64         `json:"cpu"`
	Port             int           `json:"port"`
	Ports            []PortMapping `json:"ports"`
	EggImage         string        `json:"egg_image"`
	StartupCommand   string        `json:"startup_command"`
	Environment      string        `json:"environment"` // JSON object
	InstallScript    string        `json:"install_script"`
	InstallContainer string        `json:"install_container"`
	InstallEntry     string        `json:"install_entry"`

	// What is needed again at every start; see serverRuntime.
	ConfigFiles string   `json:"config_files"`
	StartupDone []string `json:"startup_done"`
	StopCommand string   `json:"stop_command"`
}

func (s ServerSpec) runtime() serverRuntime {
	return serverRuntime{
		ConfigFiles: s.ConfigFiles,
		StartupDone: s.StartupDone,
		StopCommand: s.StopCommand,
		Port:        s.Port,
		Memory:      s.Memory,
		Environment: s.Environment,
	}
}

// CreateServerRequest is kept as the name of the create payload.
type CreateServerRequest = ServerSpec

func validPort(port int) bool {
	return port >= 1 && port <= 65535
}

func (s ServerSpec) validate() error {
	if !serverfs.IsServiceDirectoryName(s.UUID) {
		return errors.New("invalid service identifier")
	}
	if strings.TrimSpace(s.EggImage) == "" {
		return errors.New("missing game server image")
	}
	if !validPort(s.Port) {
		return fmt.Errorf("port %d is out of range", s.Port)
	}
	if _, err := s.portBindings(); err != nil {
		return err
	}
	if _, err := parseEnvironment(s.Environment); err != nil {
		return err
	}
	return nil
}

// portBindings builds the published port set. The primary port is published
// on both protocols because games differ in which one they use.
func (s ServerSpec) portBindings() (nat.PortMap, error) {
	bindings := nat.PortMap{}
	add := func(port int, protocol string) {
		key := nat.Port(fmt.Sprintf("%d/%s", port, protocol))
		bindings[key] = []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: strconv.Itoa(port)}}
	}

	add(s.Port, "tcp")
	add(s.Port, "udp")

	for _, extra := range s.Ports {
		if !validPort(extra.Port) {
			return nil, fmt.Errorf("port %d is out of range", extra.Port)
		}
		switch strings.ToLower(strings.TrimSpace(extra.Protocol)) {
		case "tcp":
			add(extra.Port, "tcp")
		case "udp":
			add(extra.Port, "udp")
		case "", "both":
			add(extra.Port, "tcp")
			add(extra.Port, "udp")
		default:
			return nil, fmt.Errorf("port %d has unknown protocol %q", extra.Port, extra.Protocol)
		}
	}
	return bindings, nil
}

func parseEnvironment(environment string) ([]string, error) {
	if strings.TrimSpace(environment) == "" {
		return nil, nil
	}

	values := make(map[string]string)
	if err := json.Unmarshal([]byte(environment), &values); err != nil {
		return nil, fmt.Errorf("parse server environment: %w", err)
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]string, 0, len(values))
	for _, key := range keys {
		result = append(result, fmt.Sprintf("%s=%s", key, values[key]))
	}
	return result, nil
}

// CreateServer starts provisioning a new service.
func CreateServer(c *gin.Context) {
	var spec ServerSpec
	if err := c.ShouldBindJSON(&spec); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := spec.validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !startProvision(spec, false) {
		c.JSON(http.StatusConflict, gin.H{"error": "An installation is already running for this server"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "provisioning"})
}

// HandleReinstall wipes the game files and installs the service again.
func HandleReinstall(c *gin.Context) {
	var spec ServerSpec
	if err := c.ShouldBindJSON(&spec); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload: " + err.Error()})
		return
	}
	spec.UUID = c.Param("uuid")
	if err := spec.validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if backups.restoring(spec.UUID) {
		c.JSON(http.StatusConflict, gin.H{"error": "A backup is being restored to this server. Wait for it to finish."})
		return
	}
	if !startProvision(spec, true) {
		c.JSON(http.StatusConflict, gin.H{"error": "An installation is already running for this server"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "reinstall_triggered"})
}

// HandleCancelInstall stops a running installation.
func HandleCancelInstall(c *gin.Context) {
	if !installs.cancel(c.Param("uuid")) {
		c.JSON(http.StatusConflict, gin.H{"error": "No installation is running for this server"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "cancelling"})
}

// HandleInstallLog returns the installer transcript from a byte offset, so
// the panel can follow it incrementally.
func HandleInstallLog(c *gin.Context) {
	uuid := c.Param("uuid")
	offset, _ := strconv.ParseInt(c.Query("offset"), 10, 64)

	content, next, err := readInstallLog(uuid, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read the installation log"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"content": content,
		"offset":  next,
		"running": installs.active(uuid),
	})
}

// startProvision runs one provisioning job in the background. It returns
// false when the service already has a job running.
func startProvision(spec ServerSpec, wipe bool) bool {
	ctx, finish, ok := installs.begin(spec.UUID, installTimeout())
	if !ok {
		return false
	}

	go func() {
		defer finish()

		transcript, err := openInstallLog(spec.UUID)
		if err != nil {
			// Installing without a transcript is still better than not installing.
			log.Printf("[Daemon] Install log unavailable for %s: %v", spec.UUID, err)
		}
		defer transcript.Close()

		queued := "Provisioning queued"
		if wipe {
			queued = "Reinstallation queued"
		}
		NotifyInstallationProgress(spec.UUID, queued, 0)

		err = runProvision(ctx, spec, wipe, transcript)
		if err == nil {
			return
		}

		// Report why the job stopped, not the low-level error it caused.
		if cause := context.Cause(ctx); cause != nil {
			switch {
			case errors.Is(cause, errInstallCancelled):
				err = errInstallCancelled
			case errors.Is(cause, errInstallTimedOut):
				err = fmt.Errorf("installation exceeded the %s limit and was stopped", installTimeout())
			}
		}

		log.Printf("[Daemon] Provisioning FAILED for %s: %v", spec.UUID, err)
		transcript.Printf("Installation failed: %v", err)
		NotifyInstallationFailure(spec.UUID, err)
		_ = serverfs.WriteFile(filepath.Join(serverfs.LocalServicePath(spec.UUID), installFailedMarker), []byte(err.Error()), 0660)
	}()
	return true
}

func runProvision(ctx context.Context, spec ServerSpec, wipe bool, transcript *installLog) error {
	dataDir := serverfs.LocalServicePath(spec.UUID)
	if err := serverfs.EnsureServiceDirectory(dataDir); err != nil {
		return fmt.Errorf("create server data directory: %w", err)
	}

	// Any previous container is replaced. A first install has none.
	if err := removeGameContainer(ctx, spec.UUID); err != nil {
		return err
	}
	if wipe {
		transcript.Printf("Removing existing server files")
		if err := wipeServiceFiles(dataDir); err != nil {
			return err
		}
	}
	_ = os.Remove(filepath.Join(dataDir, installFailedMarker))

	progress := func(stage string, percent int) {
		NotifyInstallationProgress(spec.UUID, stage, percent)
	}

	transcript.Printf("Pulling game server image %s", spec.EggImage)
	progress("Pulling game server image", 1)
	if err := pullImage(ctx, spec.EggImage, func(percent int) {
		progress(fmt.Sprintf("Pulling game server image: %d%%", percent), 1+(percent*9/100))
	}); err != nil {
		return fmt.Errorf("pull game server image: %w", err)
	}

	environment, err := parseEnvironment(spec.Environment)
	if err != nil {
		return err
	}

	installed := strings.TrimSpace(spec.InstallScript) != ""
	if installed {
		installImage := spec.InstallContainer
		if installImage == "" {
			installImage = defaultInstallImage
		}

		err := installer.New(docker.Client).Install(ctx, installer.Request{
			UUID:          spec.UUID,
			Image:         installImage,
			Script:        spec.InstallScript,
			Entry:         spec.InstallEntry,
			Env:           environment,
			ScriptDir:     atlasDir("install", spec.UUID),
			HostScriptDir: atlasHostDir("install", spec.UUID),
			Output:        transcript,
			Reporter: func(update installer.ProgressUpdate) {
				progress(update.Stage, 10+(update.Progress*85/100))
			},
		})
		if err != nil {
			return err
		}
	}

	progress("Creating game server container", 98)
	transcript.Printf("Creating game server container")
	containerID, err := createGameContainer(ctx, spec)
	if err != nil {
		return err
	}

	if installed {
		if err := serverfs.WriteFile(filepath.Join(dataDir, installedMarker), []byte(time.Now().Format(time.RFC3339)), 0660); err != nil {
			return fmt.Errorf("write installation marker: %w", err)
		}
		transcript.Printf("Installation complete")
		progress("Installation complete", 100)
		NotifyStatus(spec.UUID, "offline")
		log.Printf("[Daemon] Installation SUCCEEDED for %s", spec.UUID)
		return nil
	}

	// Eggs without an install script run straight from their image.
	NotifyStatus(spec.UUID, "starting")
	applyConfigFiles(ctx, spec.UUID, spec.runtime())
	if err := docker.Client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("start game server container: %w", err)
	}
	return nil
}

func pullImage(ctx context.Context, ref string, report func(int)) error {
	reader, err := docker.Client.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()
	return installer.ConsumeImagePull(reader, report)
}

// removeGameContainer stops and removes a service's container if it exists.
func removeGameContainer(ctx context.Context, uuid string) error {
	timeout := 10
	if err := docker.Client.ContainerStop(ctx, uuid, container.StopOptions{Timeout: &timeout}); err != nil && !errdefs.IsNotFound(err) {
		log.Printf("[Daemon] Stop before replace warning for %s: %v", uuid, err)
	}
	if err := docker.Client.ContainerRemove(ctx, uuid, container.RemoveOptions{Force: true}); err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("remove existing game container: %w", err)
	}
	return nil
}

// wipeServiceFiles removes game files and installer caches while keeping the
// files Atlas manages itself.
func wipeServiceFiles(dataDir string) error {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return fmt.Errorf("read server files: %w", err)
	}
	for _, entry := range entries {
		if entry.Name() == startScriptName || entry.Name() == permissionsMarker {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dataDir, entry.Name())); err != nil {
			return fmt.Errorf("remove server files: %w", err)
		}
	}
	return nil
}

// createGameContainer writes the start script and creates (but does not
// start) the service's container from its spec.
func createGameContainer(ctx context.Context, spec ServerSpec) (string, error) {
	if err := writeStartScript(spec.UUID, spec.StartupCommand, spec.Port, spec.Memory, spec.Environment); err != nil {
		return "", err
	}
	saveRuntime(spec.UUID, spec.runtime())

	bindings, err := spec.portBindings()
	if err != nil {
		return "", err
	}
	exposed := nat.PortSet{}
	for port := range bindings {
		exposed[port] = struct{}{}
	}

	environment, err := parseEnvironment(spec.Environment)
	if err != nil {
		return "", err
	}
	// Atlas-defined values come last so an egg variable can never shadow them.
	containerEnvironment := append([]string{}, environment...)
	containerEnvironment = append(containerEnvironment,
		"STARTUP=bash "+startScriptName,
		fmt.Sprintf("SERVER_MEMORY=%d", spec.Memory),
		fmt.Sprintf("SERVER_PORT=%d", spec.Port),
		"SERVER_IP="+serverBindAddress,
		"SERVER_UUID="+spec.UUID,
	)

	resp, err := docker.Client.ContainerCreate(ctx, &container.Config{
		Image:        spec.EggImage,
		Tty:          true,
		OpenStdin:    true,
		Env:          containerEnvironment,
		ExposedPorts: exposed,
		User:         serverfs.ContainerUser(),
	}, &container.HostConfig{
		NetworkMode:  container.NetworkMode(defaultServerNetwork),
		PortBindings: bindings,
		// A private, writable /tmp, as Pterodactyl's Wings gives every game
		// container. Many game images write a passwd entry for the
		// unprivileged user there at start; without it Unity games such as
		// Valheim crash before they load.
		Tmpfs: map[string]string{"/tmp": gameTmpfsOptions},
		Resources: container.Resources{
			Memory:   spec.Memory * 1024 * 1024,
			NanoCPUs: spec.Cpu * 10000000,
		},
		Mounts: []mount.Mount{{
			Type:   mount.TypeBind,
			Source: serverfs.HostServicePath(spec.UUID),
			Target: "/home/container",
		}},
	}, nil, nil, spec.UUID)
	if err != nil {
		return "", fmt.Errorf("create game server container: %w", err)
	}
	return resp.ID, nil
}

// HandleRebuild recreates a service's container from an updated spec (new
// ports, image or limits) without touching its files.
func HandleRebuild(c *gin.Context) {
	var spec ServerSpec
	if err := c.ShouldBindJSON(&spec); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload: " + err.Error()})
		return
	}
	spec.UUID = c.Param("uuid")
	if err := spec.validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Holding the install slot keeps a rebuild and an install from racing.
	ctx, finish, ok := installs.begin(spec.UUID, 15*time.Minute)
	if !ok {
		c.JSON(http.StatusConflict, gin.H{"error": "An installation is running for this server"})
		return
	}
	defer finish()

	wasRunning := false
	if inspect, err := docker.Client.ContainerInspect(ctx, spec.UUID); err == nil && inspect.State != nil {
		wasRunning = inspect.State.Running
	}

	// Pull before removing anything, so a bad image leaves the server intact.
	if err := pullImage(ctx, spec.EggImage, nil); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to pull game server image: " + err.Error()})
		return
	}

	if wasRunning {
		NotifyStatus(spec.UUID, "stopping")
		if err := stopGameContainer(ctx, spec.UUID, c.Query("stop_command")); err != nil && !errdefs.IsNotFound(err) {
			log.Printf("[Daemon] Rebuild stop warning for %s: %v", spec.UUID, err)
		}
	}
	if err := removeGameContainer(ctx, spec.UUID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	containerID, err := createGameContainer(ctx, spec)
	if err != nil {
		NotifyStatus(spec.UUID, "offline")
		c.JSON(http.StatusInternalServerError, gin.H{"error": powerActionErrorMessage("rebuild", spec.Port, err)})
		return
	}

	if wasRunning {
		NotifyStatus(spec.UUID, "starting")
		applyConfigFiles(ctx, spec.UUID, spec.runtime())
		if err := docker.Client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
			reconcileStatusAfterPowerFailure(ctx, spec.UUID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": powerActionErrorMessage("start", spec.Port, err)})
			return
		}
	} else {
		NotifyStatus(spec.UUID, "offline")
	}

	c.JSON(http.StatusOK, gin.H{"status": "rebuilt", "restarted": wasRunning})
}

// gracefulStopWait is how long a server gets to save and exit after its stop
// command before the container is stopped by signal.
const gracefulStopWait = 30 * time.Second

// stopGameContainer stops a server the way its egg asks: by sending the
// game's own stop command ("stop", "quit", ...) or, for "^C", an interrupt,
// and only then falling back to Docker's stop. Stopping by signal alone can
// cost unsaved world data.
func stopGameContainer(ctx context.Context, uuid string, stopCommand string) error {
	stopCommand = strings.TrimSpace(stopCommand)
	if stopCommand != "" {
		var err error
		if stopCommand == "^C" {
			err = docker.Client.ContainerKill(ctx, uuid, "SIGINT")
		} else {
			err = sendConsoleCommand(ctx, uuid, stopCommand)
		}
		if err == nil && waitForExit(ctx, uuid, gracefulStopWait) {
			return nil
		}
	}

	timeout := 10
	return docker.Client.ContainerStop(ctx, uuid, container.StopOptions{Timeout: &timeout})
}

func waitForExit(ctx context.Context, uuid string, limit time.Duration) bool {
	waitCtx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()

	statusCh, errCh := docker.Client.ContainerWait(waitCtx, uuid, container.WaitConditionNotRunning)
	select {
	case <-statusCh:
		return true
	case <-errCh:
		return false
	case <-waitCtx.Done():
		return false
	}
}

var errServerNotRunning = errors.New("the server is not running")

// sendConsoleCommand writes one line to the server's standard input.
func sendConsoleCommand(ctx context.Context, uuid string, command string) error {
	// Docker happily attaches to a stopped container and accepts input that
	// nothing will ever read, so the state is checked first.
	inspect, err := docker.Client.ContainerInspect(ctx, uuid)
	if err != nil {
		return err
	}
	if inspect.State == nil || !inspect.State.Running {
		return errServerNotRunning
	}

	resp, err := docker.Client.ContainerAttach(ctx, uuid, container.AttachOptions{Stream: true, Stdin: true})
	if err != nil {
		return err
	}
	defer resp.Close()

	_, err = fmt.Fprintf(resp.Conn, "%s\n", command)
	return err
}

// HandleListServers reports what actually exists on this node so Core can
// correct any status that drifted (lost callbacks, a restarted daemon).
func HandleListServers(c *gin.Context) {
	containers, err := docker.Client.ContainerList(c.Request.Context(), container.ListOptions{All: true})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list containers: " + err.Error()})
		return
	}

	servers := make(map[string]string)
	for _, item := range containers {
		for _, name := range item.Names {
			name = strings.TrimPrefix(name, "/")
			if !serverfs.IsServiceDirectoryName(name) {
				continue
			}
			if item.State == "running" || item.State == "restarting" {
				servers[name] = "running"
				if readiness.isStarting(name) {
					servers[name] = "starting"
				}
			} else {
				servers[name] = "stopped"
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"servers":    servers,
		"installing": installs.list(),
		"backups":    backups.list(),
	})
}

// CleanupInterruptedInstalls removes installer containers left behind by a
// daemon that stopped mid-install. Their services are reported as failed by
// Core's reconciler, which sees no install job and no game container.
func CleanupInterruptedInstalls(ctx context.Context) {
	containers, err := docker.Client.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		log.Printf("[Daemon] Could not check for interrupted installations: %v", err)
		return
	}
	for _, item := range containers {
		for _, name := range item.Names {
			if !installer.IsInstallerContainer(name) {
				continue
			}
			log.Printf("[Daemon] Removing interrupted installer container %s", name)
			if err := docker.Client.ContainerRemove(ctx, item.ID, container.RemoveOptions{Force: true}); err != nil {
				log.Printf("[Daemon] Failed to remove %s: %v", name, err)
			}
		}
	}
	_ = os.RemoveAll(atlasDir("install"))
}
