package api

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/daemon/internal/config"
	"github.com/luketaylor45/atlas/daemon/internal/serverfs"
)

// insecureNodeTokens are placeholder values that must never authenticate a request.
var insecureNodeTokens = map[string]bool{
	"":          true,
	"change-me": true,
}

// NodeTokenConfigured reports whether the daemon has a real node token.
func NodeTokenConfigured() bool {
	return !insecureNodeTokens[config.NodeConfig.NodeToken]
}

func validNodeToken(candidate string) bool {
	if !NodeTokenConfigured() || candidate == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(config.NodeConfig.NodeToken)) == 1
}

// RequireNodeToken rejects every request that does not carry the node token.
// All daemon API routes sit behind it; only Core holds the token.
func RequireNodeToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !NodeTokenConfigured() {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "This node has no token configured. Set NODE_TOKEN to the token shown in Admin > Nodes and restart the daemon.",
			})
			return
		}
		if !validNodeToken(c.GetHeader("X-Node-Token")) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		c.Next()
	}
}

// RequireServiceUUID ensures the :uuid route parameter is a canonical UUID, so
// it can never be used to address another directory or container.
func RequireServiceUUID() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !serverfs.IsServiceDirectoryName(c.Param("uuid")) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid service identifier"})
			return
		}
		c.Next()
	}
}
