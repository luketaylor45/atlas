package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/daemon/internal/config"
)

// TokenEqual compares two secrets in constant time.
func TokenEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func validNodeToken(c *gin.Context) bool {
	// Fail closed: a missing/placeholder/short token never authenticates.
	if config.ValidateNodeToken() != nil {
		return false
	}
	return TokenEqual(c.GetHeader("X-Node-Token"), config.NodeConfig.NodeToken)
}

// NodeAuth requires the node token on every request it guards.
func NodeAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !validNodeToken(c) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		c.Next()
	}
}

// signTicket returns the HMAC of "uuid|expiry" keyed with the node token.
func signTicket(uuid string, expiry int64) string {
	mac := hmac.New(sha256.New, []byte(config.NodeConfig.NodeToken))
	mac.Write([]byte(uuid + "|" + strconv.FormatInt(expiry, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

// ValidTicket verifies a "<expiry>.<hmac>" ticket issued by the Core for this server.
func ValidTicket(ticket, uuid string) bool {
	if config.ValidateNodeToken() != nil {
		return false
	}
	parts := strings.SplitN(ticket, ".", 2)
	if len(parts) != 2 {
		return false
	}
	expiry, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().Unix() > expiry {
		return false
	}
	return TokenEqual(parts[1], signTicket(uuid, expiry))
}

// NodeOrTicketAuth accepts either the node token or a short-lived per-server ticket
// (?ticket=...), so browsers can reach the console and stats without the node token.
func NodeOrTicketAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if validNodeToken(c) {
			c.Next()
			return
		}
		if t := c.Query("ticket"); t != "" && ValidTicket(t, c.Param("uuid")) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
	}
}
