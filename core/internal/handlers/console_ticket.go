package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/utils"
)

const consoleTicketTTL = 5 * time.Minute

// GetConsoleTicket issues a short-lived, per-server ticket that lets the browser
// reach the daemon's console/stats routes without ever seeing the node token.
// Ticket format: "<unix expiry>.<hex HMAC-SHA256(node token, uuid|expiry)>".
func GetConsoleTicket(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	// Admins and owners pass; sub-users need console permission.
	service, subUser, ok := utils.FindServiceForUser(c.Param("uuid"), userID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found or no access"})
		return
	}
	if subUser != nil && !subUser.CanViewConsole {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to view this console"})
		return
	}

	expiry := time.Now().Add(consoleTicketTTL).Unix()
	mac := hmac.New(sha256.New, []byte(service.Node.Token))
	mac.Write([]byte(service.UUID + "|" + strconv.FormatInt(expiry, 10)))

	c.JSON(http.StatusOK, gin.H{
		"ticket":     strconv.FormatInt(expiry, 10) + "." + hex.EncodeToString(mac.Sum(nil)),
		"expires_at": expiry,
	})
}
