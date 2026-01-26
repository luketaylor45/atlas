package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
	"github.com/luketaylor45/atlas/core/internal/utils"
)

// GetNodes returns a list of all nodes
func GetNodes(c *gin.Context) {
	var nodes []models.Node
	if err := database.DB.Find(&nodes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch nodes"})
		return
	}

	// Update online status based on heartbeat
	now := time.Now()
	for i := range nodes {
		// If last heartbeat was > 30 seconds ago, mark offline
		if nodes[i].LastHeartbeat.IsZero() || now.Sub(nodes[i].LastHeartbeat) > 30*time.Second {
			if nodes[i].IsOnline {
				nodes[i].IsOnline = false
				database.DB.Model(&nodes[i]).Update("is_online", false)
			}
		}
	}

	c.JSON(http.StatusOK, nodes)
}

// CreateNode adds a new node manually
func CreateNode(c *gin.Context) {
	var req models.Node
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate a secure token for the node
	// Re-using a simple helper or just uuid.
	req.Token = "n_" + utils.RandomString(32)

	if err := database.DB.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create node"})
		return
	}

	utils.LogActivity(c, 0, "create", "node", fmt.Sprintf("Registered new node: %s", req.Name), nil)

	// Return the token explicitly since it's hidden in the JSON struct output
	c.JSON(http.StatusCreated, gin.H{
		"node":  req,
		"token": req.Token,
	})
}

// UpdateNode modifies an existing node
func UpdateNode(c *gin.Context) {
	nodeID := c.Param("id")
	var node models.Node
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := database.DB.Save(&node).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update node"})
		return
	}

	utils.LogActivity(c, 0, "update", "node", fmt.Sprintf("Updated configuration for node: %s", node.Name), nil)

	c.JSON(http.StatusOK, node)
}

// DeleteNode removes a node
func DeleteNode(c *gin.Context) {
	nodeID := c.Param("id")
	if err := database.DB.Delete(&models.Node{}, nodeID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete node"})
		return
	}

	utils.LogActivity(c, 0, "delete", "node", fmt.Sprintf("Removed node ID: %s from the network", nodeID), nil)

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
