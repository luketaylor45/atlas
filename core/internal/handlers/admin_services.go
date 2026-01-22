package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
)

type CreateServiceRequest struct {
	Name        string `json:"name"`
	NodeID      uint   `json:"node_id"`
	UserID      uint   `json:"user_id"` // Explicit owner
	EggID       uint   `json:"egg_id"`
	Memory      uint64 `json:"memory"`
	Disk        uint64 `json:"disk"`
	Cpu         uint64 `json:"cpu"`
	Port        int    `json:"port"`
	Environment string `json:"environment"`
	DockerImage string `json:"docker_image"`
}

func checkNodeResources(nodeID uint, reqMem, reqDisk uint64, excludeServiceID uint) error {
	var node models.Node
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		return fmt.Errorf("node not found")
	}

	if node.TotalRAM == 0 || node.TotalDisk == 0 {
		return nil // Resource tracking disabled or not set
	}

	var services []models.Service
	query := database.DB.Where("node_id = ?", nodeID)
	if excludeServiceID > 0 {
		query = query.Where("id != ?", excludeServiceID)
	}
	if err := query.Find(&services).Error; err != nil {
		return fmt.Errorf("failed to fetch existing services")
	}

	var usedMem, usedDisk uint64
	for _, s := range services {
		usedMem += s.Memory
		usedDisk += s.Disk
	}

	if usedMem+reqMem > node.TotalRAM {
		return fmt.Errorf("insufficient RAM on node (Available: %dMB, Requested: %dMB)", node.TotalRAM-usedMem, reqMem)
	}
	if usedDisk+reqDisk > node.TotalDisk {
		return fmt.Errorf("insufficient Disk on node (Available: %dMB, Requested: %dMB)", node.TotalDisk-usedDisk, reqDisk)
	}

	return nil
}

// GetServices returns all services with their relations
func GetServices(c *gin.Context) {
	var services []models.Service
	if err := database.DB.Preload("Node").Preload("Egg.Nest").Preload("Egg.Variables").Preload("User").Find(&services).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch services"})
		return
	}
	c.JSON(http.StatusOK, services)
}

// GetEggs returns all available service templates
func GetEggs(c *gin.Context) {
	var eggs []models.Egg
	if err := database.DB.Preload("Nest").Preload("Variables").Find(&eggs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch service templates"})
		return
	}
	c.JSON(http.StatusOK, eggs)
}

// GetNests returns all available nests for categorization
func GetNests(c *gin.Context) {
	var nests []models.Nest
	if err := database.DB.Preload("Eggs.Variables").Find(&nests).Error; err != nil {
		log.Printf("Error fetching nests: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch nests: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, nests)
}

type CreateNestRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// CreateNest creates a new nest category
func CreateNest(c *gin.Context) {
	var req CreateNestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	nest := models.Nest{
		UUID:        uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
	}

	if err := database.DB.Create(&nest).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create nest: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, nest)
}

// CreateService handles the creation of a new service instance
func CreateService(c *gin.Context) {
	var req CreateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. Fetch Node
	var node models.Node
	if err := database.DB.First(&node, req.NodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	// 1.5 Check Resources
	if err := checkNodeResources(req.NodeID, req.Memory, req.Disk, 0); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 2. Fetch Egg
	var egg models.Egg
	if err := database.DB.First(&egg, req.EggID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service type not found"})
		return
	}

	// 3. Create Service Record
	service := models.Service{
		UUID:        uuid.New().String(),
		Name:        req.Name,
		UserID:      req.UserID,
		NodeID:      req.NodeID,
		EggID:       req.EggID,
		Memory:      req.Memory,
		Disk:        req.Disk,
		Cpu:         req.Cpu,
		Port:        req.Port,
		Environment: req.Environment,
		DockerImage: req.DockerImage,
	}

	// Default to first image if none selected
	if service.DockerImage == "" {
		var images []string
		json.Unmarshal([]byte(egg.DockerImages), &images)
		if len(images) > 0 {
			service.DockerImage = images[0]
		}
	}

	if err := database.DB.Create(&service).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create service record"})
		return
	}

	// 4. Send Create Request to Daemon
	if err := notifyDaemon(&node, &service, &egg); err != nil {
		database.DB.Delete(&service)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to contact node: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, service)
}

func notifyDaemon(node *models.Node, service *models.Service, egg *models.Egg) error {
	url := fmt.Sprintf("http://%s:%s/api/servers", node.Address, node.Port)

	eggImage := service.DockerImage
	if eggImage == "" {
		var images []string
		json.Unmarshal([]byte(egg.DockerImages), &images)
		if len(images) > 0 {
			eggImage = images[0]
		}
	}

	payload := map[string]interface{}{
		"uuid":              service.UUID,
		"memory":            service.Memory,
		"disk":              service.Disk,
		"cpu":               service.Cpu,
		"port":              service.Port,
		"egg_image":         eggImage,
		"startup_command":   egg.StartupCommand,
		"environment":       service.Environment, // Use service overrides
		"install_script":    egg.InstallScript,
		"install_container": egg.InstallContainer,
	}

	body, _ := json.Marshal(payload)
	client := &http.Client{}
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("X-Node-Token", node.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[Core] !! Daemon Connection Error !! URL: %s, Error: %v", url, err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var daemonErr struct {
			Error   string `json:"error"`
			Details string `json:"details"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&daemonErr); err == nil && daemonErr.Error != "" {
			log.Printf("[Core] Daemon responded with error: %s - %s", daemonErr.Error, daemonErr.Details)
			return fmt.Errorf("%s: %s", daemonErr.Error, daemonErr.Details)
		}
		log.Printf("[Core] Daemon responded with status %d", resp.StatusCode)
		return fmt.Errorf("daemon responded with %d", resp.StatusCode)
	}

	return nil
}

// DeleteService removes a service from DB and Node
func DeleteService(c *gin.Context) {
	serviceID := c.Param("id")

	var service models.Service
	if err := database.DB.Preload("Node").First(&service, serviceID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	if err := notifyDaemonDelete(&service.Node, service.UUID); err != nil {
		log.Printf("Failed to notify node for deletion: %v", err)
	}

	if err := database.DB.Unscoped().Delete(&service).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete from database"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func notifyDaemonDelete(node *models.Node, uuid string) error {
	url := fmt.Sprintf("http://%s:%s/api/servers/%s", node.Address, node.Port, uuid)

	client := &http.Client{}
	req, _ := http.NewRequest("DELETE", url, nil)
	req.Header.Set("X-Node-Token", node.Token)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("node responded with %d", resp.StatusCode)
	}

	return nil
}

// UpdateService modifies an existing service record
func UpdateService(c *gin.Context) {
	serviceID := c.Param("id")
	var service models.Service
	if err := database.DB.Preload("Node").Preload("Egg").First(&service, serviceID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	var req models.Service
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check resources if limits changed
	if req.Memory != service.Memory || req.Disk != service.Disk {
		if err := checkNodeResources(service.NodeID, req.Memory, req.Disk, service.ID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	// Update fields
	service.Name = req.Name
	service.Memory = req.Memory
	service.Disk = req.Disk
	service.Cpu = req.Cpu
	service.DockerImage = req.DockerImage

	if err := database.DB.Save(&service).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update service"})
		return
	}

	// Notify daemon of resource changes
	if err := notifyDaemonUpdate(&service.Node, &service, &service.Egg); err != nil {
		log.Printf("[Core] Failed to update daemon for service %s: %v", service.UUID, err)
	}

	c.JSON(http.StatusOK, service)
}

func notifyDaemonUpdate(node *models.Node, service *models.Service, egg *models.Egg) error {
	url := fmt.Sprintf("http://%s:%s/api/servers/%s", node.Address, node.Port, service.UUID)

	payload := map[string]interface{}{
		"memory":          service.Memory,
		"disk":            service.Disk,
		"cpu":             service.Cpu,
		"port":            service.Port,
		"docker_image":    service.DockerImage,
		"startup_command": egg.StartupCommand,
		"environment":     service.Environment,
	}

	body, _ := json.Marshal(payload)
	client := &http.Client{}
	req, _ := http.NewRequest("PUT", url, bytes.NewBuffer(body))
	req.Header.Set("X-Node-Token", node.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("daemon responded with %d", resp.StatusCode)
	}
	return nil
}

// UpdateNest modifies an existing nest
func UpdateNest(c *gin.Context) {
	nestID := c.Param("id")
	var nest models.Nest
	if err := database.DB.First(&nest, nestID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Nest not found"})
		return
	}

	if err := c.ShouldBindJSON(&nest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := database.DB.Save(&nest).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update nest"})
		return
	}

	c.JSON(http.StatusOK, nest)
}

// DeleteNest removes a nest
func DeleteNest(c *gin.Context) {
	nestID := c.Param("id")
	if err := database.DB.Delete(&models.Nest{}, nestID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete nest"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
