package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
)

// GetUserOverview returns statistics for the user dashboard
func GetUserOverview(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	var totalServices int64
	var runningServices int64

	database.DB.Model(&models.Service{}).Where("user_id = ?", userID).Count(&totalServices)
	database.DB.Model(&models.Service{}).Where("user_id = ? AND status = 'running'", userID).Count(&runningServices)

	c.JSON(http.StatusOK, gin.H{
		"total_services":   totalServices,
		"running_services": runningServices,
		"health":           "STABLE",
	})
}

// GetUserServices returns services belonging to the authenticated user
func GetUserServices(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	var services []models.Service
	if err := database.DB.Preload("Node").Preload("Egg.Nest").Preload("Egg.Variables").Where("user_id = ?", userID).Find(&services).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch services"})
		return
	}

	c.JSON(http.StatusOK, services)
}

// GetServiceDetails returns a single service by UUID
func GetServiceDetails(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	uuid := c.Param("uuid")

	var service models.Service
	if err := database.DB.Preload("Node").Preload("Egg.Nest").Preload("Egg.Variables").Where("uuid = ? AND user_id = ?", uuid, userID).First(&service).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	c.JSON(http.StatusOK, service)
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

	var service models.Service
	if err := database.DB.Preload("Node").Preload("Egg.Variables").Where("uuid = ? AND user_id = ?", uuid, userID).First(&service).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
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
		StartupCommand: service.Egg.StartupCommand,
		Environment:    string(finalEnvJSON),
		Memory:         service.Memory,
		Port:           service.Port,
	}

	log.Printf("[Core] Sending Power Action '%s' to Node %s (Service: %s). Env: %s", req.Action, service.Node.Name, service.UUID, payloadObj.Environment)

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
		c.JSON(resp.StatusCode, gin.H{"error": "Node responded with error"})
		return
	}

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

	var service models.Service
	if err := database.DB.Preload("Node").Where("uuid = ? AND user_id = ?", uuid, userID).First(&service).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
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

	var service models.Service
	if err := database.DB.Preload("Node").Where("uuid = ? AND user_id = ?", uuid, userID).First(&service).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	url := fmt.Sprintf("http://%s:%s/api/servers/%s/reinstall", service.Node.Address, service.Node.Port, service.UUID)

	client := &http.Client{}
	proxyReq, _ := http.NewRequest("POST", url, nil)
	proxyReq.Header.Set("X-Node-Token", service.Node.Token)

	resp, err := client.Do(proxyReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to connect to node"})
		return
	}
	defer resp.Body.Close()

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// UpdateServiceEnvironment updates the environment JSON for a service
func UpdateServiceEnvironment(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	uuid := c.Param("uuid")

	var req struct {
		Environment string `json:"environment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var service models.Service
	if err := database.DB.Where("uuid = ? AND user_id = ?", uuid, userID).First(&service).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	service.Environment = req.Environment
	if err := database.DB.Save(&service).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update environment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// ServiceListFiles proxies a file list request to the node
func ServiceListFiles(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	uuid := c.Param("uuid")
	path := c.Query("path")

	var service models.Service
	if err := database.DB.Preload("Node").Where("uuid = ? AND user_id = ?", uuid, userID).First(&service).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
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

	var service models.Service
	if err := database.DB.Preload("Node").Where("uuid = ? AND user_id = ?", uuid, userID).First(&service).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
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

	var service models.Service
	if err := database.DB.Preload("Node").Where("uuid = ? AND user_id = ?", uuid, userID).First(&service).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	url := fmt.Sprintf("http://%s:%s/api/servers/%s/files/write", service.Node.Address, service.Node.Port, service.UUID)
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

	var service models.Service
	if err := database.DB.Preload("Node").Where("uuid = ? AND user_id = ?", uuid, userID).First(&service).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
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

func ServiceDeleteFile(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	uuid := c.Param("uuid")
	path := c.Query("path")

	var service models.Service
	if err := database.DB.Preload("Node").Where("uuid = ? AND user_id = ?", uuid, userID).First(&service).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
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

	var service models.Service
	if err := database.DB.Preload("Node").Where("uuid = ? AND user_id = ?", uuid, userID).First(&service).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
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
