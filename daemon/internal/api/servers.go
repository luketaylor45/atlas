package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/luketaylor45/atlas/daemon/internal/docker"
	"github.com/luketaylor45/atlas/daemon/internal/installer"
	"github.com/luketaylor45/atlas/daemon/internal/serverfs"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

const defaultServerNetwork = "bridge"

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

type PowerActionRequest struct {
	Action         string `json:"action"` // start, stop, restart, kill
	StartupCommand string `json:"startup_command"`
	StopCommand    string `json:"stop_command"`
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
		return prefix + ": the game container does not exist. Rebuild or reinstall the service to recreate it. Docker: " + detail
	case strings.Contains(lowerDetail, "port is already allocated") || strings.Contains(lowerDetail, "address already in use"):
		return fmt.Sprintf("%s: host port %d is already in use. Stop the conflicting container, or assign this service a different port in the admin panel. Docker: %s", prefix, port, detail)
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

// containerHasNetwork reports whether a container is set up to join a
// network. Docker only fills in the network ID once a container has started,
// so a container that was created but never run has a configured endpoint
// with an empty ID. That is normal and is not a container needing repair.
func containerHasNetwork(networks map[string]*network.EndpointSettings, networkName string, running bool) bool {
	endpoint, ok := networks[networkName]
	if !ok || endpoint == nil {
		return false
	}
	return endpoint.NetworkID != "" || !running
}

func ensureContainerNetwork(ctx context.Context, uuid string) (bool, error) {
	inspect, err := docker.Client.ContainerInspect(ctx, uuid)
	if err != nil {
		return false, fmt.Errorf("inspect server container network: %w", err)
	}
	running := inspect.State != nil && inspect.State.Running
	if inspect.NetworkSettings != nil && containerHasNetwork(inspect.NetworkSettings.Networks, defaultServerNetwork, running) {
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
	if inspect.NetworkSettings == nil || !containerHasNetwork(inspect.NetworkSettings.Networks, defaultServerNetwork, false) {
		return false, fmt.Errorf("Docker did not create a %s network endpoint for the server container", defaultServerNetwork)
	}
	return true, nil
}

func HandlePowerAction(c *gin.Context) {
	uuid := c.Param("uuid")
	var req PowerActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if installs.active(uuid) {
		c.JSON(http.StatusConflict, gin.H{"error": "This server is being installed. Wait for the installation to finish or cancel it."})
		return
	}

	log.Printf("[Daemon] Power action %s for %s", req.Action, uuid)

	// Re-write the start script so the latest startup settings apply.
	if req.StartupCommand != "" && req.Action != "stop" && req.Action != "kill" {
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
		err = stopGameContainer(ctx, uuid, req.StopCommand)
	case "restart":
		NotifyStatus(uuid, "stopping")
		err = stopGameContainer(ctx, uuid, req.StopCommand)
		if err == nil {
			_, err = ensureContainerNetwork(ctx, uuid)
		}
		if err == nil {
			NotifyStatus(uuid, "starting")
			err = docker.Client.ContainerStart(ctx, uuid, container.StartOptions{})
		}
	case "kill":
		err = docker.Client.ContainerKill(ctx, uuid, "SIGKILL")
		if err == nil {
			NotifyStatus(uuid, "offline")
		}
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

	if strings.TrimSpace(req.Command) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Command is empty"})
		return
	}

	if err := sendConsoleCommand(c.Request.Context(), uuid, req.Command); err != nil {
		if errors.Is(err, errServerNotRunning) || errdefs.IsNotFound(err) || errdefs.IsConflict(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "The server is not running"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send command: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "sent"})
}

func DeleteServer(c *gin.Context) {
	uuid := c.Param("uuid")
	ctx := context.Background()

	// An install in progress would recreate what is being deleted.
	if installs.cancel(uuid) {
		deadline := time.Now().Add(30 * time.Second)
		for installs.active(uuid) && time.Now().Before(deadline) {
			time.Sleep(200 * time.Millisecond)
		}
		if installs.active(uuid) {
			c.JSON(http.StatusConflict, gin.H{"error": "The running installation could not be stopped. Try again shortly."})
			return
		}
	}

	_ = docker.Client.ContainerRemove(ctx, installer.ContainerName(uuid), container.RemoveOptions{Force: true})
	if err := docker.Client.ContainerRemove(ctx, uuid, container.RemoveOptions{Force: true, RemoveVolumes: true}); err != nil && !errdefs.IsNotFound(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove the game container: " + err.Error()})
		return
	}

	if err := os.RemoveAll(serverfs.LocalServicePath(uuid)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove server files: " + err.Error()})
		return
	}
	_ = os.Remove(installLogPath(uuid))

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
