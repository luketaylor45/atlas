package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/luketaylor45/atlas/daemon/internal/config"
)

func natPort(value string) nat.Port { return nat.Port(value) }

func TestInstallRegistryAllowsOneJobPerService(t *testing.T) {
	registry := &installRegistry{jobs: make(map[string]*installJob)}

	_, finish, ok := registry.begin("a", time.Minute)
	if !ok {
		t.Fatal("first job was refused")
	}
	if _, _, ok := registry.begin("a", time.Minute); ok {
		t.Fatal("a second job started for the same service")
	}
	if _, finishOther, ok := registry.begin("b", time.Minute); !ok {
		t.Fatal("a job for another service was refused")
	} else {
		finishOther()
	}
	if got := registry.list(); len(got) != 1 || got[0] != "a" {
		t.Fatalf("list = %v, want [a]", got)
	}

	finish()
	if registry.active("a") {
		t.Fatal("job is still active after finishing")
	}
	if _, finishAgain, ok := registry.begin("a", time.Minute); !ok {
		t.Fatal("service could not be installed again after finishing")
	} else {
		finishAgain()
	}
}

func TestInstallRegistryCancelReportsItsCause(t *testing.T) {
	registry := &installRegistry{jobs: make(map[string]*installJob)}

	ctx, finish, _ := registry.begin("a", time.Minute)
	defer finish()

	if registry.cancel("missing") {
		t.Fatal("cancel reported success for a service with no job")
	}
	if !registry.cancel("a") {
		t.Fatal("cancel did not find the running job")
	}
	<-ctx.Done()
	if !errors.Is(context.Cause(ctx), errInstallCancelled) {
		t.Fatalf("cause = %v, want cancellation", context.Cause(ctx))
	}
}

func TestInstallRegistryTimeoutReportsItsCause(t *testing.T) {
	registry := &installRegistry{jobs: make(map[string]*installJob)}

	ctx, finish, _ := registry.begin("a", 10*time.Millisecond)
	defer finish()

	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("job did not time out")
	}
	if !errors.Is(context.Cause(ctx), errInstallTimedOut) {
		t.Fatalf("cause = %v, want timeout", context.Cause(ctx))
	}
}

func TestInstallLogIsReadIncrementally(t *testing.T) {
	previous := config.NodeConfig
	config.NodeConfig.DataPath = t.TempDir()
	t.Cleanup(func() { config.NodeConfig = previous })

	transcript, err := openInstallLog(testServiceUUID)
	if err != nil {
		t.Fatal(err)
	}
	transcript.Printf("first line")

	content, offset, err := readInstallLog(testServiceUUID, 0)
	if err != nil || content != "[Atlas] first line\n" {
		t.Fatalf("content = %q, err = %v", content, err)
	}

	transcript.Printf("second line")
	transcript.Close()

	content, next, err := readInstallLog(testServiceUUID, offset)
	if err != nil || content != "[Atlas] second line\n" {
		t.Fatalf("incremental content = %q, err = %v", content, err)
	}
	if content, again, _ := readInstallLog(testServiceUUID, next); content != "" || again != next {
		t.Fatalf("reading at the end returned %q and moved the offset", content)
	}

	// A new install truncates the log; a stale offset must restart from the top.
	restarted, _ := openInstallLog(testServiceUUID)
	restarted.Printf("fresh")
	restarted.Close()
	if content, _, _ := readInstallLog(testServiceUUID, next); content != "[Atlas] fresh\n" {
		t.Fatalf("stale offset returned %q", content)
	}
}

func TestInstallLogIsStoredOutsideServiceDirectories(t *testing.T) {
	previous := config.NodeConfig
	config.NodeConfig.DataPath = t.TempDir()
	t.Cleanup(func() { config.NodeConfig = previous })

	path := installLogPath(testServiceUUID)
	serviceDir := filepath.Join(config.NodeConfig.DataPath, testServiceUUID)
	if strings.HasPrefix(path, serviceDir+string(os.PathSeparator)) {
		t.Fatalf("install log %s lives inside the service directory and would be wiped by a reinstall", path)
	}
}

func TestMissingInstallLogReadsAsEmpty(t *testing.T) {
	previous := config.NodeConfig
	config.NodeConfig.DataPath = t.TempDir()
	t.Cleanup(func() { config.NodeConfig = previous })

	content, offset, err := readInstallLog(testServiceUUID, 0)
	if err != nil || content != "" || offset != 0 {
		t.Fatalf("content = %q, offset = %d, err = %v", content, offset, err)
	}
}

func TestServerSpecPortBindings(t *testing.T) {
	spec := ServerSpec{
		UUID:     testServiceUUID,
		EggImage: "game:latest",
		Port:     8211,
		Ports: []PortMapping{
			{Port: 25575, Protocol: "tcp"},
			{Port: 27015, Protocol: "udp"},
			{Port: 27016},
		},
	}
	if err := spec.validate(); err != nil {
		t.Fatal(err)
	}

	bindings, err := spec.portBindings()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"8211/tcp", "8211/udp", "25575/tcp", "27015/udp", "27016/tcp", "27016/udp"}
	if len(bindings) != len(want) {
		t.Fatalf("got %d bindings, want %d: %v", len(bindings), len(want), bindings)
	}
	for _, key := range want {
		if _, ok := bindings[natPort(key)]; !ok {
			t.Fatalf("missing binding %s in %v", key, bindings)
		}
	}
	if _, ok := bindings[natPort("25575/udp")]; ok {
		t.Fatal("a tcp-only port was also published on udp")
	}
}

func TestServerSpecRejectsBadInput(t *testing.T) {
	valid := ServerSpec{UUID: testServiceUUID, EggImage: "game:latest", Port: 25565}

	tests := map[string]func(*ServerSpec){
		"path traversal uuid":  func(s *ServerSpec) { s.UUID = "../etc" },
		"missing image":        func(s *ServerSpec) { s.EggImage = " " },
		"port zero":            func(s *ServerSpec) { s.Port = 0 },
		"extra port too high":  func(s *ServerSpec) { s.Ports = []PortMapping{{Port: 70000}} },
		"unknown protocol":     func(s *ServerSpec) { s.Ports = []PortMapping{{Port: 1234, Protocol: "sctp"}} },
		"environment not json": func(s *ServerSpec) { s.Environment = "{oops" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			spec := valid
			mutate(&spec)
			if err := spec.validate(); err == nil {
				t.Fatal("invalid spec was accepted")
			}
		})
	}
}
