package router_test

// A local demo backend for looking at the panel in a browser: the real API on
// a throwaway database with a simulated node. Not part of the test suite.
//
//   ATLAS_DEV_SERVER=1 go test ./internal/router/ -run TestDevServer -timeout 0

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/websocket"
	"github.com/luketaylor45/atlas/core/internal/config"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
	"github.com/luketaylor45/atlas/core/internal/router"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func demoNode(token string) *httptest.Server {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	files := map[string]string{
		"server.properties": "motd=An Atlas server\nmax-players=20\nserver-port=25565\n",
		"config.json":       "{\n  \"name\": \"demo\",\n  \"enabled\": true\n}\n",
		"eula.txt":          "eula=true\n",
		"start.sh":          "#!/bin/bash\njava -jar server.jar\n",
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Node-Token") != token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		path := r.URL.Path
		reply := func(value interface{}) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(value)
		}

		switch {
		case strings.HasSuffix(path, "/console"):
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			stamp := func() string { return time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z") }
			lines := []string{
				"--- Atlas Instance Wrapper ---",
				"Starting Server...",
				"\x1b[32m[Server thread/INFO]\x1b[0m: Starting minecraft server version 26.3",
				"\x1b[32m[Server thread/INFO]\x1b[0m: Preparing level \"world\"",
				"\x1b[33m[Server thread/WARN]\x1b[0m: Ambiguity between arguments [teleport, location] and [teleport, destination]",
				"\x1b[32m[Server thread/INFO]\x1b[0m: Done (4.812s)! For help, type \"help\"",
			}
			for _, line := range lines {
				if conn.WriteMessage(websocket.TextMessage, []byte(stamp()+" "+line)) != nil {
					return
				}
			}
			for i := 0; ; i++ {
				time.Sleep(4 * time.Second)
				line := fmt.Sprintf("\x1b[32m[Server thread/INFO]\x1b[0m: Saved the game (tick %d)", i)
				if i%5 == 4 {
					line = "\x1b[31m[Server thread/ERROR]\x1b[0m: Player lost connection: Timed out"
				}
				if conn.WriteMessage(websocket.TextMessage, []byte(stamp()+" "+line)) != nil {
					return
				}
			}
		case strings.HasSuffix(path, "/stats"):
			reply(map[string]interface{}{"cpu": fmt.Sprintf("%.1f", 20+float64(time.Now().Second()%30)), "memory": 1380 + time.Now().Second()*4, "network": map[string]int{"rx": 48211023, "tx": 9120331}})
		case strings.HasSuffix(path, "/install/log"):
			content := "[Atlas] Pulling game server image ghcr.io/ptero-eggs/yolks:java_25\n[Atlas] Pulling installer image ghcr.io/pterodactyl/installers:alpine\n[Atlas] Running install script with ash\n[Atlas] Looking up Paper versions\n[Atlas] Installing Paper 26.3 (build latest)\n[Atlas] Downloading https://fill-data.papermc.io/v1/objects/3e75/paper-26.3-251.jar\n  % Total    % Received % Xferd  Average Speed\n 42 52.7M   42 22.1M    0     0  11.2M      0  0:00:04\n"
			if strings.Contains(path, "dddddddd") {
				content += "curl: (22) The requested URL returned error: 404\n[Atlas] Error: the download failed.\n[Atlas] Installation failed: installer failed with exit code 1\n"
			}
			offset := 0
			fmt.Sscanf(r.URL.Query().Get("offset"), "%d", &offset)
			if offset > len(content) {
				offset = 0
			}
			reply(map[string]interface{}{"content": content[offset:], "offset": len(content), "running": !strings.Contains(path, "dddddddd")})
		case strings.HasSuffix(path, "/files/list"):
			if sub := strings.Trim(r.URL.Query().Get("path"), "/"); sub != "" {
				reply([]map[string]interface{}{{"name": "level.dat", "size": 2048, "is_dir": false, "mime": "application/octet-stream"}})
				return
			}
			list := []map[string]interface{}{
				{"name": "world", "size": 0, "is_dir": true, "mime": ""},
				{"name": "plugins", "size": 0, "is_dir": true, "mime": ""},
				{"name": "server.jar", "size": 55280056, "is_dir": false, "mime": "application/java-archive"},
				{"name": "backup.zip", "size": 10485760, "is_dir": false, "mime": "application/zip"},
			}
			for name, body := range files {
				list = append(list, map[string]interface{}{"name": name, "size": len(body), "is_dir": false, "mime": "text/plain"})
			}
			reply(list)
		case strings.HasSuffix(path, "/files/content"):
			name := strings.Trim(r.URL.Query().Get("path"), "/")
			body, ok := files[name]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				reply(map[string]string{"error": "File not found"})
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte(body))
		case strings.HasSuffix(path, "/files/write"):
			var body struct {
				Content string `json:"content"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			files[strings.Trim(r.URL.Query().Get("path"), "/")] = body.Content
			reply(map[string]string{"status": "success"})
		case strings.HasSuffix(path, "/command"):
			time.Sleep(150 * time.Millisecond)
			reply(map[string]string{"status": "sent"})
		case path == "/api/servers" && r.Method == http.MethodGet:
			reply(map[string]interface{}{"servers": map[string]string{}, "installing": []string{}})
		default:
			time.Sleep(300 * time.Millisecond)
			reply(map[string]string{"status": "ok"})
		}
	}))
}

func TestDevServer(t *testing.T) {
	if os.Getenv("ATLAS_DEV_SERVER") == "" {
		t.Skip("set ATLAS_DEV_SERVER=1 to run the local demo backend")
	}
	gin.SetMode(gin.ReleaseMode)

	dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "demo.db")) + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	must(t, err)
	database.DB = db
	must(t, database.Migrate())
	config.AppConfig.JWTSecret = "demo-secret-demo-secret-demo-secret-000"
	config.AppConfig.RegistrationEnabled = os.Getenv("ATLAS_DEV_REGISTRATION") != ""

	const token = "n_demo-node-token"
	node := demoNode(token)
	defer node.Close()
	nodeURL, _ := url.Parse(node.URL)

	hash, _ := bcrypt.GenerateFromPassword([]byte("atlas-demo-pass"), bcrypt.MinCost)
	admin := models.User{Username: "admin", Password: string(hash), IsAdmin: true}
	owner := models.User{Username: "luke", Password: string(hash)}
	friend := models.User{Username: "matt", Password: string(hash)}
	for _, user := range []*models.User{&admin, &owner, &friend} {
		must(t, db.Create(user).Error)
	}

	primary := models.Node{Name: "Node-01", Address: "play.example.com", InternalAddress: nodeURL.Hostname(), Port: nodeURL.Port(), SFTPPort: "2022", Token: token, TotalRAM: 32768, TotalDisk: 512000, UsedRAM: 21300, UsedCPU: 37.5, Location: "London, UK", IsOnline: true, LastHeartbeat: time.Now()}
	spare := models.Node{Name: "Node-02", Address: "eu2.example.com", Port: "8081", SFTPPort: "2022", Token: "n_other", TotalRAM: 16384, TotalDisk: 256000, UsedRAM: 15900, UsedCPU: 93, Location: "Frankfurt, DE", IsOnline: false, LastHeartbeat: time.Now().Add(-3 * time.Hour)}
	must(t, db.Create(&primary).Error)
	must(t, db.Create(&spare).Error)

	games := models.Nest{UUID: "n-games", Name: "Games", Code: "games", Description: "Game servers"}
	must(t, db.Create(&games).Error)
	minecraft := models.Nest{UUID: "n-mc", Name: "Minecraft", Code: "minecraft", Description: "Minecraft servers", ParentID: &games.ID}
	gmod := models.Nest{UUID: "n-gmod", Name: "Garry's Mod", Code: "gmod", Description: "Source engine sandbox", ParentID: &games.ID}
	empty := models.Nest{UUID: "n-empty", Name: "Voice", Code: "voice", Description: "Voice servers"}
	for _, nest := range []*models.Nest{&minecraft, &gmod, &empty} {
		must(t, db.Create(nest).Error)
	}

	paper := models.Egg{UUID: "atlas-mc-paper", NestID: minecraft.ID, Available: true, Author: "Atlas", Name: "Paper", Description: "High performance Minecraft server.", DockerImages: `["ghcr.io/ptero-eggs/yolks:java_25","ghcr.io/ptero-eggs/yolks:java_21","ghcr.io/ptero-eggs/yolks:java_17"]`, StartupCommand: "java -Xms128M -Xmx{{SERVER_MEMORY}}M -jar {{SERVER_JARFILE}}", StopCommand: "stop", ScriptInstall: "#!/bin/ash\nset -eu\necho installing\n", ScriptContainer: "ghcr.io/pterodactyl/installers:alpine", ScriptEntry: "ash", Source: "catalog"}
	palworld := models.Egg{UUID: "egg-palworld", NestID: games.ID, Available: true, Author: "Imported", Name: "Palworld", Description: "Survival and creature collecting.", DockerImages: `["ghcr.io/ptero-eggs/steamcmd:debian"]`, StartupCommand: "./PalServer.sh -port={{SERVER_PORT}}", StopCommand: "^C", ScriptInstall: "#!/bin/bash\necho hi\n", ScriptContainer: "ghcr.io/pterodactyl/installers:debian", ScriptEntry: "bash", Ports: `[{"name":"Query","env":"QUERY_PORT","default":27015,"protocol":"udp"},{"name":"RCON","env":"RCON_PORT","default":25575,"protocol":"tcp"}]`, Source: "import", Customized: true}
	sandbox := models.Egg{UUID: "atlas-gmod-x64", NestID: gmod.ID, Available: true, Author: "Atlas", Name: "Modern (x86-64)", Description: "64-bit Garry's Mod branch.", DockerImages: `["ghcr.io/ptero-eggs/games:source"]`, StartupCommand: "./srcds_run_x64 -game garrysmod -port {{SERVER_PORT}} +map {{SRCDS_MAP}}", StopCommand: "quit", ScriptContainer: "ghcr.io/pterodactyl/installers:debian", ScriptEntry: "bash", Source: "catalog"}
	for _, egg := range []*models.Egg{&paper, &palworld, &sandbox} {
		must(t, db.Create(egg).Error)
	}
	variable := func(egg models.Egg, name, env, value, description string, editable, viewable bool, input string) {
		row := models.EggVariable{EggID: egg.ID, Name: name, EnvironmentVariable: env, DefaultValue: value, Description: description, InputType: input}
		must(t, db.Create(&row).Error)
		must(t, db.Model(&row).Updates(map[string]interface{}{"user_editable": editable, "user_viewable": viewable}).Error)
	}
	variable(paper, "Minecraft version", "MINECRAFT_VERSION", "latest", "Version to install, or 'latest'.", true, true, "text")
	variable(paper, "Server jar file", "SERVER_JARFILE", "server.jar", "The jar the server starts.", false, true, "text")
	variable(paper, "Download URL", "DL_PATH", "", "Optional direct download link.", true, true, "text")
	variable(paper, "Build number", "BUILD_NUMBER", "latest", "", false, false, "text")
	variable(sandbox, "Map", "SRCDS_MAP", "gm_flatgrass", "The map loaded at start.", true, true, "text")
	variable(sandbox, "Steam token", "STEAM_TOKEN", "", "Game server login token.", true, true, "password")
	variable(palworld, "Server name", "SERVER_NAME", "An Atlas hosted Palworld Server", "", true, true, "text")
	variable(palworld, "Admin password", "ADMIN_PASSWORD", "", "", true, true, "password")

	service := func(uuid, name string, owner models.User, node models.Node, egg models.Egg, port int, status, stage string, progress int, image string) models.Service {
		row := models.Service{UUID: uuid, Name: name, UserID: owner.ID, NodeID: node.ID, EggID: egg.ID, Memory: 8192, Disk: 40000, Cpu: 400, Port: port, DockerImage: image, Status: status, InstallationStage: stage, InstallationProgress: progress, StartupCommand: egg.StartupCommand, Environment: "{}"}
		must(t, db.Create(&row).Error)
		must(t, db.Create(&models.ServicePort{ServiceID: row.ID, NodeID: node.ID, Port: port, Env: models.PrimaryPortEnv, Name: "Game", Protocol: "both", IsPrimary: true}).Error)
		return row
	}
	survival := service("aaaaaaaa-1111-4111-8111-111111111111", "Survival SMP", owner, primary, paper, 25565, "running", "Installation complete", 100, "ghcr.io/ptero-eggs/yolks:java_25")
	service("bbbbbbbb-2222-4222-8222-222222222222", "City-Life DarkRP", owner, primary, sandbox, 27015, "offline", "Installation complete", 100, "ghcr.io/ptero-eggs/games:source")
	service("cccccccc-3333-4333-8333-333333333333", "Creative Build", owner, primary, paper, 25566, "installing", "Downloading files: 42%", 46, "ghcr.io/ptero-eggs/yolks:java_25")
	failed := service("dddddddd-4444-4444-8444-444444444444", "Modded Test With A Rather Long Server Name", admin, primary, paper, 25567, "installation_failed", "Installation failed: installer failed with exit code 1; recent output: curl: (22) The requested URL returned error: 404", 30, "ghcr.io/ptero-eggs/yolks:java_21")
	pal := service("eeeeeeee-5555-4555-8555-555555555555", "Palworld Friends", friend, primary, palworld, 8211, "starting", "Installation complete", 100, "ghcr.io/ptero-eggs/steamcmd:debian")
	must(t, db.Create(&models.ServicePort{ServiceID: pal.ID, NodeID: primary.ID, Port: 27016, Env: "QUERY_PORT", Name: "Query", Protocol: "udp"}).Error)
	must(t, db.Create(&models.ServicePort{ServiceID: pal.ID, NodeID: primary.ID, Port: 25575, Env: "RCON_PORT", Name: "RCON", Protocol: "tcp"}).Error)
	_ = failed

	// matt can see Survival SMP's console and files only.
	grant := models.ServiceUser{ServiceID: survival.ID, UserID: friend.ID, CanManageFiles: true}
	must(t, db.Create(&grant).Error)

	for i, entry := range []struct{ action, resource, description string }{
		{"power", "start", "Server start"}, {"sftp_login", "SFTP", "SFTP connection established (Owner)"},
		{"file_extract", "backup.zip", "Extracted ZIP archive"}, {"power", "restart", "Server restart"},
		{"user_added", "", "Sub-user added to service"}, {"reinstall", "service", "Server reinstall started"},
	} {
		must(t, db.Create(&models.ActivityLog{ServiceID: &survival.ID, UserID: &owner.ID, Action: entry.action, Resource: entry.resource, Description: entry.description, IPAddress: "203.0.113.24", UserAgent: "Mozilla/5.0", Metadata: `{"node":"Node-01"}`, CreatedAt: time.Now().Add(-time.Duration(i*47) * time.Minute)}).Error)
	}
	must(t, db.Create(&models.ActivityLog{UserID: &admin.ID, Action: "create", Resource: "node", Description: "Registered new node: Node-02", IPAddress: "203.0.113.9", CreatedAt: time.Now().Add(-26 * time.Hour)}).Error)

	must(t, db.Create(&models.News{Title: "Atlas 2.0 is here", Content: "A redesigned panel with light and dark themes, a real console, and more reliable installs.", Type: "Release"}).Error)
	must(t, db.Create(&models.News{Title: "Scheduled maintenance on Sunday", Content: "Node-02 will be restarted for a kernel update between 03:00 and 03:30 UTC. Servers on that node will be offline for a few minutes.", Type: "Maintenance"}).Error)

	engine := gin.New()
	router.Setup(engine)
	fmt.Println("Atlas demo backend on http://localhost:8080 — users admin / luke / matt, password atlas-demo-pass")
	must(t, engine.Run("127.0.0.1:8080"))
}
