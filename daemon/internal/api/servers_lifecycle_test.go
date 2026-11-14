package api

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/errdefs"
	"github.com/luketaylor45/atlas/daemon/internal/config"
)

func TestConsoleUnavailableMessageIncludesInstallerFailure(t *testing.T) {
	previous := config.NodeConfig
	config.NodeConfig.DataPath = t.TempDir()
	t.Cleanup(func() { config.NodeConfig = previous })

	uuid := "service-uuid"
	serviceDir := filepath.Join(config.NodeConfig.DataPath, uuid)
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(serviceDir, ".atlas_install_failed"), []byte("steamcmd exited with code 8"), 0644); err != nil {
		t.Fatal(err)
	}

	message := consoleUnavailableMessage(uuid, errdefs.NotFound(errors.New("missing")))
	if !strings.Contains(message, "steamcmd exited with code 8") {
		t.Fatalf("console message did not include installer failure: %q", message)
	}
}

func TestConsoleUnavailableMessageDistinguishesProvisioning(t *testing.T) {
	previous := config.NodeConfig
	config.NodeConfig.DataPath = t.TempDir()
	t.Cleanup(func() { config.NodeConfig = previous })

	message := consoleUnavailableMessage("service-uuid", errdefs.NotFound(errors.New("missing")))
	if !strings.Contains(message, "not been created yet") {
		t.Fatalf("unexpected provisioning message: %q", message)
	}
}

func TestPowerActionErrorMessageExplainsCommonStartFailures(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want []string
	}{
		{
			name: "port collision",
			err:  errors.New("Bind for 0.0.0.0:27015 failed: port is already allocated"),
			want: []string{"host port 27015 is already in use", "different port", "port is already allocated"},
		},
		{
			name: "missing bind source",
			err:  errors.New("invalid mount config: bind source path does not exist: /srv/atlas/server"),
			want: []string{"Check DATA_PATH/HOST_DATA_PATH", "bind source path does not exist"},
		},
		{
			name: "permission denied",
			err:  errors.New("failed to mount local volume: permission denied"),
			want: []string{"Check DATA_PATH and SERVER_UID/SERVER_GID", "permission denied"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			message := powerActionErrorMessage("start", 27015, test.err)
			for _, expected := range test.want {
				if !strings.Contains(message, expected) {
					t.Fatalf("message %q does not contain %q", message, expected)
				}
			}
		})
	}
}

func TestContainerHasNetwork(t *testing.T) {
	tests := []struct {
		name     string
		networks map[string]*network.EndpointSettings
		running  bool
		want     bool
	}{
		{name: "missing network map", networks: nil, want: false},
		{name: "none network", networks: map[string]*network.EndpointSettings{"none": {}}, want: false},
		{name: "nil bridge endpoint", networks: map[string]*network.EndpointSettings{"bridge": nil}, want: false},
		{
			// What Docker reports for a container that was created by an
			// install and has never been started.
			name:     "created container with configured bridge",
			networks: map[string]*network.EndpointSettings{"bridge": {}},
			running:  false,
			want:     true,
		},
		{
			name:     "running container without a bridge endpoint needs repair",
			networks: map[string]*network.EndpointSettings{"bridge": {}},
			running:  true,
			want:     false,
		},
		{
			name:     "running container on the bridge",
			networks: map[string]*network.EndpointSettings{"bridge": {NetworkID: "bridge-network-id"}},
			running:  true,
			want:     true,
		},
		{
			name:     "stopped container keeps its bridge",
			networks: map[string]*network.EndpointSettings{"bridge": {NetworkID: "bridge-network-id"}},
			running:  false,
			want:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := containerHasNetwork(test.networks, defaultServerNetwork, test.running); got != test.want {
				t.Fatalf("containerHasNetwork() = %t, want %t", got, test.want)
			}
		})
	}
}
