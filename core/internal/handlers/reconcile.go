package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
)

const (
	reconcileInterval = 30 * time.Second
	// nodeOfflineAfter is how long a node may go without a heartbeat.
	nodeOfflineAfter = 30 * time.Second
	// statusGrace protects a status that was just written (a request to the
	// node may still be in flight) from being "corrected" too early.
	statusGrace = 60 * time.Second

	interruptedInstallStage = "Installation was interrupted before it finished. Retry the installation."
)

// nodeState is what a node reports about itself.
type nodeState struct {
	// Servers maps a service UUID to "running", "starting" or "stopped" for
	// every game container that exists.
	Servers map[string]string `json:"servers"`
	// Installing lists the services with an install job in progress.
	Installing []string `json:"installing"`
	// Backups lists the backups with a create or restore job in progress.
	Backups []string `json:"backups"`
}

// statusCorrection is the change the reconciler wants to make to a service.
type statusCorrection struct {
	Status string
	Stage  string
}

// reconcileStatus compares what Core believes with what the node reports and
// returns the correction, if any. Core only ever hears about status changes
// through callbacks, so a lost callback, a restarted daemon or a crashed
// installer would otherwise leave a service showing the wrong state forever.
func reconcileStatus(current string, sinceUpdate time.Duration, container string, installing bool) *statusCorrection {
	if installing {
		// The node is the authority on running installs.
		if current != StatusInstalling {
			return &statusCorrection{Status: StatusInstalling}
		}
		return nil
	}
	if sinceUpdate < statusGrace {
		return nil
	}

	actual := StatusOffline
	switch container {
	case "running":
		actual = StatusRunning
	case "starting":
		actual = StatusStarting
	}

	switch current {
	case StatusInstalling:
		// No job is running. A container means the install finished and the
		// final callback was lost; no container means it died part-way.
		if container == "" {
			return &statusCorrection{Status: StatusInstallationFailed, Stage: interruptedInstallStage}
		}
		return &statusCorrection{Status: actual, Stage: "Installation complete"}
	case StatusInstallationFailed:
		// A failed install stays failed until someone retries it.
		return nil
	default:
		if current != actual {
			return &statusCorrection{Status: actual}
		}
		return nil
	}
}

// StartReconciler runs the periodic node and service status check.
func StartReconciler(ctx context.Context) {
	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reconcileOnce(time.Now())
		}
	}
}

func reconcileOnce(now time.Time) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("[Core] Status reconciler recovered from: %v", recovered)
		}
	}()

	// Nodes that stopped sending heartbeats are offline, wherever that is read.
	if err := database.DB.Model(&models.Node{}).
		Where("is_online = ? AND (last_heartbeat IS NULL OR last_heartbeat < ?)", true, now.Add(-nodeOfflineAfter)).
		Update("is_online", false).Error; err != nil {
		log.Printf("[Core] Failed to expire offline nodes: %v", err)
	}

	var nodes []models.Node
	if err := database.DB.Where("is_online = ?", true).Find(&nodes).Error; err != nil {
		log.Printf("[Core] Reconciler could not list nodes: %v", err)
		return
	}

	for i := range nodes {
		reconcileNode(&nodes[i], now)
	}
}

func fetchNodeState(node *models.Node) (*nodeState, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := daemonJSON(client, node, http.MethodGet, "/api/servers", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &nodeError{Status: resp.StatusCode, Message: nodeErrorMessage(resp.Body, "node state unavailable")}
	}

	var state nodeState
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return nil, err
	}
	return &state, nil
}

func reconcileNode(node *models.Node, now time.Time) {
	var services []models.Service
	if err := database.DB.Select("id", "uuid", "status", "updated_at").Where("node_id = ?", node.ID).Find(&services).Error; err != nil || len(services) == 0 {
		return
	}

	state, err := fetchNodeState(node)
	if err != nil {
		// An unreachable node says nothing about its servers; leave them be.
		return
	}
	reconcileBackups(node, state.Backups, now)

	installing := make(map[string]bool, len(state.Installing))
	for _, uuid := range state.Installing {
		installing[uuid] = true
	}

	for _, service := range services {
		correction := reconcileStatus(service.Status, now.Sub(service.UpdatedAt), state.Servers[service.UUID], installing[service.UUID])
		if correction == nil {
			continue
		}

		updates := map[string]interface{}{"status": correction.Status}
		if correction.Stage != "" {
			updates["installation_stage"] = correction.Stage
		}
		// The status is compared again in the query so a callback that
		// arrived in the meantime is never overwritten.
		result := database.DB.Model(&models.Service{}).
			Where("id = ? AND status = ? AND updated_at = ?", service.ID, service.Status, service.UpdatedAt).
			Updates(updates)
		if result.Error == nil && result.RowsAffected > 0 {
			log.Printf("[Core] Corrected status of %s on %s: %s -> %s", service.UUID, node.Name, service.Status, correction.Status)
		}
	}
}
