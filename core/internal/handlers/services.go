package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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

// nodeStatus maps a node's HTTP status onto the one Core returns. A node
// rejecting Core's token is a server-side fault, and must not reach the
// browser as 401, which the panel treats as an expired session.
func nodeStatus(code int) int {
	if code == http.StatusUnauthorized || code == http.StatusForbidden || code == http.StatusServiceUnavailable {
		return http.StatusBadGateway
	}
	return code
}

// respondNodeResult relays the outcome of a node operation, including its error.
func respondNodeResult(c *gin.Context, resp *http.Response, fallback string) {
	if resp.StatusCode >= 400 {
		c.JSON(nodeStatus(resp.StatusCode), gin.H{"error": nodeErrorMessage(resp.Body, fallback)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// validateEnvironmentOverrides checks user-supplied variables against the
// egg. Only variables the egg defines are accepted, and only editable ones
// unless the caller is an admin, so a user cannot inject arbitrary values into
// the container environment or the startup command.
func validateEnvironmentOverrides(raw string, egg *models.Egg, current string, isAdmin bool) (string, error) {
	submitted := make(map[string]string)
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &submitted); err != nil {
			return "", fmt.Errorf("environment must be a JSON object of text values")
		}
	}
	existing := make(map[string]string)
	if strings.TrimSpace(current) != "" {
		_ = json.Unmarshal([]byte(current), &existing)
	}

	known := make(map[string]models.EggVariable, len(egg.Variables))
	for _, variable := range egg.Variables {
		known[variable.EnvironmentVariable] = variable
	}

	result := make(map[string]string, len(known))
	for key, variable := range known {
		value, hasCurrent := existing[key]
		if !hasCurrent {
			value = variable.DefaultValue
		}
		if next, ok := submitted[key]; ok && next != value {
			if !isAdmin && !variable.UserEditable {
				return "", fmt.Errorf("%s cannot be changed", variable.Name)
			}
			if strings.ContainsAny(next, "\n\r\x00") {
				return "", fmt.Errorf("%s must be a single line", variable.Name)
			}
			value = next
		}
		result[key] = value
	}
	// Values stored before the egg dropped a variable are kept as they are,
	// but nothing new may be introduced outside the egg's definition.
	for key, value := range existing {
		if _, ok := known[key]; !ok {
			result[key] = value
		}
	}
	for key, value := range submitted {
		if _, ok := known[key]; ok {
			continue
		}
		if previous, stored := existing[key]; !stored || previous != value {
			return "", fmt.Errorf("unknown variable %q", key)
		}
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
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
	url := nodeURL(&service.Node, fmt.Sprintf("/api/servers/%s/power", service.UUID))

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
	client := daemonHTTP
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
		c.JSON(nodeStatus(resp.StatusCode), gin.H{"error": nodeErrorMessage(resp.Body, fallback)})
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

	url := nodeURL(&service.Node, fmt.Sprintf("/api/servers/%s/command", service.UUID))
	payload, _ := json.Marshal(req)

	client := daemonHTTP
	proxyReq, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	proxyReq.Header.Set("X-Node-Token", service.Node.Token)
	proxyReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(proxyReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to connect to node"})
		return
	}
	defer resp.Body.Close()

	respondNodeResult(c, resp, "The node could not deliver the command")
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

	url := nodeURL(&service.Node, fmt.Sprintf("/api/servers/%s/reinstall", service.UUID))

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

	client := daemonHTTP
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
		c.JSON(nodeStatus(resp.StatusCode), gin.H{"error": nodeErrorMessage(resp.Body, "Node returned error during reinstall trigger")})
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

	isAdmin, _ := c.Get("is_admin")
	admin, _ := isAdmin.(bool)

	environment, err := validateEnvironmentOverrides(req.Environment, &service.Egg, service.Environment, admin)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{
		"environment": environment,
	}
	// The startup command decides what runs in the container, so only an
	// administrator may replace it. Everyone else configures it through variables.
	if req.StartupCommand != "" && req.StartupCommand != resolveStartupCommand(service) {
		if !admin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only an administrator can change the startup command"})
			return
		}
		updates["startup_command"] = req.StartupCommand
		service.StartupCommand = req.StartupCommand
	}
	service.Environment = environment
	if err := database.DB.Model(&models.Service{}).Where("id = ?", service.ID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update environment"})
		return
	}

	url := nodeURL(&service.Node, fmt.Sprintf("/api/servers/%s", service.UUID))
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

	client := daemonHTTP
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
		c.JSON(nodeStatus(resp.StatusCode), gin.H{"error": nodeErrorMessage(resp.Body, "Saved configuration, but the node rejected the update")})
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

	url := nodeURL(&service.Node, fmt.Sprintf("/api/servers/%s/files/list?path=%s", service.UUID, url.QueryEscape(path)))
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Node-Token", service.Node.Token)

	client := daemonHTTP
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Node unreachable"})
		return
	}
	defer resp.Body.Close()

	c.DataFromReader(nodeStatus(resp.StatusCode), resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
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

	url := nodeURL(&service.Node, fmt.Sprintf("/api/servers/%s/files/content?path=%s", service.UUID, url.QueryEscape(path)))
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Node-Token", service.Node.Token)

	client := daemonHTTP
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Node unreachable"})
		return
	}
	defer resp.Body.Close()

	c.DataFromReader(nodeStatus(resp.StatusCode), resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
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

	url := nodeURL(&service.Node, fmt.Sprintf("/api/servers/%s/files/write?path=%s", service.UUID, url.QueryEscape(path)))
	req, _ := http.NewRequest("POST", url, c.Request.Body)
	req.Header.Set("X-Node-Token", service.Node.Token)
	req.Header.Set("Content-Type", "application/json")

	client := daemonHTTP
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Node unreachable"})
		return
	}
	defer resp.Body.Close()

	respondNodeResult(c, resp, "The node could not complete the file operation")
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

	url := nodeURL(&service.Node, fmt.Sprintf("/api/servers/%s/files/create-folder", service.UUID))
	req, _ := http.NewRequest("POST", url, c.Request.Body)
	req.Header.Set("X-Node-Token", service.Node.Token)
	req.Header.Set("Content-Type", "application/json")

	client := daemonHTTP
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Node unreachable"})
		return
	}
	defer resp.Body.Close()

	respondNodeResult(c, resp, "The node could not complete the file operation")
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
	url := nodeURL(&service.Node, fmt.Sprintf("/api/servers/%s/files/extract", service.UUID))
	proxyRequest, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare node request"})
		return
	}
	proxyRequest.Header.Set("X-Node-Token", service.Node.Token)
	proxyRequest.Header.Set("Content-Type", "application/json")

	resp, err := daemonHTTP.Do(proxyRequest)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Node unreachable"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		fallback := fmt.Sprintf("Node returned HTTP %d while extracting the archive", resp.StatusCode)
		c.JSON(nodeStatus(resp.StatusCode), gin.H{"error": nodeErrorMessage(resp.Body, fallback)})
		return
	}

	utils.LogActivity(c, service.ID, "file_extract", extractRequest.Path, "Extracted ZIP archive", nil)
	c.DataFromReader(nodeStatus(resp.StatusCode), resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
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

	url := nodeURL(&service.Node, fmt.Sprintf("/api/servers/%s/files?path=%s", service.UUID, url.QueryEscape(path)))
	req, _ := http.NewRequest("DELETE", url, nil)
	req.Header.Set("X-Node-Token", service.Node.Token)

	client := daemonHTTP
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Node unreachable"})
		return
	}
	defer resp.Body.Close()

	respondNodeResult(c, resp, "The node could not complete the file operation")
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

	url := nodeURL(&service.Node, fmt.Sprintf("/api/servers/%s/files/upload?path=%s", service.UUID, url.QueryEscape(path)))

	// Proxy the multipart body
	req, _ := http.NewRequest("POST", url, c.Request.Body)
	req.Header.Set("X-Node-Token", service.Node.Token)
	req.Header.Set("Content-Type", c.GetHeader("Content-Type"))

	client := daemonTransferHTTP
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Node unreachable"})
		return
	}
	defer resp.Body.Close()

	respondNodeResult(c, resp, "The node could not complete the file operation")
}
