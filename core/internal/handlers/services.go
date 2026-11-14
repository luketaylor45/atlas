package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
	"github.com/luketaylor45/atlas/core/internal/utils"
)

func resolveStartupCommand(service *models.Service) string {
	if service.StartupCommand != "" {
		return service.StartupCommand
	}
	return service.Egg.StartupCommand
}

func nodeErrorMessage(body io.Reader, fallback string) string {
	var daemonErr struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(body, 64*1024)).Decode(&daemonErr); err == nil {
		if message := strings.TrimSpace(daemonErr.Error); message != "" {
			return message
		}
	}
	return fallback
}

// GetUserOverview returns statistics for the user dashboard
func GetUserOverview(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	var totalServices int64
	var runningServices int64

	// Count services where user is owner OR sub-user
	database.DB.Model(&models.Service{}).
		Where("user_id = ? OR id IN (SELECT service_id FROM service_users WHERE user_id = ?)", userID, userID).
		Count(&totalServices)

	database.DB.Model(&models.Service{}).
		Where("(user_id = ? OR id IN (SELECT service_id FROM service_users WHERE user_id = ?)) AND status = 'running'", userID, userID).
		Count(&runningServices)

	// Determine infrastructure health for this user
	var nodeIDs []uint
	database.DB.Model(&models.Service{}).
		Where("user_id = ? OR id IN (SELECT service_id FROM service_users WHERE user_id = ?)", userID, userID).
		Pluck("node_id", &nodeIDs)

	health := "OPTIMAL"
	if len(nodeIDs) > 0 {
		var offlineCount int64
		database.DB.Model(&models.Node{}).
			Where("id IN (?) AND is_online = ?", nodeIDs, false).
			Count(&offlineCount)

		if offlineCount > 0 {
			health = "DEGRADED"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total_services":   totalServices,
		"running_services": runningServices,
		"health":           health,
	})
}

// GetUserServices returns services the user owns or has sub-user access to
func GetUserServices(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	var services []models.Service

	// Query services where user is owner OR user is a sub-user
	err := database.DB.Preload("Node").Preload("Egg.Nest").Preload("Egg.Variables").
		Where("user_id = ? OR id IN (SELECT service_id FROM service_users WHERE user_id = ?)", userID, userID).
		Find(&services).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch services"})
		return
	}

	c.JSON(http.StatusOK, services)
}

// GetServiceDetails returns a single service by UUID
func GetServiceDetails(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	uuid := c.Param("uuid")

	service, subUser, ok := utils.FindServiceForUser(uuid, userID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found or no access"})
		return
	}

	// Check if this user is an admin
	var user models.User
	database.DB.First(&user, userID)

	if user.IsAdmin {
		c.JSON(http.StatusOK, gin.H{
			"service":  service,
			"is_owner": true, // Give admins "owner" view
		})
		return
	}

	if subUser != nil {
		c.JSON(http.StatusOK, gin.H{
			"service":     service,
			"permissions": subUser,
			"is_owner":    false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"service":  service,
		"is_owner": true,
	})
}

// ServicePowerAction proxies a power request to the target node
func ServicePowerAction(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	uuid := c.Param("uuid")

	var req struct {
		Action string `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	service, subUser, ok := utils.FindServiceForUser(uuid, userID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found or no access"})
		return
	}

	// Permission check for sub-users
	if subUser != nil && !subUser.CanControlPower {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to control power for this server"})
		return
	}

	// MERGE Environment: Start with Egg defaults, then apply Service overrides
	mergedEnv := make(map[string]string)
	for _, v := range service.Egg.Variables {
		mergedEnv[v.EnvironmentVariable] = v.DefaultValue
	}
	if service.Environment != "" {
		overrides := make(map[string]string)
		if err := json.Unmarshal([]byte(service.Environment), &overrides); err == nil {
			for k, v := range overrides {
				mergedEnv[k] = v
			}
		}
	}
	finalEnvJSON, _ := json.Marshal(mergedEnv)

	// Proxy to Daemon
	url := fmt.Sprintf("http://%s:%s/api/servers/%s/power", service.Node.Address, service.Node.Port, service.UUID)

	payloadObj := struct {
		Action         string `json:"action"`
		StartupCommand string `json:"startup_command"`
		Environment    string `json:"environment"`
		Memory         uint64 `json:"memory"`
		Port           int    `json:"port"`
	}{
		Action:         req.Action,
		StartupCommand: resolveStartupCommand(service),
		Environment:    string(finalEnvJSON),
		Memory:         service.Memory,
		Port:           service.Port,
	}

	log.Printf("[Core] Sending Power Action '%s' to Node %s (Service: %s). Env count: %d", req.Action, service.Node.Name, service.UUID, len(mergedEnv))

	payload, _ := json.Marshal(payloadObj)
	client := &http.Client{}
	proxyReq, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	proxyReq.Header.Set("X-Node-Token", service.Node.Token)
	proxyReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(proxyReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to connect to node: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		fallback := fmt.Sprintf("Node returned HTTP %d during power action", resp.StatusCode)
		c.JSON(resp.StatusCode, gin.H{"error": nodeErrorMessage(resp.Body, fallback)})
		return
	}

	// Log the power action
	utils.LogActivity(c, service.ID, "power", req.Action, fmt.Sprintf("Server %s", req.Action), map[string]interface{}{
		"action": req.Action,
		"node":   service.Node.Name,
	})

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// ServiceSendCommand proxies a console command to the target node
func ServiceSendCommand(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	uuid := c.Param("uuid")

	var req struct {
		Command string `json:"command"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	service, subUser, ok := utils.FindServiceForUser(uuid, userID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found or no access"})
		return
	}

	// Permission check
	if subUser != nil && !subUser.CanSendCommands {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to send commands to this server"})
		return
	}

	url := fmt.Sprintf("http://%s:%s/api/servers/%s/command", service.Node.Address, service.Node.Port, service.UUID)
	payload, _ := json.Marshal(req)

	client := &http.Client{}
	proxyReq, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	proxyReq.Header.Set("X-Node-Token", service.Node.Token)
	proxyReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(proxyReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to connect to node"})
		return
	}
	defer resp.Body.Close()

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// ServiceReinstall proxies a reinstall request to the target node
func ServiceReinstall(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	uuid := c.Param("uuid")

	service, subUser, ok := utils.FindServiceForUser(uuid, userID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found or no access"})
		return
	}

	// Permission check (Reinstall requires Control Power)
	if subUser != nil && !subUser.CanControlPower {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to reinstall this server"})
		return
	}

	url := fmt.Sprintf("http://%s:%s/api/servers/%s/reinstall", service.Node.Address, service.Node.Port, service.UUID)

	// MERGE Environment for Installer
	mergedEnv := make(map[string]string)
	for _, v := range service.Egg.Variables {
		mergedEnv[v.EnvironmentVariable] = v.DefaultValue
	}
	if service.Environment != "" {
		overrides := make(map[string]string)
		if err := json.Unmarshal([]byte(service.Environment), &overrides); err == nil {
			for k, v := range overrides {
				mergedEnv[k] = v
			}
		}
	}
	finalEnvJSON, _ := json.Marshal(mergedEnv)
	eggImage := service.DockerImage
	if eggImage == "" {
		var images []string
		if err := json.Unmarshal([]byte(service.Egg.DockerImages), &images); err == nil && len(images) > 0 {
			eggImage = images[0]
		}
	}

	payload := map[string]interface{}{
		"install_script":    service.Egg.ScriptInstall,
		"install_container": service.Egg.ScriptContainer,
		"environment":       string(finalEnvJSON),
		"memory":            service.Memory,
		"disk":              service.Disk,
		"cpu":               service.Cpu,
		"port":              service.Port,
		"egg_image":         eggImage,
		"startup_command":   service.StartupCommand,
	}
	body, _ := json.Marshal(payload)

	client := &http.Client{}
	proxyReq, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
	proxyReq.Header.Set("X-Node-Token", service.Node.Token)
	proxyReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(proxyReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to connect to node"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		c.JSON(resp.StatusCode, gin.H{"error": nodeErrorMessage(resp.Body, "Node returned error during reinstall trigger")})
		return
	}

	if err := database.DB.Model(service).Updates(map[string]interface{}{
		"status":                "installing",
		"installation_stage":    "Reinstallation queued",
		"installation_progress": 0,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Reinstall started, but service status could not be updated"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// UpdateServiceEnvironment updates the environment JSON for a service
func UpdateServiceEnvironment(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	uuid := c.Param("uuid")

	var req struct {
		StartupCommand string `json:"startup_command"`
		Environment    string `json:"environment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	service, subUser, ok := utils.FindServiceForUser(uuid, userID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found or no access"})
		return
	}

	// Permission check
	if subUser != nil && !subUser.CanEditStartup {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to edit startup variables for this server"})
		return
	}

	updates := map[string]interface{}{
		"environment": req.Environment,
	}
	if req.StartupCommand != "" {
		updates["startup_command"] = req.StartupCommand
		service.StartupCommand = req.StartupCommand
	}
	service.Environment = req.Environment
	if err := database.DB.Model(&models.Service{}).Where("id = ?", service.ID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update environment"})
		return
	}

	url := fmt.Sprintf("http://%s:%s/api/servers/%s", service.Node.Address, service.Node.Port, service.UUID)
	payload := map[string]interface{}{
		"memory":          service.Memory,
		"disk":            service.Disk,
		"cpu":             service.Cpu,
		"port":            service.Port,
		"docker_image":    service.DockerImage,
		"startup_command": resolveStartupCommand(service),
		"environment":     service.Environment,
	}
	body, _ := json.Marshal(payload)

	client := &http.Client{}
	proxyReq, _ := http.NewRequest("PUT", url, bytes.NewBuffer(body))
	proxyReq.Header.Set("X-Node-Token", service.Node.Token)
	proxyReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(proxyReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Saved configuration, but failed to update node"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		c.JSON(resp.StatusCode, gin.H{"error": "Saved configuration, but node rejected the update"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":          "success",
		"startup_command": service.StartupCommand,
		"environment":     service.Environment,
	})
}

// ServiceListFiles proxies a file list request to the node
func ServiceListFiles(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	uuid := c.Param("uuid")
	path := c.Query("path")

	service, subUser, ok := utils.FindServiceForUser(uuid, userID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found or no access"})
		return
	}

	// Permission check
	if subUser != nil && !subUser.CanManageFiles {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to manage files for this server"})
		return
	}

	url := fmt.Sprintf("http://%s:%s/api/servers/%s/files/list?path=%s", service.Node.Address, service.Node.Port, service.UUID, path)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Node-Token", service.Node.Token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Node unreachable"})
		return
	}
	defer resp.Body.Close()

	c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
}

func ServiceGetFileContent(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	uuid := c.Param("uuid")
	path := c.Query("path")

	service, subUser, ok := utils.FindServiceForUser(uuid, userID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found or no access"})
		return
	}

	// Permission check
	if subUser != nil && !subUser.CanManageFiles {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to manage files for this server"})
		return
	}

	url := fmt.Sprintf("http://%s:%s/api/servers/%s/files/content?path=%s", service.Node.Address, service.Node.Port, service.UUID, path)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Node-Token", service.Node.Token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Node unreachable"})
		return
	}
	defer resp.Body.Close()

	c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
}

func ServiceWriteFile(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	uuid := c.Param("uuid")
	path := c.Query("path")

	service, subUser, ok := utils.FindServiceForUser(uuid, userID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found or no access"})
		return
	}

	// Permission check
	if subUser != nil && !subUser.CanManageFiles {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to manage files for this server"})
		return
	}

	url := fmt.Sprintf("http://%s:%s/api/servers/%s/files/write?path=%s", service.Node.Address, service.Node.Port, service.UUID, path)
	req, _ := http.NewRequest("POST", url, c.Request.Body)
	req.Header.Set("X-Node-Token", service.Node.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Node unreachable"})
		return
	}
	defer resp.Body.Close()

	c.JSON(resp.StatusCode, gin.H{"status": "success"})
}

func ServiceCreateFolder(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	uuid := c.Param("uuid")

	service, subUser, ok := utils.FindServiceForUser(uuid, userID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found or no access"})
		return
	}

	// Permission check
	if subUser != nil && !subUser.CanManageFiles {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to manage files for this server"})
		return
	}

	url := fmt.Sprintf("http://%s:%s/api/servers/%s/files/create-folder", service.Node.Address, service.Node.Port, service.UUID)
	req, _ := http.NewRequest("POST", url, c.Request.Body)
	req.Header.Set("X-Node-Token", service.Node.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Node unreachable"})
		return
	}
	defer resp.Body.Close()

	c.JSON(resp.StatusCode, gin.H{"status": "success"})
}

func ServiceExtractArchive(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	uuid := c.Param("uuid")

	var extractRequest struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&extractRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "A ZIP archive path is required"})
		return
	}

	service, subUser, ok := utils.FindServiceForUser(uuid, userID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found or no access"})
		return
	}
	if subUser != nil && !subUser.CanManageFiles {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to manage files for this server"})
		return
	}

	payload, err := json.Marshal(extractRequest)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare extraction request"})
		return
	}
	url := fmt.Sprintf("http://%s:%s/api/servers/%s/files/extract", service.Node.Address, service.Node.Port, service.UUID)
	proxyRequest, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare node request"})
		return
	}
	proxyRequest.Header.Set("X-Node-Token", service.Node.Token)
	proxyRequest.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{}).Do(proxyRequest)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Node unreachable"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		fallback := fmt.Sprintf("Node returned HTTP %d while extracting the archive", resp.StatusCode)
		c.JSON(resp.StatusCode, gin.H{"error": nodeErrorMessage(resp.Body, fallback)})
		return
	}

	utils.LogActivity(c, service.ID, "file_extract", extractRequest.Path, "Extracted ZIP archive", nil)
	c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
}

func ServiceDeleteFile(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	uuid := c.Param("uuid")
	path := c.Query("path")

	service, subUser, ok := utils.FindServiceForUser(uuid, userID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found or no access"})
		return
	}

	// Permission check
	if subUser != nil && !subUser.CanManageFiles {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to manage files for this server"})
		return
	}

	url := fmt.Sprintf("http://%s:%s/api/servers/%s/files?path=%s", service.Node.Address, service.Node.Port, service.UUID, path)
	req, _ := http.NewRequest("DELETE", url, nil)
	req.Header.Set("X-Node-Token", service.Node.Token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Node unreachable"})
		return
	}
	defer resp.Body.Close()

	c.JSON(resp.StatusCode, gin.H{"status": "success"})
}

func ServiceUploadFile(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	uuid := c.Param("uuid")
	path := c.Query("path")

	service, subUser, ok := utils.FindServiceForUser(uuid, userID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found or no access"})
		return
	}

	// Permission check
	if subUser != nil && !subUser.CanManageFiles {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to manage files for this server"})
		return
	}

	url := fmt.Sprintf("http://%s:%s/api/servers/%s/files/upload?path=%s", service.Node.Address, service.Node.Port, service.UUID, path)

	// Proxy the multipart body
	req, _ := http.NewRequest("POST", url, c.Request.Body)
	req.Header.Set("X-Node-Token", service.Node.Token)
	req.Header.Set("Content-Type", c.GetHeader("Content-Type"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Node unreachable"})
		return
	}
	defer resp.Body.Close()

	c.JSON(resp.StatusCode, gin.H{"status": "success"})
}
