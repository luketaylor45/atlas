package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
	"github.com/luketaylor45/atlas/core/internal/utils"
	"gorm.io/gorm"
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
	// ExtraPorts maps an egg port's variable (for example RCON_PORT) to the
	// port to use. Ports left out are assigned automatically.
	ExtraPorts map[string]int `json:"extra_ports"`
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
		return fmt.Errorf("insufficient RAM on node (Available: %dMB, Requested: %dMB)", saturatingSub(node.TotalRAM, usedMem), reqMem)
	}
	if usedDisk+reqDisk > node.TotalDisk {
		return fmt.Errorf("insufficient Disk on node (Available: %dMB, Requested: %dMB)", saturatingSub(node.TotalDisk, usedDisk), reqDisk)
	}

	return nil
}

func saturatingSub(a, b uint64) uint64 {
	if b > a {
		return 0
	}
	return a - b
}

func validateServicePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

// GetServices returns all services with their relations
func GetServices(c *gin.Context) {
	var services []models.Service
	if err := database.DB.Preload("Node").Preload("Egg.Nest").Preload("Egg.Variables").Preload("User").Preload("Ports").Find(&services).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch services"})
		return
	}
	c.JSON(http.StatusOK, services)
}

// CreateService handles the creation of a new service instance
func CreateService(c *gin.Context) {
	var req CreateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "A server name is required"})
		return
	}
	if req.Memory == 0 || req.Cpu == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Memory and CPU limits must be greater than zero"})
		return
	}

	var node models.Node
	if err := database.DB.First(&node, req.NodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	var owner models.User
	if err := database.DB.First(&owner, req.UserID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "The selected owner does not exist"})
		return
	}

	if err := checkNodeResources(req.NodeID, req.Memory, req.Disk, 0); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var egg models.Egg
	if err := database.DB.Preload("Variables").Where("available = ?", true).First(&egg, req.EggID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service type not found"})
		return
	}

	environment, err := validateEnvironmentOverrides(req.Environment, &egg, "", true)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	image := strings.TrimSpace(req.DockerImage)
	if image == "" {
		images := eggImages(&egg)
		if len(images) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "This service type has no Docker image configured"})
			return
		}
		image = images[0]
	}

	service := models.Service{
		UUID:                 uuid.New().String(),
		Name:                 req.Name,
		UserID:               req.UserID,
		NodeID:               req.NodeID,
		EggID:                req.EggID,
		Memory:               req.Memory,
		Disk:                 req.Disk,
		Cpu:                  req.Cpu,
		Port:                 req.Port,
		StartupCommand:       egg.StartupCommand,
		Environment:          environment,
		DockerImage:          image,
		Status:               StatusInstalling,
		InstallationStage:    "Provisioning queued",
		InstallationProgress: 0,
	}

	// The service and its ports are created together. The unique index on
	// (node, port) is the final arbiter if two requests race for a port.
	var planErr error
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		used, err := usedNodePorts(tx, req.NodeID, 0)
		if err != nil {
			return err
		}
		plan, err := planServicePorts(used, req.Port, eggPorts(&egg), req.ExtraPorts)
		if err != nil {
			planErr = err
			return err
		}
		if err := tx.Create(&service).Error; err != nil {
			return err
		}
		for i := range plan {
			plan[i].ServiceID = service.ID
			plan[i].NodeID = service.NodeID
		}
		if err := tx.Create(&plan).Error; err != nil {
			return err
		}
		service.Ports = plan
		return nil
	})
	switch {
	case planErr != nil:
		status := http.StatusBadRequest
		if errors.Is(planErr, errPortTaken) {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": planErr.Error()})
		return
	case isUniqueViolation(err):
		c.JSON(http.StatusConflict, gin.H{"error": "One of the requested ports was just taken by another service. Try again."})
		return
	case err != nil:
		log.Printf("[Core] Failed to create service: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create service record"})
		return
	}

	service.Node = node
	service.Egg = egg
	if err := daemonDo(daemonHTTP, &node, http.MethodPost, "/api/servers", buildServerSpec(&service)); err != nil {
		// Nothing exists on the node, so the record and its ports are removed
		// outright rather than left behind as a phantom installing service.
		if purgeErr := purgeService(&service); purgeErr != nil {
			log.Printf("[Core] Failed to clean up service %s after a failed create: %v", service.UUID, purgeErr)
		}
		c.JSON(nodeErrorStatus(err), gin.H{"error": "Failed to start provisioning on the node: " + err.Error()})
		return
	}

	utils.LogActivity(c, service.ID, "create", "service", fmt.Sprintf("Provisioned new service: %s", service.Name), nil)

	c.JSON(http.StatusCreated, service)
}

// purgeService permanently removes a service and everything that references it.
func purgeService(service *models.Service) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("service_id = ?", service.ID).Delete(&models.ServicePort{}).Error; err != nil {
			return err
		}
		if err := tx.Where("service_id = ?", service.ID).Delete(&models.ServiceUser{}).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM activity_logs WHERE service_id = ?", service.ID).Error; err != nil {
			return err
		}
		if err := deleteServiceBackups(tx, service.ID); err != nil {
			return err
		}
		if err := tx.Where("service_id = ?", service.ID).Delete(&models.Schedule{}).Error; err != nil {
			return err
		}
		return tx.Unscoped().Delete(&models.Service{}, service.ID).Error
	})
}

// moveServicePorts gives a service a new primary port. Ports the game derives
// from the primary one (see EggPort.Offset) move with it; the service's other
// ports stay where they are.
func moveServicePorts(tx *gorm.DB, service *models.Service, newPort int) error {
	var egg models.Egg
	derived := map[string]models.EggPort{}
	if err := tx.First(&egg, service.EggID).Error; err == nil {
		for _, port := range eggPorts(&egg) {
			if port.Offset != 0 {
				derived[port.Env] = port
			}
		}
	}

	// Everything held by other services, plus this service's own ports that
	// are staying put.
	used, err := usedNodePorts(tx, service.NodeID, service.ID)
	if err != nil {
		return err
	}
	var own []models.ServicePort
	if err := tx.Where("service_id = ?", service.ID).Find(&own).Error; err != nil {
		return err
	}
	hasPrimary := false
	for _, port := range own {
		_, moves := derived[port.Env]
		if port.IsPrimary {
			hasPrimary = true
		} else if !moves {
			used[port.Port] = true
		}
	}

	if used[newPort] {
		return &portTakenError{Port: newPort}
	}
	targets := map[string]int{}
	claimed := map[int]bool{newPort: true}
	for env, port := range derived {
		target := newPort + port.Offset
		if err := validateServicePort(target); err != nil {
			return fmt.Errorf("%s (the game port plus %d): %w", port.Name, port.Offset, err)
		}
		if used[target] || claimed[target] {
			return &derivedPortTakenError{portTakenError: portTakenError{Port: target}, Name: port.Name, Offset: port.Offset}
		}
		claimed[target] = true
		targets[env] = target
	}

	// The derived rows are removed first so that shifting every port by one
	// never trips over the service's own old allocations.
	for env := range derived {
		if err := tx.Where("service_id = ? AND env = ? AND is_primary = ?", service.ID, env, false).Delete(&models.ServicePort{}).Error; err != nil {
			return err
		}
	}
	if hasPrimary {
		if err := tx.Model(&models.ServicePort{}).Where("service_id = ? AND is_primary = ?", service.ID, true).Update("port", newPort).Error; err != nil {
			return err
		}
	} else {
		primary := models.ServicePort{ServiceID: service.ID, NodeID: service.NodeID, Port: newPort, Env: models.PrimaryPortEnv, Name: "Game", Protocol: "both", IsPrimary: true}
		if err := tx.Create(&primary).Error; err != nil {
			return err
		}
	}
	for env, target := range targets {
		port := derived[env]
		row := models.ServicePort{ServiceID: service.ID, NodeID: service.NodeID, Port: target, Env: env, Name: port.Name, Protocol: normalizeProtocol(port.Protocol)}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

// DeleteService removes a service from its node and then from the database.
// If the node cannot be reached the service is kept, because deleting only
// the record would orphan its container, files and ports. An admin can pass
// ?force=true to remove the record anyway (for a node that is gone for good).
func DeleteService(c *gin.Context) {
	serviceID := c.Param("id")
	force := c.Query("force") == "true"

	var service models.Service
	if err := database.DB.Preload("Node").First(&service, serviceID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	if err := daemonDo(daemonHTTP, &service.Node, http.MethodDelete, "/api/servers/"+service.UUID, nil); err != nil {
		if !force {
			c.JSON(nodeErrorStatus(err), gin.H{
				"error":     "The node could not remove this server, so it was not deleted: " + err.Error(),
				"can_force": true,
			})
			return
		}
		log.Printf("[Core] Force-deleting service %s although its node failed: %v", service.UUID, err)
	}

	if err := purgeService(&service); err != nil {
		log.Printf("[Core] Error deleting service from database: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "The server was removed from its node but not from the database: " + err.Error()})
		return
	}

	utils.LogActivity(c, 0, "delete", "service", fmt.Sprintf("Permanently deleted service: %s", service.Name), map[string]interface{}{"uuid": service.UUID})

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// UpdateServiceRequest carries only the fields being changed; a field that
// is not sent is left as it is.
type UpdateServiceRequest struct {
	Name        *string `json:"name"`
	Memory      *uint64 `json:"memory"`
	Disk        *uint64 `json:"disk"`
	Cpu         *uint64 `json:"cpu"`
	DockerImage *string `json:"docker_image"`
	Port        *int    `json:"port"`
	BackupLimit *int    `json:"backup_limit"`
}

// UpdateService changes a service's name, limits, image or primary port.
// A new port or image needs a new container, which is rebuilt in place; the
// server's files are never touched.
func UpdateService(c *gin.Context) {
	var existing models.Service
	if err := database.DB.First(&existing, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	var req UpdateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	memory, disk := existing.Memory, existing.Disk

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "A server name is required"})
			return
		}
		updates["name"] = name
	}
	if req.Memory != nil {
		if *req.Memory == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Memory must be greater than zero"})
			return
		}
		memory = *req.Memory
		updates["memory"] = memory
	}
	if req.Disk != nil {
		disk = *req.Disk
		updates["disk"] = disk
	}
	if req.Cpu != nil {
		if *req.Cpu == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "CPU must be greater than zero"})
			return
		}
		updates["cpu"] = *req.Cpu
	}
	if req.BackupLimit != nil {
		if *req.BackupLimit < 0 || *req.BackupLimit > maxBackupLimit {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("The backup limit must be between 0 and %d", maxBackupLimit)})
			return
		}
		updates["backup_limit"] = *req.BackupLimit
	}
	if memory != existing.Memory || disk != existing.Disk {
		if err := checkNodeResources(existing.NodeID, memory, disk, existing.ID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	needsRebuild := false
	if req.DockerImage != nil {
		image := strings.TrimSpace(*req.DockerImage)
		if image != "" && image != existing.DockerImage {
			updates["docker_image"] = image
			needsRebuild = true
		}
	}

	newPort := existing.Port
	if req.Port != nil && *req.Port != existing.Port {
		if err := validateServicePort(*req.Port); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		newPort = *req.Port
		updates["port"] = newPort
		needsRebuild = true
	}

	if existing.Status == StatusInstalling && needsRebuild {
		c.JSON(http.StatusConflict, gin.H{"error": "The port and image cannot be changed while the server is installing"})
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if newPort != existing.Port {
			if err := moveServicePorts(tx, &existing, newPort); err != nil {
				return err
			}
		}
		if len(updates) == 0 {
			return nil
		}
		return tx.Model(&models.Service{}).Where("id = ?", existing.ID).Updates(updates).Error
	})
	if err != nil {
		var derivedTaken *derivedPortTakenError
		if errors.As(err, &derivedTaken) {
			c.JSON(http.StatusConflict, gin.H{"error": derivedTaken.Error()})
			return
		}
		if errors.Is(err, errPortTaken) || isUniqueViolation(err) {
			c.JSON(http.StatusConflict, gin.H{"error": (&portTakenError{Port: newPort}).Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update service"})
		return
	}

	service, err := loadServiceForNode(existing.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reload service"})
		return
	}

	utils.LogActivity(c, service.ID, "update", "service", fmt.Sprintf("Modified configuration for: %s", service.Name), nil)

	// Tell the node. The change is already saved, so a node failure is
	// reported as a warning the admin can act on, not as a failed save.
	var nodeErr error
	switch {
	case needsRebuild:
		nodeErr = rebuildOnNode(service)
	case service.Status != StatusInstalling && service.Status != StatusInstallationFailed:
		nodeErr = daemonDo(daemonHTTP, &service.Node, http.MethodPut, "/api/servers/"+service.UUID, withEggRuntime(map[string]interface{}{
			"memory":          service.Memory,
			"disk":            service.Disk,
			"cpu":             service.Cpu,
			"port":            service.Port,
			"docker_image":    service.DockerImage,
			"startup_command": resolveStartupCommand(service),
			"environment":     mustJSON(mergedEnvironment(service)),
		}, &service.Egg))
	}
	if nodeErr != nil {
		log.Printf("[Core] Failed to apply update for service %s on its node: %v", service.UUID, nodeErr)
		c.JSON(http.StatusOK, gin.H{
			"service": service,
			"warning": "Saved, but the node did not apply the change: " + nodeErr.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"service": service})
}

func mustJSON(value interface{}) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

// rebuildOnNode recreates the service's container from its current settings.
func rebuildOnNode(service *models.Service) error {
	path := "/api/servers/" + service.UUID + "/rebuild"
	if stop := strings.TrimSpace(service.Egg.StopCommand); stop != "" {
		path += "?stop_command=" + url.QueryEscape(stop)
	}
	// Pulling a new image can take a while.
	return daemonDo(daemonTransferHTTP, &service.Node, http.MethodPost, path, buildServerSpec(service))
}
