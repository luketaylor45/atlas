package serverfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/luketaylor45/atlas/daemon/internal/config"
)

const permissionMarker = ".atlas_permissions_v1"

// ConfigureProcessPermissions prevents requested group-write bits from being masked off.
func ConfigureProcessPermissions() {
	setProcessUmask()
}

func ContainerUser() string {
	return fmt.Sprintf("%d:%d", config.NodeConfig.ServerUID, config.NodeConfig.ServerGID)
}

func LocalServicePath(uuid string) string {
	return filepath.Join(config.NodeConfig.DataPath, uuid)
}

func HostServicePath(uuid string) string {
	return filepath.Join(config.NodeConfig.HostDataPath, uuid)
}

func EnsureDirectory(path string) error {
	if err := os.MkdirAll(path, 0770); err != nil {
		return fmt.Errorf("create directory %s: %w", path, err)
	}
	return NormalizePath(path)
}

func WriteFile(path string, data []byte, mode os.FileMode) error {
	if err := EnsureDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if err := os.Chmod(path, info.Mode().Perm()|mode.Perm()); err != nil {
		return err
	}
	return NormalizePath(path)
}

// EnsureServiceDirectory creates and fully normalizes a service tree.
func EnsureServiceDirectory(root string) error {
	if err := EnsureDirectory(root); err != nil {
		return err
	}
	return NormalizeTree(root)
}

// RepairServiceDirectory performs a full repair once, then only checks the root.
func RepairServiceDirectory(root string) error {
	if err := EnsureDirectory(root); err != nil {
		return err
	}

	marker := filepath.Join(root, permissionMarker)
	if _, err := os.Stat(marker); err == nil {
		return NormalizePath(root)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect permission marker: %w", err)
	}

	if err := NormalizeTree(root); err != nil {
		return err
	}
	if err := WriteFile(marker, []byte("managed by Atlas\n"), 0660); err != nil {
		return fmt.Errorf("write permission marker: %w", err)
	}
	return nil
}

// RepairExistingServices repairs legacy root-owned service trees once at startup.
func RepairExistingServices(dataRoot string) error {
	if err := os.MkdirAll(dataRoot, 0755); err != nil {
		return fmt.Errorf("create data root: %w", err)
	}

	entries, err := os.ReadDir(dataRoot)
	if err != nil {
		return fmt.Errorf("read data root: %w", err)
	}

	var repairErrors []error
	for _, entry := range entries {
		if !entry.IsDir() || !IsServiceDirectoryName(entry.Name()) {
			continue
		}
		serviceRoot := filepath.Join(dataRoot, entry.Name())
		if err := RepairServiceDirectory(serviceRoot); err != nil {
			repairErrors = append(repairErrors, fmt.Errorf("repair %s: %w", entry.Name(), err))
		}
	}
	return errors.Join(repairErrors...)
}

func IsServiceDirectoryName(name string) bool {
	if len(name) != 36 {
		return false
	}
	for index, char := range name {
		switch index {
		case 8, 13, 18, 23:
			if char != '-' {
				return false
			}
		default:
			if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
				return false
			}
		}
	}
	return true
}

func NormalizeTree(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return normalize(path, info.Mode())
	})
}

func NormalizePath(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	return normalize(path, info.Mode())
}

func normalize(path string, mode os.FileMode) error {
	if runtime.GOOS != "windows" {
		if err := os.Lchown(path, config.NodeConfig.ServerUID, config.NodeConfig.ServerGID); err != nil {
			return fmt.Errorf("chown %s: %w", path, err)
		}
	}

	if mode&os.ModeSymlink != 0 {
		return nil
	}

	normalizedMode, ok := requiredMode(mode)
	if !ok {
		return nil
	}

	if err := os.Chmod(path, normalizedMode); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	return nil
}

func requiredMode(mode os.FileMode) (os.FileMode, bool) {
	switch {
	case mode.IsDir():
		return mode.Perm() | 0770 | os.ModeSetgid, true
	case mode.IsRegular():
		return mode.Perm() | 0660, true
	default:
		return 0, false
	}
}
