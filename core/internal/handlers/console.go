package handlers

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/luketaylor45/atlas/core/internal/utils"
)

// The ticket is the authentication for the console socket, so the browser's
// origin does not need to match Core's.
var consoleUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

const consoleWriteTimeout = 10 * time.Second

// CreateConsoleTicket issues a short-lived ticket for the console websocket
// after checking that the caller may view this service's console.
func CreateConsoleTicket(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	uuid := c.Param("uuid")

	service, subUser, ok := utils.FindServiceForUser(uuid, userID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found or no access"})
		return
	}
	if subUser != nil && !subUser.CanViewConsole {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to view this server's console"})
		return
	}

	ticket, err := utils.GenerateConsoleTicket(userID, service.UUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to issue console ticket"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ticket": ticket})
}

// ServiceConsoleSocket relays a service's console from its node to the
// browser. The daemon is never exposed to the browser and the node token never
// leaves Core.
func ServiceConsoleSocket(c *gin.Context) {
	uuid := c.Param("uuid")

	claims, err := utils.ValidateConsoleTicket(c.Query("ticket"), uuid)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired console ticket"})
		return
	}

	// Access is re-checked at connect time in case it was revoked after the
	// ticket was issued.
	service, subUser, ok := utils.FindServiceForUser(uuid, claims.UserID)
	if !ok || (subUser != nil && !subUser.CanViewConsole) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Console access denied"})
		return
	}

	client, err := consoleUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer client.Close()

	header := http.Header{}
	header.Set("X-Node-Token", service.Node.Token)
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	upstream, resp, err := dialer.Dial(nodeWebsocketURL(&service.Node, fmt.Sprintf("/api/servers/%s/console", service.UUID)), header)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		log.Printf("[Core] Console relay for %s could not reach node %s: %v", service.UUID, service.Node.Name, err)
		_ = client.WriteMessage(websocket.TextMessage, []byte("[Atlas] The node hosting this server is unreachable."))
		return
	}
	defer upstream.Close()

	// The browser never sends console data over this socket (commands go
	// through the API), so reading is only used to notice that it went away.
	go func() {
		for {
			if _, _, err := client.ReadMessage(); err != nil {
				upstream.Close()
				return
			}
		}
	}()

	for {
		messageType, data, err := upstream.ReadMessage()
		if err != nil {
			return
		}
		_ = client.SetWriteDeadline(time.Now().Add(consoleWriteTimeout))
		if err := client.WriteMessage(messageType, data); err != nil {
			return
		}
	}
}

// ServiceStats proxies live resource usage from the node.
func ServiceStats(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	uuid := c.Param("uuid")

	service, subUser, ok := utils.FindServiceForUser(uuid, userID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found or no access"})
		return
	}
	if subUser != nil && !subUser.CanViewConsole {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to view this server's usage"})
		return
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, nodeURL(&service.Node, fmt.Sprintf("/api/servers/%s/stats", service.UUID)), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare node request"})
		return
	}
	req.Header.Set("X-Node-Token", service.Node.Token)

	resp, err := daemonHTTP.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Node unreachable"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// A node-side auth failure must not look like an expired user session.
		c.JSON(http.StatusBadGateway, gin.H{"error": nodeErrorMessage(resp.Body, "Usage is unavailable")})
		return
	}
	c.DataFromReader(http.StatusOK, resp.ContentLength, "application/json", resp.Body, nil)
}
