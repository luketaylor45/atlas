package installer

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/luketaylor45/atlas/daemon/internal/config"
)

type Installer struct {
	Client *client.Client
}

func New(cli *client.Client) *Installer {
	return &Installer{Client: cli}
}

// Install runs the installation process for a server
// It pulls the installer image, runs a transient container, and executes the script
func (i *Installer) Install(ctx context.Context, uuid string, installImage string, installScript string, envVars []string) error {
	log.Printf("[Installer] Starting installation for %s using %s", uuid, installImage)

	// 1. Pull Installer Image
	log.Printf("[Installer] Pulling image: %s", installImage)
	reader, err := i.Client.ImagePull(ctx, installImage, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull installer image: %v", err)
	}
	io.Copy(io.Discard, reader) // Consume output to let pull finish
	reader.Close()

	// 2. Prepare Data Directory
	hostDataDir := filepath.Join(config.NodeConfig.DataPath, uuid)
	if err := os.MkdirAll(hostDataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	// Force ownership so no permission errors when installing
	uid := 1000
	gid := 1000

	if err := os.Chown(hostDataDir, uid, gid); err != nil {
		return fmt.Errorf("failed to set directory ownership: %v", err)
	}

	// 3. Prepare Environment
	// Add default required env vars for the script
	env := append(envVars, "SERVER_UUID="+uuid)
	env = append(env, "mnt_server=/mnt/server") // Standard Pterodactyl path

	// 4. Create Ephemeral Container
	// We mount the host data dir to /mnt/server
	containerConfig := &container.Config{
		Image:      installImage,
		Cmd:        []string{"/bin/ash", "-c", installScript}, // Default to ash (Alpine), script usually handles bash check
		Env:        env,
		Tty:        true,
		WorkingDir: "/mnt/server",
	}

	// For Windows Daemon, we assume C:\AtlasData is accessible to Docker
	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: hostDataDir,
				Target: "/mnt/server",
			},
		},
		AutoRemove: true, // auto-remove after finish
	}

	// Adjust for entrypoint override if needed (Pterodactyl scripts usually expect /bin/bash or /bin/sh)
	// Some images use a custom entrypoint. For now, running the script via sh/ash -c is safest "wrapper".
	// Pterodactyl actually injects the script into the container or passes it as an env var sometimes?
	// Standard Ptero Install:
	// The script is usually passed as the CMD or entrypoint args.
	// Actually, Ptero mounts the install script or curls it.
	// We will attempt to run it directly as a command string.
	// IMPORTANT: Use /bin/sh to ensure compatibility if bash isn't there,
	// but mostly these scripts assume bash.
	if strings.HasPrefix(installScript, "#!/bin/bash") {
		containerConfig.Cmd = []string{"/bin/bash", "-c", installScript}
	} else {
		containerConfig.Cmd = []string{"/bin/sh", "-c", installScript}
	}

	log.Printf("[Installer] Creating container...")
	resp, err := i.Client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "install-"+uuid)
	if err != nil {
		return fmt.Errorf("failed to create installer container: %v", err)
	}

	defer func() {
		// Just in case AutoRemove fails or we crash
		i.Client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
	}()

	if err := i.Client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start installer container: %v", err)
	}

	// 6. Stream Logs to stdout
	out, err := i.Client.ContainerLogs(ctx, resp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err == nil {
		defer out.Close()
		io.Copy(os.Stdout, out)
	}

	// 7. Wait for completion
	log.Printf("[Installer] Waiting for completion of %s...", resp.ID)

	statusCh, errCh := i.Client.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("error waiting for installer: %v", err)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return fmt.Errorf("installer failed with exit code %d", status.StatusCode)
		}
	}

	log.Printf("[Installer] Installation completed successfully for %s", uuid)
	return nil
}
