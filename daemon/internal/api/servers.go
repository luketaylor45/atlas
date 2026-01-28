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
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/luketaylor45/atlas/daemon/internal/config"
	"github.com/luketaylor45/atlas/daemon/internal/docker"
	"github.com/luketaylor45/atlas/daemon/internal/installer"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

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
	// 1. Auth Check (Simple Token Check)
	token := c.GetHeader("X-Node-Token")
	if token != config.NodeConfig.NodeToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Token"})
		return
	}

	var req CreateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Received Create Server Request: %s (%s)", req.UUID, req.EggImage)

	ctx := context.Background()

	// 2. Pull Image
	// Using a goroutine for non-blocking? For now, blocking is safer to report errors.
	// Or just trigger it and return "Accepted". "Accepted" is better for long ops.
	// But for this "vertical slice", let's block or use a background routine but logging carefully.

	log.Printf("Starting pull for image: %s", req.EggImage)
	reader, err := docker.Client.ImagePull(ctx, req.EggImage, image.PullOptions{})
	if err != nil {
		log.Printf("!! CRITICAL: Failed to pull image: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Docker Error",
			"details": err.Error(),
		})
		return
	}
	defer reader.Close()
	io.Copy(os.Stdout, reader) // Pipe pull output to logs

	// 3. Configure Container

	cPortUDP := nat.Port(fmt.Sprintf("%d/udp", req.Port))
	cPortTCP := nat.Port(fmt.Sprintf("%d/tcp", req.Port))

	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			cPortUDP: []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", req.Port)}},
			cPortTCP: []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", req.Port)}},
		},
		Resources: container.Resources{
			// ...
			Memory:   req.Memory * 1024 * 1024, // MB to Bytes
			NanoCPUs: req.Cpu * 10000000,       // 100% = 1e9
		},
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: filepath.Join(config.NodeConfig.DataPath, req.UUID),
				Target: "/home/container",
			},
		},
	}

	// Ensure data directory exists
	dataDir := filepath.Join(config.NodeConfig.DataPath, req.UUID)
	os.MkdirAll(dataDir, 0755)

	// 3. Handle Installation Phase
	if req.InstallScript != "" {
		// Set Status to Installing
		NotifyStatus(req.UUID, "installing")

		go func() {
			log.Printf("[Daemon] Starting background installation for %s", req.UUID)
			inst := installer.New(docker.Client)

			installImage := req.InstallContainer
			if installImage == "" {
				installImage = "ghcr.io/pterodactyl/installers:alpine" // Better default
			}

			// Parse env variables
			var envVars []string
			if req.Environment != "" {
				var envMap map[string]string
				if err := json.Unmarshal([]byte(req.Environment), &envMap); err == nil {
					for k, v := range envMap {
						envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
					}
				}
			}

			err := inst.Install(context.Background(), req.UUID, installImage, req.InstallScript, envVars)
			if err != nil {
				log.Printf("[Daemon] Installation FAILED for %s: %v", req.UUID, err)
				NotifyStatus(req.UUID, "installation_failed")
				os.WriteFile(filepath.Join(dataDir, ".atlas_install_failed"), []byte(err.Error()), 0644)
				return
			}

			log.Printf("[Daemon] Installation SUCCEEDED for %s", req.UUID)
			os.WriteFile(filepath.Join(dataDir, ".atlas_installed"), []byte(time.Now().Format(time.RFC3339)), 0644)

			NotifyStatus(req.UUID, "offline")
		}()
	}

	// 4. Write Start Script
	writeStartScript(req.UUID, req.StartupCommand, req.Port, req.Memory, req.Environment, config.NodeConfig.NodeToken)

	// Determine how the container should reach the Core.
	containerCoreURL := config.NodeConfig.CoreURL
	if strings.Contains(containerCoreURL, "localhost") || strings.Contains(containerCoreURL, "127.0.0.1") {
		containerCoreURL = strings.Replace(containerCoreURL, "localhost", "host.docker.internal", 1)
		containerCoreURL = strings.Replace(containerCoreURL, "127.0.0.1", "host.docker.internal", 1)
	}

	env := []string{
		"STARTUP=bash start.sh",
		"SERVER_MEMORY=" + fmt.Sprintf("%d", req.Memory),
		"SERVER_PORT=" + fmt.Sprintf("%d", req.Port),
		"SERVER_UUID=" + req.UUID,
		"CORE_URL=" + containerCoreURL,
		"NODE_TOKEN=" + config.NodeConfig.NodeToken,
	}

	// Parse Egg Environment JSON for actual container env too
	if req.Environment != "" {
		var eggEnv map[string]string
		if err := json.Unmarshal([]byte(req.Environment), &eggEnv); err == nil {
			for k, v := range eggEnv {
				env = append(env, fmt.Sprintf("%s=%s", k, v))
			}
		}
	}

	containerConfig := &container.Config{
		Image:     req.EggImage,
		Tty:       true,
		OpenStdin: true,
		Env:       env,
	}

	log.Printf("[Daemon] Injected Wrapper Script and set STARTUP=bash start.sh")

	// 4. Create Container
	resp, err := docker.Client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, req.UUID)
	if err != nil {
		log.Printf("Failed to create container: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create container: " + err.Error()})
		return
	}

	// 5. Notify Core that we are ready
	NotifyStatus(req.UUID, "installing")

	// 6. START THE CONTAINER AUTOMATICALLY (Only if NOT installing)
	if req.InstallScript == "" {
		log.Printf("[Daemon] Starting container %s...", resp.ID)
		if err := docker.Client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
			log.Printf("Failed to start container: %v", err)
		}
	} else {
		log.Printf("[Daemon] Container %s created but waiting for installation to finish.", resp.ID)
	}

	c.JSON(http.StatusCreated, gin.H{"container_id": resp.ID})
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
	writeStartScript(uuid, req.StartupCommand, req.Port, int64(req.Memory), req.Environment, config.NodeConfig.NodeToken)

	// 3. We can't change the Image of a running container without recreating it.
	// But we can update the record. Recreations should happen on Reinstall or manual rebuild.
	// For now, we update scripts and resources.

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

type ReinstallRequest struct {
	InstallScript    string `json:"install_script"`
	InstallContainer string `json:"install_container"`
	Environment      string `json:"environment"`
}

// HandleReinstall wipes game files to trigger a fresh install
func HandleReinstall(c *gin.Context) {
	uuid := c.Param("uuid")

	var req ReinstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload: " + err.Error()})
		return
	}

	ctx := context.Background()

	// 1. Stop container
	docker.Client.ContainerStop(ctx, uuid, container.StopOptions{})

	// 2. Wipe files except start.sh and steamcmd
	dataDir := filepath.Join(config.NodeConfig.DataPath, uuid)
	files, _ := os.ReadDir(dataDir)
	for _, f := range files {
		if f.Name() == "start.sh" || f.Name() == "steamcmd" {
			continue
		}
		os.RemoveAll(filepath.Join(dataDir, f.Name()))
	}

	// 3. Notify Core
	NotifyStatus(uuid, "installing")

	// 4. Run Installer
	go func() {
		log.Printf("[Daemon] Starting background RE-installation for %s", uuid)
		inst := installer.New(docker.Client)

		installImage := req.InstallContainer
		if installImage == "" {
			installImage = "ghcr.io/pterodactyl/installers:alpine"
		}

		// Parse env variables
		var envVars []string
		if req.Environment != "" {
			var envMap map[string]string
			if err := json.Unmarshal([]byte(req.Environment), &envMap); err == nil {
				for k, v := range envMap {
					envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
				}
			}
		}

		err := inst.Install(context.Background(), uuid, installImage, req.InstallScript, envVars)
		if err != nil {
			log.Printf("[Daemon] Re-installation FAILED for %s: %v", uuid, err)
			NotifyStatus(uuid, "installation_failed")
			os.WriteFile(filepath.Join(dataDir, ".atlas_install_failed"), []byte(err.Error()), 0644)
			return
		}

		log.Printf("[Daemon] Re-installation SUCCEEDED for %s", uuid)
		os.WriteFile(filepath.Join(dataDir, ".atlas_installed"), []byte(time.Now().Format(time.RFC3339)), 0644)

		NotifyStatus(uuid, "offline")
	}()

	c.JSON(http.StatusOK, gin.H{"status": "reinstall_triggered"})
}

func writeStartScript(uuid string, startupCmd string, port int, memory int64, environment string, nodeToken string) {
	dataDir := filepath.Join(config.NodeConfig.DataPath, uuid)
	os.MkdirAll(dataDir, 0755)

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
	startScript := fmt.Sprintf(`#!/bin/bash
echo "--- Atlas Instance Wrapper ---"
echo "Working Directory: $(pwd)"
echo "Environment: Port=%d, Memory=%dMB"

# Notify Running
curl -X POST "${CORE_URL}/api/v1/internal/services/${SERVER_UUID}/status" -H "Content-Type: application/json" -d "{\"status\":\"running\", \"token\": \"${NODE_TOKEN}\"}" > /dev/null 2>&1

echo "Starting Server..."
# Anchor CWD to game root
cd "/home/container"

# Run the startup command directly
%s
`, port, memory, processedCmd)

	// Normalize line endings for Linux
	startScript = strings.ReplaceAll(startScript, "\r\n", "\n")

	os.WriteFile(filepath.Join(dataDir, "start.sh"), []byte(startScript), 0755)
}

func writeInstallScript(uuid string, installScript string, environment string) {
	dataDir := filepath.Join(config.NodeConfig.DataPath, uuid)

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

	os.WriteFile(filepath.Join(dataDir, "install.sh"), []byte(installScript), 0755)
}

type PowerActionRequest struct {
	Action         string `json:"action"` // start, stop, restart, kill
	StartupCommand string `json:"startup_command"`
	Environment    string `json:"environment"`
	Port           int    `json:"port"`
	Memory         int64  `json:"memory"`
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
		writeStartScript(uuid, req.StartupCommand, req.Port, req.Memory, req.Environment, config.NodeConfig.NodeToken)
	}

	ctx := context.Background()
	var err error
	switch req.Action {
	case "start":
		NotifyStatus(uuid, "starting")
		err = docker.Client.ContainerStart(ctx, uuid, container.StartOptions{})
	case "stop":
		NotifyStatus(uuid, "stopping")
		timeout := 10
		err = docker.Client.ContainerStop(ctx, uuid, container.StopOptions{Timeout: &timeout})
	case "restart":
		NotifyStatus(uuid, "starting")
		timeout := 10
		err = docker.Client.ContainerRestart(ctx, uuid, container.StopOptions{Timeout: &timeout})
	case "kill":
		NotifyStatus(uuid, "offline")
		err = docker.Client.ContainerKill(ctx, uuid, "SIGKILL")
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func HandleConsole(c *gin.Context) {
	uuid := c.Param("uuid")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx := context.Background()
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: true,
		Tail:       "500",
	}

	reader, err := docker.Client.ContainerLogs(ctx, uuid, options)
	if err != nil {
		return
	}
	defer reader.Close()

	// Handle disconnect from client side
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				reader.Close()
				break
			}
		}
	}()

	// Use a scanner to read line by line properly
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()

		// Docker logs often have trailing \r on Windows or just extra space
		line = strings.TrimRight(line, "\r\n ")
		if line == "" {
			continue
		}

		if err := conn.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
			return
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[Daemon] Console scanner error for %s: %v", uuid, err)
	}
}

func HandleStats(c *gin.Context) {
	uuid := c.Param("uuid")
	// Increased timeout to 10s to account for slow Docker pipes on Windows
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Get stats once. Docker's API returns current and previous samples (precpu_stats).
	s, err := docker.Client.ContainerStats(ctx, uuid, false)
	if err != nil {
		if err == context.DeadlineExceeded {
			log.Printf("[Daemon] Stats timeout for %s - returning zeroed metrics", uuid)
			c.JSON(http.StatusOK, gin.H{
				"cpu":     "0.0",
				"memory":  0,
				"network": gin.H{"rx": 0, "tx": 0},
			})
			return
		}
		log.Printf("[Daemon] Error fetching stats for %s: %v", uuid, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer s.Body.Close()

	// We define a local struct to precisely match the Docker JSON without SDK version conflicts
	var stats struct {
		CPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
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
			Usage uint64 `json:"usage"`
			Stats struct {
				Cache uint64 `json:"cache"`
			} `json:"stats"`
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

	// Calculate CPU % using Pterodactyl / Docker Dashboard logic
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage) - float64(stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage) - float64(stats.PreCPUStats.SystemUsage)

	cpuPercent := 0.0
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpus := float64(stats.CPUStats.OnlineCPUs)
		if cpus == 0 {
			cpus = 1
		}
		// Multiply by core count to match physical core utilization
		cpuPercent = (cpuDelta / systemDelta) * cpus * 100.0
	}

	// Calculate Memory (Subtracting cache provides the "real" usage as expected by power users)
	realMemory := stats.MemoryStats.Usage - stats.MemoryStats.Stats.Cache
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
	dataDir := filepath.Join(config.NodeConfig.DataPath, uuid)
	if err := os.RemoveAll(dataDir); err != nil {
		log.Printf("[Daemon] Error cleaning up directory %s: %v", dataDir, err)
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
