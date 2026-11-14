package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
)

const nodeContextKey = "node"

// NodeAuthMiddleware authenticates a daemon by its node token. Every internal
// route sits behind it, and handlers scope their work to the calling node.
func NodeAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Node-Token")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Node token is required"})
			return
		}

		var node models.Node
		if err := database.DB.Where("token = ?", token).First(&node).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid node token"})
			return
		}

		c.Set(nodeContextKey, &node)
		c.Next()
	}
}

// NodeFromContext returns the daemon's node, set by NodeAuthMiddleware.
func NodeFromContext(c *gin.Context) *models.Node {
	return c.MustGet(nodeContextKey).(*models.Node)
}
