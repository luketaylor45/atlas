package api

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/luketaylor45/atlas/daemon/internal/config"
)

func TestPostCoreJSONReusesConnection(t *testing.T) {
	var connections atomic.Int32
	var requests atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = io.WriteString(w, strings.Repeat("ok", 2048))
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	server.Start()
	defer server.Close()

	previousURL := config.NodeConfig.CoreURL
	previousClient := coreHTTPClient
	transport := &http.Transport{
		MaxIdleConns:        2,
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     time.Minute,
	}
	coreHTTPClient = &http.Client{Timeout: time.Second, Transport: transport}
	config.NodeConfig.CoreURL = server.URL
	t.Cleanup(func() {
		transport.CloseIdleConnections()
		coreHTTPClient = previousClient
		config.NodeConfig.CoreURL = previousURL
	})

	for range 3 {
		if err := postCoreJSON("/heartbeat", map[string]string{"token": "test"}); err != nil {
			t.Fatalf("postCoreJSON failed: %v", err)
		}
	}

	if requests.Load() != 3 {
		t.Fatalf("expected 3 requests, got %d", requests.Load())
	}
	if connections.Load() != 1 {
		t.Fatalf("expected one reused TCP connection, got %d", connections.Load())
	}
}
