package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/daemon/internal/config"
)

const testServiceUUID = "14778ff7-a24f-43c5-b6d3-ec2febce7177"

func guardedRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	server := router.Group("/api/servers", RequireNodeToken()).Group("/:uuid", RequireServiceUUID())
	reached := func(c *gin.Context) { c.Status(http.StatusNoContent) }
	server.GET("/console", reached)
	server.GET("/stats", reached)
	server.POST("/command", reached)
	return router
}

func request(router *gin.Engine, method, path, token string) int {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("X-Node-Token", token)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder.Code
}

func TestConsoleStatsAndCommandRequireNodeToken(t *testing.T) {
	previous := config.NodeConfig
	config.NodeConfig.NodeToken = "n_real-token"
	t.Cleanup(func() { config.NodeConfig = previous })

	router := guardedRouter()
	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/servers/" + testServiceUUID + "/console"},
		{http.MethodGet, "/api/servers/" + testServiceUUID + "/stats"},
		{http.MethodPost, "/api/servers/" + testServiceUUID + "/command"},
	}

	for _, route := range routes {
		if code := request(router, route.method, route.path, ""); code != http.StatusUnauthorized {
			t.Fatalf("%s %s without a token returned %d, want 401", route.method, route.path, code)
		}
		if code := request(router, route.method, route.path, "wrong"); code != http.StatusUnauthorized {
			t.Fatalf("%s %s with a wrong token returned %d, want 401", route.method, route.path, code)
		}
		if code := request(router, route.method, route.path, "n_real-token"); code != http.StatusNoContent {
			t.Fatalf("%s %s with the node token returned %d, want 204", route.method, route.path, code)
		}
	}
}

func TestPlaceholderNodeTokenNeverAuthenticates(t *testing.T) {
	previous := config.NodeConfig
	t.Cleanup(func() { config.NodeConfig = previous })

	for _, placeholder := range []string{"", "change-me"} {
		config.NodeConfig.NodeToken = placeholder
		code := request(guardedRouter(), http.MethodGet, "/api/servers/"+testServiceUUID+"/stats", placeholder)
		if code != http.StatusServiceUnavailable {
			t.Fatalf("placeholder token %q returned %d, want 503", placeholder, code)
		}
	}
}

func TestServiceIdentifierMustBeAUUID(t *testing.T) {
	previous := config.NodeConfig
	config.NodeConfig.NodeToken = "n_real-token"
	t.Cleanup(func() { config.NodeConfig = previous })

	for _, identifier := range []string{"..", "atlas_db", "service-uuid", "14778ff7"} {
		code := request(guardedRouter(), http.MethodGet, "/api/servers/"+identifier+"/stats", "n_real-token")
		if code != http.StatusBadRequest {
			t.Fatalf("identifier %q returned %d, want 400", identifier, code)
		}
	}
}
