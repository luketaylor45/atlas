package serverfs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/luketaylor45/atlas/daemon/internal/config"
)

func TestRequiredMode(t *testing.T) {
	directoryMode, ok := requiredMode(os.ModeDir | 0500)
	if !ok {
		t.Fatal("directory mode was not recognized")
	}
	if directoryMode.Perm() != 0770 {
		t.Fatalf("directory permissions = %04o, want 0770", directoryMode.Perm())
	}
	if directoryMode&os.ModeSetgid == 0 {
		t.Fatal("directory mode is missing setgid")
	}

	fileMode, ok := requiredMode(0500)
	if !ok {
		t.Fatal("regular file mode was not recognized")
	}
	if fileMode.Perm() != 0760 {
		t.Fatalf("file permissions = %04o, want 0760", fileMode.Perm())
	}

	if _, ok := requiredMode(os.ModeSymlink | 0777); ok {
		t.Fatal("symlink mode should not be chmodded")
	}
}

func TestRepairExistingServices(t *testing.T) {
	uid, gid := currentTestIdentity()
	previous := config.NodeConfig
	config.NodeConfig.ServerUID = uid
	config.NodeConfig.ServerGID = gid
	t.Cleanup(func() { config.NodeConfig = previous })

	dataRoot := t.TempDir()
	serviceRoot := filepath.Join(dataRoot, "12345678-1234-1234-1234-123456789abc")
	if err := os.MkdirAll(filepath.Join(serviceRoot, "steamcmd"), 0500); err != nil {
		t.Fatal(err)
	}
	legacyFile := filepath.Join(serviceRoot, "steamcmd", "steamcmd.sh")
	if err := os.WriteFile(legacyFile, []byte("#!/bin/sh\n"), 0500); err != nil {
		t.Fatal(err)
	}

	if err := RepairExistingServices(dataRoot); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(serviceRoot, permissionMarker)); err != nil {
		t.Fatalf("permission marker was not created: %v", err)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(legacyFile)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0660 != 0660 {
			t.Fatalf("legacy file permissions = %04o, owner/group write was not restored", info.Mode().Perm())
		}
	}
}

func TestServiceDirectoryName(t *testing.T) {
	valid := []string{
		"12345678-1234-1234-1234-123456789abc",
		"ABCDEF12-ABCD-ABCD-ABCD-ABCDEF123456",
	}
	for _, name := range valid {
		if !IsServiceDirectoryName(name) {
			t.Fatalf("valid UUID directory %q was rejected", name)
		}
	}

	invalid := []string{"lost+found", "server-1", "12345678-1234-1234-1234-123456789abz"}
	for _, name := range invalid {
		if IsServiceDirectoryName(name) {
			t.Fatalf("non-service directory %q was accepted", name)
		}
	}
}

func TestConfiguredServicePaths(t *testing.T) {
	previous := config.NodeConfig
	config.NodeConfig.DataPath = "/inside/data"
	config.NodeConfig.HostDataPath = "/host/data"
	config.NodeConfig.ServerUID = 1234
	config.NodeConfig.ServerGID = 5678
	t.Cleanup(func() { config.NodeConfig = previous })

	if got := LocalServicePath("abc"); got != filepath.Join("/inside/data", "abc") {
		t.Fatalf("local path = %q", got)
	}
	if got := HostServicePath("abc"); got != filepath.Join("/host/data", "abc") {
		t.Fatalf("host path = %q", got)
	}
	if got := ContainerUser(); got != "1234:5678" {
		t.Fatalf("container user = %q, want 1234:5678", got)
	}
}
