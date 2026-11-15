package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
	"github.com/luketaylor45/atlas/core/internal/utils"
)

// NodeRequest carries the editable settings of a node. Pointers distinguish
// a field that was not sent from one set to an empty or zero value. The
// token, heartbeat and usage fields are never accepted from a request.
type NodeRequest struct {
	Name            *string `json:"name"`
	Address         *string `json:"address"`
	InternalAddress *string `json:"internal_address"`
	Port            *string `json:"port"`
	SFTPPort        *string `json:"sftp_port"`
	TotalRAM        *uint64 `json:"total_ram"`
	TotalDisk       *uint64 `json:"total_disk"`
	Location        *string `json:"location"`
}

func validPortString(value string) bool {
	port, err := strconv.Atoi(value)
	return err == nil && port >= 1 && port <= 65535
}

// validHost accepts a hostname or IP address without scheme, path or port.
func validHost(value string) bool {
	if value == "" || len(value) > 253 {
		return false
	}
	return !strings.ContainsAny(value, " /\\?#@\t\r\n") && !strings.Contains(value, "://")
}

// apply validates the request and returns the column updates it describes.
func (r NodeRequest) apply(creating bool) (map[string]interface{}, error) {
	updates := map[string]interface{}{}

	if r.Name != nil {
		name := strings.TrimSpace(*r.Name)
		if name == "" {
			return nil, errors.New("a node name is required")
		}
		updates["name"] = name
	} else if creating {
		return nil, errors.New("a node name is required")
	}

	if r.Address != nil {
		address := strings.TrimSpace(*r.Address)
		if !validHost(address) {
			return nil, errors.New("address must be a hostname or IP address, without http:// or a port")
		}
		updates["address"] = address
	} else if creating {
		return nil, errors.New("a node address is required")
	}

	if r.InternalAddress != nil {
		internal := strings.TrimSpace(*r.InternalAddress)
		if internal != "" && !validHost(internal) {
			return nil, errors.New("internal address must be a hostname or IP address, without http:// or a port")
		}
		updates["internal_address"] = internal
	}

	for column, value := range map[string]*string{"port": r.Port, "sftp_port": r.SFTPPort} {
		if value == nil {
			continue
		}
		port := strings.TrimSpace(*value)
		if !validPortString(port) {
			return nil, fmt.Errorf("%s must be a number between 1 and 65535", strings.ReplaceAll(column, "_", " "))
		}
		updates[column] = port
	}

	if r.TotalRAM != nil {
		updates["total_ram"] = *r.TotalRAM
	}
	if r.TotalDisk != nil {
		updates["total_disk"] = *r.TotalDisk
	}
	if r.Location != nil {
		updates["location"] = strings.TrimSpace(*r.Location)
	}
	return updates, nil
}

// GetNodes returns a list of all nodes
func GetNodes(c *gin.Context) {
	var nodes []models.Node
	if err := database.DB.Order("id").Find(&nodes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch nodes"})
		return
	}

	allocations, err := nodeAllocations()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch node usage"})
		return
	}

	// Online state is kept current by the reconciler.
	result := make([]nodeWithAllocation, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, nodeWithAllocation{Node: node, nodeAllocation: allocations[node.ID]})
	}
	c.JSON(http.StatusOK, result)
}

// nodeAllocation is what the servers on a node have been given, which is
// what limits new servers (not what they happen to be using right now).
type nodeAllocation struct {
	AllocatedMemory uint64 `json:"allocated_memory"` // MB
	AllocatedDisk   uint64 `json:"allocated_disk"`   // MB
	AllocatedCPU    uint64 `json:"allocated_cpu"`    // percent, 100 per core
	Servers         int64  `json:"servers"`
}

type nodeWithAllocation struct {
	models.Node
	nodeAllocation
}

func nodeAllocations() (map[uint]nodeAllocation, error) {
	var rows []struct {
		NodeID uint
		Memory uint64
		Disk   uint64
		Cpu    uint64
		Count  int64
	}
	err := database.DB.Model(&models.Service{}).
		Select("node_id, COALESCE(SUM(memory), 0) AS memory, COALESCE(SUM(disk), 0) AS disk, COALESCE(SUM(cpu), 0) AS cpu, COUNT(*) AS count").
		Group("node_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[uint]nodeAllocation, len(rows))
	for _, row := range rows {
		result[row.NodeID] = nodeAllocation{AllocatedMemory: row.Memory, AllocatedDisk: row.Disk, AllocatedCPU: row.Cpu, Servers: row.Count}
	}
	return result, nil
}

func newNodeToken() string {
	return "n_" + utils.RandomString(48)
}

// CreateNode registers a node and returns its token once.
func CreateNode(c *gin.Context) {
	var req NodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	updates, err := req.apply(true)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	node := models.Node{
		Name:    updates["name"].(string),
		Address: updates["address"].(string),
		Token:   newNodeToken(),
	}
	if value, ok := updates["internal_address"].(string); ok {
		node.InternalAddress = value
	}
	if value, ok := updates["port"].(string); ok {
		node.Port = value
	}
	if value, ok := updates["sftp_port"].(string); ok {
		node.SFTPPort = value
	}
	if value, ok := updates["total_ram"].(uint64); ok {
		node.TotalRAM = value
	}
	if value, ok := updates["total_disk"].(uint64); ok {
		node.TotalDisk = value
	}
	if value, ok := updates["location"].(string); ok && value != "" {
		node.Location = value
	}

	if err := database.DB.Create(&node).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create node"})
		return
	}

	utils.LogActivity(c, 0, "create", "node", fmt.Sprintf("Registered new node: %s", node.Name), nil)

	// The token is hidden from the node's JSON, so it is returned explicitly.
	c.JSON(http.StatusCreated, gin.H{
		"node":  node,
		"token": node.Token,
	})
}

// UpdateNode modifies an existing node
func UpdateNode(c *gin.Context) {
	var node models.Node
	if err := database.DB.First(&node, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	var req NodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	updates, err := req.apply(false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(updates) > 0 {
		// Only the edited columns are written, so a heartbeat arriving at the
		// same moment is not overwritten.
		if err := database.DB.Model(&models.Node{}).Where("id = ?", node.ID).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update node"})
			return
		}
		database.DB.First(&node, node.ID)
		utils.LogActivity(c, 0, "update", "node", fmt.Sprintf("Updated configuration for node: %s", node.Name), nil)
	}

	c.JSON(http.StatusOK, node)
}

// RegenerateNodeToken replaces a node's token and returns the new one once.
// The daemon must be reconfigured with it; until then the node is offline.
func RegenerateNodeToken(c *gin.Context) {
	var node models.Node
	if err := database.DB.First(&node, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	token := newNodeToken()
	if err := database.DB.Model(&models.Node{}).Where("id = ?", node.ID).Updates(map[string]interface{}{
		"token":     token,
		"is_online": false,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to regenerate token"})
		return
	}

	utils.LogActivity(c, 0, "update", "node", fmt.Sprintf("Regenerated token for node: %s", node.Name), nil)
	c.JSON(http.StatusOK, gin.H{"token": token})
}

// DeleteNode removes a node that hosts no servers.
func DeleteNode(c *gin.Context) {
	var node models.Node
	if err := database.DB.First(&node, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	var serviceCount int64
	database.DB.Model(&models.Service{}).Where("node_id = ?", node.ID).Count(&serviceCount)
	if serviceCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("This node hosts %d server(s). Delete them first.", serviceCount)})
		return
	}

	// The token column is unique. Clearing it lets the deleted row stay for
	// history without ever authenticating again.
	err := database.DB.Model(&models.Node{}).Where("id = ?", node.ID).Updates(map[string]interface{}{
		"token":     fmt.Sprintf("deleted-%d-%s", node.ID, utils.RandomString(16)),
		"is_online": false,
	}).Error
	if err == nil {
		err = database.DB.Delete(&models.Node{}, node.ID).Error
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete node"})
		return
	}

	utils.LogActivity(c, 0, "delete", "node", fmt.Sprintf("Removed node: %s", node.Name), nil)
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
