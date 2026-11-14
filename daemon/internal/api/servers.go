package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/luketaylor45/atlas/daemon/internal/config"
	"github.com/luketaylor45/atlas/daemon/internal/docker"
	"github.com/luketaylor45/atlas/daemon/internal/installer"
	"github.com/luketaylor45/atlas/daemon/internal/serverfs"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

const defaultServerNetwork = "bridge"

type CreateServerRequest struct {
	UUID             string `json:"uuid"`
	Memory           int64  `json:"memory"`
	Disk             int64  `json:"disk"`
	Cpu              int64  `json:"cpu"`
	Port             int    `json:"port"`
	EggImage         string `json:"egg_image"`
	StartupCommand   string `json:"startup_command"`
	Environment      string `json:"environment"` // JSON string
	InstallScript    string `json:"install_script"`
	InstallContainer string `json:"install_container"`
}

func CreateServer(c *gin.Context) {
	var req CreateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !serverfs.IsServiceDirectoryName(req.UUID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service identifier"})
		return
	}

	log.Printf("Received Create Server Request: %s (%s)", req.UUID, req.EggImage)
	c.JSON(http.StatusAccepted, gin.H{"status": "provisioning"})

	go func() {
		NotifyInstallationProgress(req.UUID, "Provisioning queued", 0)
		if err := provisionServer(context.Background(), req); err != nil {
			log.Printf("[Daemon] Provisioning FAILED for %s: %v", req.UUID, err)
			NotifyInstallationFailure(req.UUID, err)
			dataDir := serverfs.LocalServicePath(req.UUID)
			_ = serverfs.WriteFile(filepath.Join(dataDir, ".atlas_install_failed"), []byte(err.Error()), 0660)
		}
	}()
}

func provisionServer(ctx context.Context, req CreateServerRequest) error {
	dataDir := serverfs.LocalServicePath(req.UUID)
	if err := serverfs.EnsureServiceDirectory(dataDir); err != nil {
		return fmt.Errorf("create server data directory: %w", err)
	}
	_ = os.Remove(filepath.Join(dataDir, ".atlas_install_failed"))

	log.Printf("[Daemon] Pulling game image %s for %s", req.EggImage, req.UUID)
	NotifyInstallationProgress(req.UUID, "Pulling game server image", 1)
	reader, err := docker.Client.ImagePull(ctx, req.EggImage, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull game server image: %w", err)
	}
	if err := installer.ConsumeImagePull(reader, func(progress int) {
		NotifyInstallationProgress(req.UUID, fmt.Sprintf("Pulling game server image: %d%%", progress), 1+(progress*9/100))
	}); err != nil {
		reader.Close()
		return err
	}
	if err := reader.Close(); err != nil {
		return fmt.Errorf("close game image pull: %w", err)
	}

	environment, err := parseEnvironment(req.Environment)
	if err != nil {
		return err
	}

	if req.InstallScript != "" {
		log.Printf("[Daemon] Starting installation for %s", req.UUID)
		installImage := req.InstallContainer
		if installImage == "" {
			installImage = "ghcr.io/pterodactyl/installers:alpine"
		}

		inst := installer.New(docker.Client)
		if err := inst.Install(ctx, req.UUID, installImage, req.InstallScript, environment, func(update installer.ProgressUpdate) {
			progress := 10 + (update.Progress * 85 / 100)
			NotifyInstallationProgress(req.UUID, update.Stage, progress)
		}); err != nil {
			return err
		}
	}

	NotifyInstallationProgress(req.UUID, "Creating game server container", 98)
	if err := writeStartScript(req.UUID, req.StartupCommand, req.Port, req.Memory, req.Environment); err != nil {
		return err
	}

	cPortUDP := nat.Port(fmt.Sprintf("%d/udp", req.Port))
	cPortTCP := nat.Port(fmt.Sprintf("%d/tcp", req.Port))
	hostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(defaultServerNetwork),
		PortBindings: nat.PortMap{
			cPortUDP: []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", req.Port)}},
			cPortTCP: []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", req.Port)}},
		},
		Resources: container.Resources{
			Memory:   req.Memory * 1024 * 1024,
			NanoCPUs: req.Cpu * 10000000,
		},
		Mounts: []mount.Mount{{
			Type:   mount.TypeBind,
			Source: serverfs.HostServicePath(req.UUID),
			Target: "/home/container",
		}},
	}

	containerEnvironment := []string{
		"STARTUP=bash start.sh",
		fmt.Sprintf("SERVER_MEMORY=%d", req.Memory),
		fmt.Sprintf("SERVER_PORT=%d", req.Port),
		"SERVER_UUID=" + req.UUID,
	}
	containerEnvironment = append(containerEnvironment, environment...)

	resp, err := docker.Client.ContainerCreate(ctx, &container.Config{
		Image:     req.EggImage,
		Tty:       true,
		OpenStdin: true,
		Env:       containerEnvironment,
		User:      serverfs.ContainerUser(),
	}, hostConfig, nil, nil, req.UUID)
	if err != nil {
		return fmt.Errorf("create game server container: %w", err)
	}

	if req.InstallScript != "" {
		if err := serverfs.WriteFile(filepath.Join(dataDir, ".atlas_installed"), []byte(time.Now().Format(time.RFC3339)), 0660); err != nil {
			return fmt.Errorf("write installation marker: %w", err)
		}
		NotifyInstallationProgress(req.UUID, "Installation complete", 100)
		NotifyStatus(req.UUID, "offline")
		log.Printf("[Daemon] Installation SUCCEEDED for %s", req.UUID)
		return nil
	}

	NotifyStatus(req.UUID, "starting")
	if err := docker.Client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("start game server container: %w", err)
	}
	return nil
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

type UpdateServerRequest struct {
	Memory         uint64 `json:"memory"`
	Disk           uint64 `json:"disk"`
	Cpu            uint64 `json:"cpu"`
	Port           int    `json:"port"`
	DockerImage    string `json:"docker_image"`
	StartupCommand string `json:"startup_command"`
	Environment    string `json:"environment"`
}

func UpdateServer(c *gin.Context) {
	token := c.GetHeader("X-Node-Token")
	if token != config.NodeConfig.NodeToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	uuid := c.Param("uuid")
	var req UpdateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	// 1. Update Container Resources (Live)
	updateConfig := container.UpdateConfig{
		Resources: container.Resources{
			Memory:   int64(req.Memory) * 1024 * 1024,
			NanoCPUs: int64(req.Cpu) * 10000000,
		},
	}

	if _, err := docker.Client.ContainerUpdate(ctx, uuid, updateConfig); err != nil {
		log.Printf("[Daemon] Warning: Failed to update live container resources for %s: %v", uuid, err)
	}

	// 2. Regenerate Start Script
	if err := writeStartScript(uuid, req.StartupCommand, req.Port, int64(req.Memory), req.Environment); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 3. We can't change the Image of a running container without recreating it.
	// But we can update the record. Recreations should happen on Reinstall or manual rebuild.
	// For now, we update scripts and resources.

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

type ReinstallRequest struct {
	InstallScript    string `json:"install_script"`
	InstallContainer string `json:"install_container"`
	Environment      string `json:"environment"`
	Memory           int64  `json:"memory"`
	Disk             int64  `json:"disk"`
	Cpu              int64  `json:"cpu"`
	Port             int    `json:"port"`
	EggImage         string `json:"egg_image"`
	StartupCommand   string `json:"startup_command"`
}

func (r ReinstallRequest) provisionRequest(uuid string) CreateServerRequest {
	return CreateServerRequest{
		UUID:             uuid,
		Memory:           r.Memory,
		Disk:             r.Disk,
		Cpu:              r.Cpu,
		Port:             r.Port,
		EggImage:         r.EggImage,
		StartupCommand:   r.StartupCommand,
		Environment:      r.Environment,
		InstallScript:    r.InstallScript,
		InstallContainer: r.InstallContainer,
	}
}

// HandleReinstall wipes game files to trigger a fresh install
func HandleReinstall(c *gin.Context) {
	if c.GetHeader("X-Node-Token") != config.NodeConfig.NodeToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	uuid := c.Param("uuid")

	var req ReinstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload: " + err.Error()})
		return
	}

	if _, err := parseEnvironment(req.Environment); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.EggImage) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing game server image for reinstall"})
		return
	}

	dataDir := serverfs.LocalServicePath(uuid)
	if err := serverfs.EnsureServiceDirectory(dataDir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare server files: " + err.Error()})
		return
	}

	ctx := context.Background()

	// Remove the old game container if it exists. A failed first install has no
	// container yet, which is a valid reinstall starting point.
	if err := docker.Client.ContainerStop(ctx, uuid, container.StopOptions{}); err != nil && !errdefs.IsNotFound(err) {
		log.Printf("[Daemon] Reinstall stop warning for %s: %v", uuid, err)
	}
	if err := docker.Client.ContainerRemove(ctx, uuid, container.RemoveOptions{Force: true}); err != nil && !errdefs.IsNotFound(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to replace existing game container: " + err.Error()})
		return
	}

	// Wipe game files and installer caches while preserving Atlas-managed files.
	files, err := os.ReadDir(dataDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read server files: " + err.Error()})
		return
	}
	for _, f := range files {
		if f.Name() == "start.sh" || f.Name() == ".atlas_permissions_v1" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dataDir, f.Name())); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove server files: " + err.Error()})
			return
		}
	}

	NotifyInstallationProgress(uuid, "Reinstallation queued", 0)

	provisionReq := req.provisionRequest(uuid)
	go func() {
		log.Printf("[Daemon] Starting background re-provisioning for %s", uuid)
		if err := provisionServer(context.Background(), provisionReq); err != nil {
			log.Printf("[Daemon] Re-provisioning FAILED for %s: %v", uuid, err)
			NotifyInstallationFailure(uuid, err)
			_ = serverfs.WriteFile(filepath.Join(dataDir, ".atlas_install_failed"), []byte(err.Error()), 0660)
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{"status": "reinstall_triggered"})
}

func writeStartScript(uuid string, startupCmd string, port int, memory int64, environment string) error {
	dataDir := serverfs.LocalServicePath(uuid)
	if err := serverfs.EnsureDirectory(dataDir); err != nil {
		return fmt.Errorf("prepare startup script directory: %w", err)
	}

	// Create a map for all available replacements
	replacements := map[string]string{
		"SERVER_PORT":   fmt.Sprintf("%d", port),
		"SERVER_MEMORY": fmt.Sprintf("%d", memory),
	}

	// Add environment variables to the map
	if environment != "" {
		var envVars map[string]string
		if err := json.Unmarshal([]byte(environment), &envVars); err == nil {
			for k, v := range envVars {
				replacements[k] = v
			}
		}
	}

	// Use regex to find all {{VARIABLE}} and replace them
	re := regexp.MustCompile(`{{([A-Za-z0-9_.]+)}}`)
	processedCmd := re.ReplaceAllStringFunc(startupCmd, func(match string) string {
		// Extract the key name (e.g. SERVER_PORT from {{SERVER_PORT}})
		key := match[2 : len(match)-2]
		if val, ok := replacements[key]; ok {
			return val
		}
		// If key not found in replacements, return empty string
		return ""
	})

	log.Printf("[Daemon] Processed Startup Command for %s: %s", uuid, processedCmd)

	// Generate the actual start.sh content
	// The game container never receives node credentials: the daemon reports
	// status itself from Docker events.
	startScript := fmt.Sprintf(`#!/bin/bash
echo "--- Atlas Instance Wrapper ---"
echo "Working Directory: $(pwd)"
echo "Environment: Port=%d, Memory=%dMB"

echo "Starting Server..."
# Anchor CWD to game root
cd "/home/container"

# Run the startup command directly
%s
exit_code=$?

if [ $exit_code -ne 0 ]; then
  echo "[Atlas] Server process exited unexpectedly with code $exit_code"
fi

exit $exit_code
`, port, memory, processedCmd)

	// Normalize line endings for Linux
	startScript = strings.ReplaceAll(startScript, "\r\n", "\n")

	if err := serverfs.WriteFile(filepath.Join(dataDir, "start.sh"), []byte(startScript), 0770); err != nil {
		return fmt.Errorf("write startup script: %w", err)
	}
	return nil
}

func writeInstallScript(uuid string, installScript string, environment string) {
	dataDir := serverfs.LocalServicePath(uuid)

	// Replace placeholders in install script too
	if environment != "" {
		var envVars map[string]string
		if err := json.Unmarshal([]byte(environment), &envVars); err == nil {
			for k, v := range envVars {
				installScript = strings.ReplaceAll(installScript, "{{"+k+"}}", v)
			}
		}
	}

	// Normalize line endings for Linux
	installScript = strings.ReplaceAll(installScript, "\r\n", "\n")

	if err := serverfs.WriteFile(filepath.Join(dataDir, "install.sh"), []byte(installScript), 0770); err != nil {
		log.Printf("[Daemon] Failed to write install script for %s: %v", uuid, err)
	}
}

type PowerActionRequest struct {
	Action         string `json:"action"` // start, stop, restart, kill
	StartupCommand string `json:"startup_command"`
	Environment    string `json:"environment"`
	Port           int    `json:"port"`
	Memory         int64  `json:"memory"`
}

func powerActionErrorMessage(action string, port int, err error) string {
	detail := strings.TrimSpace(err.Error())
	prefix := fmt.Sprintf("Failed to %s server container", action)
	lowerDetail := strings.ToLower(detail)

	switch {
	case errdefs.IsNotFound(err):
		return prefix + ": the game container does not exist. Reinstall the service to recreate it. Docker: " + detail
	case strings.Contains(lowerDetail, "port is already allocated") || strings.Contains(lowerDetail, "address already in use"):
		return fmt.Sprintf("%s: host port %d is already in use. Stop the conflicting container, or assign this service a unique port and reinstall it to rebuild the Docker mapping. Docker: %s", prefix, port, detail)
	case strings.Contains(lowerDetail, "bind source path does not exist"):
		return prefix + ": the game data bind path does not exist on the Docker host. Check DATA_PATH/HOST_DATA_PATH. Docker: " + detail
	case strings.Contains(lowerDetail, "permission denied"):
		return prefix + ": Docker was denied access to the game files. Check DATA_PATH and SERVER_UID/SERVER_GID. Docker: " + detail
	default:
		return prefix + ": " + detail
	}
}

func reconcileStatusAfterPowerFailure(ctx context.Context, uuid string) {
	status := "offline"
	inspect, err := docker.Client.ContainerInspect(ctx, uuid)
	if err == nil && inspect.State != nil && inspect.State.Running {
		status = "running"
	} else if err != nil && !errdefs.IsNotFound(err) {
		log.Printf("[Daemon] Failed to inspect %s after power action failure: %v", uuid, err)
	}
	NotifyStatus(uuid, status)
}

func containerHasNetwork(networks map[string]*network.EndpointSettings, networkName string) bool {
	endpoint, ok := networks[networkName]
	return ok && endpoint != nil && endpoint.NetworkID != ""
}

func ensureContainerNetwork(ctx context.Context, uuid string) (bool, error) {
	inspect, err := docker.Client.ContainerInspect(ctx, uuid)
	if err != nil {
		return false, fmt.Errorf("inspect server container network: %w", err)
	}
	if inspect.NetworkSettings != nil && containerHasNetwork(inspect.NetworkSettings.Networks, defaultServerNetwork) {
		return false, nil
	}

	// A running legacy container can still hold its published host ports even
	// without a usable endpoint. Stop it before Docker programs the new bridge.
	if inspect.State != nil && inspect.State.Running {
		log.Printf("[Daemon] Stopping container %s before repairing its Docker network", uuid)
		timeout := 10
		if err := docker.Client.ContainerStop(ctx, uuid, container.StopOptions{Timeout: &timeout}); err != nil {
			return false, fmt.Errorf("stop server container before network repair: %w", err)
		}
	}

	log.Printf("[Daemon] Container %s has no %s endpoint; attaching it before start", uuid, defaultServerNetwork)
	if err := docker.Client.NetworkConnect(ctx, defaultServerNetwork, uuid, &network.EndpointSettings{}); err != nil {
		return false, fmt.Errorf("attach server container to Docker %s network: %w", defaultServerNetwork, err)
	}

	inspect, err = docker.Client.ContainerInspect(ctx, uuid)
	if err != nil {
		return false, fmt.Errorf("verify server container network: %w", err)
	}
	if inspect.NetworkSettings == nil || !containerHasNetwork(inspect.NetworkSettings.Networks, defaultServerNetwork) {
		return false, fmt.Errorf("Docker did not create a %s network endpoint for the server container", defaultServerNetwork)
	}
	return true, nil
}

func HandlePowerAction(c *gin.Context) {
	token := c.GetHeader("X-Node-Token")
	if token != config.NodeConfig.NodeToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	uuid := c.Param("uuid")
	var req PowerActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[Daemon] Power Action: %s for %s. Env count: %d", req.Action, uuid, len(req.Environment))

	// Re-write start script if we have the data
	if req.StartupCommand != "" {
		log.Printf("[Daemon] Regenerating start.sh for %s", uuid)
		if err := writeStartScript(uuid, req.StartupCommand, req.Port, req.Memory, req.Environment); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	ctx := context.Background()
	var err error
	switch req.Action {
	case "start":
		_, err = ensureContainerNetwork(ctx, uuid)
		if err == nil {
			NotifyStatus(uuid, "starting")
			err = docker.Client.ContainerStart(ctx, uuid, container.StartOptions{})
		}
	case "stop":
		NotifyStatus(uuid, "stopping")
		timeout := 10
		err = docker.Client.ContainerStop(ctx, uuid, container.StopOptions{Timeout: &timeout})
	case "restart":
		var networkRepaired bool
		networkRepaired, err = ensureContainerNetwork(ctx, uuid)
		if err == nil {
			NotifyStatus(uuid, "starting")
			if networkRepaired {
				err = docker.Client.ContainerStart(ctx, uuid, container.StartOptions{})
			} else {
				timeout := 10
				err = docker.Client.ContainerRestart(ctx, uuid, container.StopOptions{Timeout: &timeout})
			}
		}
	case "kill":
		NotifyStatus(uuid, "offline")
		err = docker.Client.ContainerKill(ctx, uuid, "SIGKILL")
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action"})
		return
	}

	if err != nil {
		errorMessage := powerActionErrorMessage(req.Action, req.Port, err)
		log.Printf("[Daemon] Power action failed for %s: %s", uuid, errorMessage)
		reconcileStatusAfterPowerFailure(ctx, uuid)
		c.JSON(http.StatusInternalServerError, gin.H{"error": errorMessage})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func HandleConsole(c *gin.Context) {
	uuid := c.Param("uuid")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[Daemon] WebSocket upgrade failed for %s: %v", uuid, err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Inspect container to see if it has TTY enabled
	inspect, err := docker.Client.ContainerInspect(ctx, uuid)
	if err != nil {
		log.Printf("[Daemon] Failed to inspect container %s: %v", uuid, err)
		conn.WriteMessage(websocket.TextMessage, []byte(consoleUnavailableMessage(uuid, err)))
		return
	}

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: true,
		Tail:       "500",
	}

	reader, err := docker.Client.ContainerLogs(ctx, uuid, options)
	if err != nil {
		log.Printf("[Daemon] Failed to get logs for %s: %v", uuid, err)
		return
	}
	defer reader.Close()

	// Handle disconnect from client side
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				cancel() // Stop the log reader if the websocket is closed
				reader.Close()
				break
			}
		}
	}()

	// Use a more robust way to read logs.
	// If TTY is enabled, logs are raw. If not, they are multiplexed with an 8-byte header.
	if inspect.Config.Tty {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Text()
			// Docker timestamps: 2024-03-21T12:00:00.123456789Z message
			// We keep the line as is, the frontend handles stripping if needed, but we do basic trimming.
			line = strings.TrimRight(line, "\r\n")

			if err := conn.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("[Daemon] Console scanner error for %s (TTY): %v", uuid, err)
		}
	} else {
		// Non-TTY: Use stdcopy to demultiplex the stream
		// We use a pipe to convert the demultiplexed stream back into lines
		pr, pw := io.Pipe()
		go func() {
			_, err := stdcopy.StdCopy(pw, pw, reader)
			pw.CloseWithError(err)
		}()

		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			line := scanner.Text()
			line = strings.TrimRight(line, "\r\n")
			if err := conn.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("[Daemon] Console scanner error for %s (Non-TTY): %v", uuid, err)
		}
	}
}

func consoleUnavailableMessage(uuid string, inspectErr error) string {
	if !errdefs.IsNotFound(inspectErr) {
		return "[Atlas] Error: Container is inaccessible: " + inspectErr.Error()
	}

	failureMarker := filepath.Join(serverfs.LocalServicePath(uuid), ".atlas_install_failed")
	if failure, err := os.ReadFile(failureMarker); err == nil {
		details := strings.TrimSpace(string(failure))
		if len(details) > 2000 {
			details = details[:2000] + "..."
		}
		if details != "" {
			return "[Atlas] Installation failed before the game container was created: " + details
		}
	}

	return "[Atlas] The game container has not been created yet. Installation may still be in progress."
}

func HandleStats(c *gin.Context) {
	uuid := c.Param("uuid")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Get stats once.
	s, err := docker.Client.ContainerStats(ctx, uuid, false)
	if err != nil {
		log.Printf("[Daemon] Error fetching stats for %s: %v", uuid, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer s.Body.Close()

	// Use a local struct to decode the stats JSON. This is more reliable than relying on
	// specific versions of the Docker SDK types.
	var stats struct {
		CPUStats struct {
			CPUUsage struct {
				TotalUsage  uint64   `json:"total_usage"`
				PercpuUsage []uint64 `json:"percpu_usage"`
			} `json:"cpu_usage"`
			SystemUsage uint64 `json:"system_cpu_usage"`
			OnlineCPUs  uint64 `json:"online_cpus"`
		} `json:"cpu_stats"`
		PreCPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemUsage uint64 `json:"system_cpu_usage"`
		} `json:"precpu_stats"`
		MemoryStats struct {
			Usage uint64            `json:"usage"`
			Stats map[string]uint64 `json:"stats"`
		} `json:"memory_stats"`
		Networks map[string]struct {
			RxBytes uint64 `json:"rx_bytes"`
			TxBytes uint64 `json:"tx_bytes"`
		} `json:"networks"`
	}

	if err := json.NewDecoder(s.Body).Decode(&stats); err != nil {
		log.Printf("[Daemon] Error decoding stats for %s: %v", uuid, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode stats"})
		return
	}

	// Calculate CPU %
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage) - float64(stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage) - float64(stats.PreCPUStats.SystemUsage)

	cpuPercent := 0.0
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpus := float64(stats.CPUStats.OnlineCPUs)
		if cpus == 0 {
			cpus = float64(len(stats.CPUStats.CPUUsage.PercpuUsage))
			if cpus == 0 {
				cpus = 1
			}
		}
		cpuPercent = (cpuDelta / systemDelta) * cpus * 100.0
	}

	// Calculate Memory
	// usage - cache is the standard way to show "real" memory usage
	realMemory := stats.MemoryStats.Usage
	if cache, ok := stats.MemoryStats.Stats["cache"]; ok {
		realMemory -= cache
	} else if inactive, ok := stats.MemoryStats.Stats["total_inactive_file"]; ok {
		realMemory -= inactive
	}

	memoryMB := realMemory / 1024 / 1024

	// Calculate Network
	var rx, tx uint64
	for _, n := range stats.Networks {
		rx += n.RxBytes
		tx += n.TxBytes
	}

	c.JSON(http.StatusOK, gin.H{
		"cpu":     fmt.Sprintf("%.1f", cpuPercent),
		"memory":  memoryMB,
		"network": gin.H{"rx": rx, "tx": tx},
	})
}

func HandleSendCommand(c *gin.Context) {
	uuid := c.Param("uuid")

	var req struct {
		Command string `json:"command"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	resp, err := docker.Client.ContainerAttach(ctx, uuid, container.AttachOptions{
		Stream: true,
		Stdin:  true,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer resp.Close()

	fmt.Fprintf(resp.Conn, "%s\n", req.Command)
	c.JSON(http.StatusOK, gin.H{"status": "sent"})
}

func DeleteServer(c *gin.Context) {
	token := c.GetHeader("X-Node-Token")
	if token != config.NodeConfig.NodeToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Token"})
		return
	}

	uuid := c.Param("uuid")
	ctx := context.Background()

	// 1. Force remove the container (stops and removes)
	removeOpts := container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	}

	if err := docker.Client.ContainerRemove(ctx, uuid, removeOpts); err != nil {
		// If container doesn't exist, we might still want to proceed with file cleanup
		log.Printf("[Daemon] Warning: Failed to remove container %s: %v", uuid, err)
	}

	// 2. Remove data directory
	dataDir := serverfs.LocalServicePath(uuid)
	if err := os.RemoveAll(dataDir); err != nil {
		log.Printf("[Daemon] Error cleaning up directory %s: %v", dataDir, err)
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
