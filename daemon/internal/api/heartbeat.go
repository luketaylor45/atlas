package api

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/docker/docker/api/types/events"
	"github.com/luketaylor45/atlas/daemon/internal/config"
	"github.com/luketaylor45/atlas/daemon/internal/docker"
)

type HeartbeatPayload struct {
	Token string `json:"token"`
	Stats struct {
		CPU float64 `json:"cpu"`
		RAM float64 `json:"ram"`
	} `json:"stats"`
}

func StartHeartbeat() {
	go MonitorEvents()

	if err := sendHeartbeat(); err != nil {
		log.Printf("Failed to send initial heartbeat: %v", err)
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if err := sendHeartbeat(); err != nil {
			log.Printf("Failed to send heartbeat: %v", err)
		}
	}
}

func MonitorEvents() {
	for {
		if err := monitorEvents(context.Background()); err != nil {
			log.Printf("[Daemon] Docker event monitor disconnected: %v; reconnecting in 5s", err)
		}
		time.Sleep(5 * time.Second)
	}
}

func monitorEvents(ctx context.Context) error {
	msgs, errs := docker.Client.Events(ctx, events.ListOptions{})
	log.Println("[Daemon] Monitoring Docker events...")

	for {
		select {
		case msg, ok := <-msgs:
			if !ok {
				return fmt.Errorf("event stream closed")
			}
			// We only care about container events
			if msg.Type != events.ContainerEventType {
				continue
			}

			uuid := msg.Actor.Attributes["name"]
			// Docker container names often have a leading slash
			uuid = strings.TrimPrefix(uuid, "/")

			if uuid == "" {
				uuid = msg.Actor.ID
			}

			// Ignore installation containers in the main monitor
			if strings.HasPrefix(uuid, "install-") {
				continue
			}

			status := ""

			switch msg.Action {
			case "start":
				status = "running"
			case "die", "stop":
				status = "offline"
			}

			if status != "" {
				log.Printf("[Daemon] Event detected for %s: %s -> notifying Core", uuid, msg.Action)
				NotifyStatus(uuid, status)
			}
		case err, ok := <-errs:
			if !ok {
				return fmt.Errorf("event error stream closed")
			}
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func sendHeartbeat() error {
	payload := HeartbeatPayload{
		Token: config.NodeConfig.NodeToken,
	}
	// Mock stats for now
	payload.Stats.CPU = 10.5
	payload.Stats.RAM = 512.0

	return postCoreJSON("/api/v1/internal/heartbeat", payload)
}

func NotifyStatus(uuid string, status string) {
	notifyServiceStatus(uuid, status, "", nil)
}

func NotifyInstallationProgress(uuid string, stage string, progress int) {
	progress = min(max(progress, 0), 100)
	notifyServiceStatus(uuid, "installing", stage, &progress)
}

func NotifyInstallationFailure(uuid string, err error) {
	stage := "Installation failed"
	if err != nil {
		stage += ": " + err.Error()
	}
	if len(stage) > 240 {
		stage = stage[:237] + "..."
	}
	notifyServiceStatus(uuid, "installation_failed", stage, nil)
}

func notifyServiceStatus(uuid string, status string, stage string, progress *int) {
	payload := map[string]any{
		"token":  config.NodeConfig.NodeToken,
		"status": status,
	}
	if stage != "" {
		payload["stage"] = stage
	}
	if progress != nil {
		payload["progress"] = *progress
	}

	if err := postCoreJSON("/api/v1/internal/services/"+uuid+"/status", payload); err != nil {
		log.Printf("Failed to notify status for %s: %v", uuid, err)
	}
}
