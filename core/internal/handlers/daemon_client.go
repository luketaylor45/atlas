package handlers

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/luketaylor45/atlas/core/internal/models"
)

// daemonHTTP is used for ordinary control requests to a node.
var daemonHTTP = &http.Client{Timeout: 60 * time.Second}

// daemonTransferHTTP is used for uploads, which can legitimately take long.
var daemonTransferHTTP = &http.Client{Timeout: 30 * time.Minute}

// nodeHost returns the host:port Core uses to reach a node's daemon. The
// internal address, when set, keeps daemon traffic off the public interface;
// the public address is what players and SFTP clients connect to.
func nodeHost(node *models.Node) string {
	host := strings.TrimSpace(node.InternalAddress)
	if host == "" {
		host = strings.TrimSpace(node.Address)
	}
	port := strings.TrimSpace(node.Port)
	if port == "" {
		port = "8081"
	}
	return net.JoinHostPort(host, port)
}

func nodeURL(node *models.Node, path string) string {
	return "http://" + nodeHost(node) + path
}

func nodeWebsocketURL(node *models.Node, path string) string {
	return "ws://" + nodeHost(node) + path
}
