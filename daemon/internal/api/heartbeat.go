package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
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
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for range ticker.C {
			sendHeartbeat()
		}
	}()

	// Also start event monitor
	go MonitorEvents()
}

func MonitorEvents() {
	ctx := context.Background()
	msgs, errs := docker.Client.Events(ctx, events.ListOptions{})

	log.Println("[Daemon] Monitoring Docker events...")

	for {
		select {
		case msg := <-msgs:
			// We only care about container events
			if msg.Type != events.ContainerEventType {
				continue
			}

			uuid := msg.Actor.Attributes["name"]
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
		case err := <-errs:
			if err != nil {
				log.Printf("[Daemon] Event monitor error: %v", err)
				return
			}
		}
	}
}

func sendHeartbeat() {
	payload := HeartbeatPayload{
		Token: config.NodeConfig.NodeToken,
	}
	// Mock stats for now
	payload.Stats.CPU = 10.5
	payload.Stats.RAM = 512.0

	data, _ := json.Marshal(payload)
	_, err := http.Post(config.NodeConfig.CoreURL+"/api/v1/internal/heartbeat", "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("Failed to send heartbeat: %v", err)
	} else {
		// log.Println("Heartbeat sent") // Too spammy
	}
}

func NotifyStatus(uuid string, status string) {
	payload := map[string]string{
		"token":  config.NodeConfig.NodeToken,
		"status": status,
	}
	data, _ := json.Marshal(payload)
	url := config.NodeConfig.CoreURL + "/api/v1/internal/services/" + uuid + "/status"

	_, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("Failed to notify status for %s: %v", uuid, err)
	}
}
