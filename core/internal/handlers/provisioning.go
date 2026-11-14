package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
	"gorm.io/gorm"
)

// eggPorts decodes the additional ports an egg declares.
func eggPorts(egg *models.Egg) []models.EggPort {
	if strings.TrimSpace(egg.Ports) == "" {
		return nil
	}
	var ports []models.EggPort
	if err := json.Unmarshal([]byte(egg.Ports), &ports); err != nil {
		return nil
	}
	valid := ports[:0]
	for _, port := range ports {
		port.Env = strings.TrimSpace(port.Env)
		if port.Env == "" || port.Env == models.PrimaryPortEnv {
			continue
		}
		port.Protocol = normalizeProtocol(port.Protocol)
		valid = append(valid, port)
	}
	return valid
}

func normalizeProtocol(protocol string) string {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "tcp":
		return "tcp"
	case "udp":
		return "udp"
	default:
		return "both"
	}
}

func eggImages(egg *models.Egg) []string {
	var images []string
	_ = json.Unmarshal([]byte(egg.DockerImages), &images)
	return images
}

// serviceImage is the image a service runs: its own choice, else the egg's first.
func serviceImage(service *models.Service) string {
	if image := strings.TrimSpace(service.DockerImage); image != "" {
		return image
	}
	if images := eggImages(&service.Egg); len(images) > 0 {
		return images[0]
	}
	return ""
}

// mergedEnvironment is what the game and its installer see: egg defaults,
// then the service's own values, then the allocated ports, which always win
// so a variable can never point a game at a port it does not hold.
func mergedEnvironment(service *models.Service) map[string]string {
	merged := make(map[string]string)
	for _, variable := range service.Egg.Variables {
		merged[variable.EnvironmentVariable] = variable.DefaultValue
	}
	if strings.TrimSpace(service.Environment) != "" {
		overrides := make(map[string]string)
		if err := json.Unmarshal([]byte(service.Environment), &overrides); err == nil {
			for key, value := range overrides {
				merged[key] = value
			}
		}
	}
	for _, port := range service.Ports {
		if !port.IsPrimary && port.Env != "" {
			merged[port.Env] = strconv.Itoa(port.Port)
		}
	}
	return merged
}

// buildServerSpec is the single description of a service sent to its node
// for create, reinstall and rebuild, so the three can never disagree.
func buildServerSpec(service *models.Service) map[string]interface{} {
	environment, _ := json.Marshal(mergedEnvironment(service))

	extra := make([]map[string]interface{}, 0, len(service.Ports))
	for _, port := range service.Ports {
		if port.IsPrimary {
			continue
		}
		extra = append(extra, map[string]interface{}{"port": port.Port, "protocol": normalizeProtocol(port.Protocol)})
	}

	return map[string]interface{}{
		"uuid":              service.UUID,
		"memory":            service.Memory,
		"disk":              service.Disk,
		"cpu":               service.Cpu,
		"port":              service.Port,
		"ports":             extra,
		"egg_image":         serviceImage(service),
		"startup_command":   resolveStartupCommand(service),
		"environment":       string(environment),
		"install_script":    service.Egg.ScriptInstall,
		"install_container": service.Egg.ScriptContainer,
		"install_entry":     service.Egg.ScriptEntry,
	}
}

// loadServiceForNode loads everything buildServerSpec needs.
func loadServiceForNode(id uint) (*models.Service, error) {
	var service models.Service
	err := database.DB.Preload("Node").Preload("Egg.Variables").Preload("Ports").First(&service, id).Error
	if err != nil {
		return nil, err
	}
	return &service, nil
}

// daemonJSON sends a JSON request to a node and returns its response. The
// caller closes the body.
func daemonJSON(client *http.Client, node *models.Node, method string, path string, payload interface{}) (*http.Response, error) {
	var body *bytes.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode node request: %w", err)
		}
		body = bytes.NewReader(encoded)
	} else {
		body = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, nodeURL(node, path), body)
	if err != nil {
		return nil, fmt.Errorf("prepare node request: %w", err)
	}
	req.Header.Set("X-Node-Token", node.Token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return client.Do(req)
}

// daemonDo sends a request and turns a node-side failure into an error
// carrying the node's own message.
func daemonDo(client *http.Client, node *models.Node, method string, path string, payload interface{}) error {
	resp, err := daemonJSON(client, node, method, path, payload)
	if err != nil {
		return fmt.Errorf("node unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return &nodeError{
			Status:  resp.StatusCode,
			Message: nodeErrorMessage(resp.Body, fmt.Sprintf("node returned HTTP %d", resp.StatusCode)),
		}
	}
	return nil
}

// nodeError is a failure reported by a node, as opposed to failing to reach it.
type nodeError struct {
	Status  int
	Message string
}

func (e *nodeError) Error() string { return e.Message }

// nodeErrorStatus picks the HTTP status Core should answer with.
func nodeErrorStatus(err error) int {
	var reported *nodeError
	if errors.As(err, &reported) {
		return nodeStatus(reported.Status)
	}
	return http.StatusBadGateway
}

var errPortTaken = errors.New("port already allocated")

// portTakenError names the port that could not be allocated.
type portTakenError struct{ Port int }

func (e *portTakenError) Error() string {
	return fmt.Sprintf("port %d is already assigned to another service on this node", e.Port)
}

func (e *portTakenError) Unwrap() error { return errPortTaken }

// usedNodePorts returns every port allocated on a node, optionally ignoring
// one service's own allocations.
func usedNodePorts(tx *gorm.DB, nodeID uint, excludeServiceID uint) (map[int]bool, error) {
	query := tx.Model(&models.ServicePort{}).Where("node_id = ?", nodeID)
	if excludeServiceID > 0 {
		query = query.Where("service_id != ?", excludeServiceID)
	}
	var ports []int
	if err := query.Pluck("port", &ports).Error; err != nil {
		return nil, err
	}
	used := make(map[int]bool, len(ports))
	for _, port := range ports {
		used[port] = true
	}
	return used, nil
}

// planServicePorts decides the full port set for a service: the primary port
// plus one per port the egg declares. A requested port must be free. A port
// that was not requested takes the egg's default, or the next free port above
// it, so creating a second server of the same game just works.
func planServicePorts(used map[int]bool, primary int, declared []models.EggPort, requested map[string]int) ([]models.ServicePort, error) {
	if err := validateServicePort(primary); err != nil {
		return nil, err
	}
	if used[primary] {
		return nil, &portTakenError{Port: primary}
	}

	taken := map[int]bool{primary: true}
	plan := []models.ServicePort{{
		Port:      primary,
		Env:       models.PrimaryPortEnv,
		Name:      "Game",
		Protocol:  "both",
		IsPrimary: true,
	}}

	known := make(map[string]bool, len(declared))
	for _, port := range declared {
		known[port.Env] = true
	}
	unknown := make([]string, 0)
	for env := range requested {
		if !known[env] {
			unknown = append(unknown, env)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("this server type has no port named %s", strings.Join(unknown, ", "))
	}

	for _, port := range declared {
		chosen, explicit := requested[port.Env]
		if explicit && chosen != 0 {
			if err := validateServicePort(chosen); err != nil {
				return nil, fmt.Errorf("%s: %w", port.Name, err)
			}
			if used[chosen] || taken[chosen] {
				return nil, &portTakenError{Port: chosen}
			}
		} else {
			chosen = port.Default
			if chosen < 1 || chosen > 65535 {
				chosen = primary + 1
			}
			for chosen <= 65535 && (used[chosen] || taken[chosen]) {
				chosen++
			}
			if chosen > 65535 {
				return nil, fmt.Errorf("no free port is available for %s", port.Name)
			}
		}

		taken[chosen] = true
		plan = append(plan, models.ServicePort{
			Port:     chosen,
			Env:      port.Env,
			Name:     port.Name,
			Protocol: normalizeProtocol(port.Protocol),
		})
	}
	return plan, nil
}

// isUniqueViolation reports a lost race for a port: two requests planned the
// same free port and the database let only one keep it.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	message := err.Error()
	return strings.Contains(message, "23505") || strings.Contains(message, "duplicate key value") ||
		strings.Contains(message, "UNIQUE constraint failed")
}
