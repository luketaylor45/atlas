package installer

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/luketaylor45/atlas/daemon/internal/serverfs"
)

type Installer struct {
	Client *client.Client
}

func New(cli *client.Client) *Installer {
	return &Installer{Client: cli}
}

// Install runs the installation process for a server
// It pulls the installer image, runs a transient container, and executes the script
func (i *Installer) Install(ctx context.Context, uuid string, installImage string, installScript string, envVars []string, reporter ProgressReporter) error {
	log.Printf("[Installer] Starting installation for %s using %s", uuid, installImage)
	report := func(stage string, progress int) {
		if reporter != nil {
			reporter(ProgressUpdate{Stage: stage, Progress: progress})
		}
	}

	// 1. Pull Installer Image
	log.Printf("[Installer] Pulling image: %s", installImage)
	report("Pulling installer image", 0)
	reader, err := i.Client.ImagePull(ctx, installImage, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull installer image: %v", err)
	}
	if err := ConsumeImagePull(reader, func(progress int) {
		report(fmt.Sprintf("Pulling installer image: %d%%", progress), progress*8/100)
	}); err != nil {
		reader.Close()
		return err
	}
	if err := reader.Close(); err != nil {
		return fmt.Errorf("close installer image pull: %w", err)
	}

	// 2. Prepare Data Directory
	report("Preparing server storage", 8)
	localDataDir := serverfs.LocalServicePath(uuid)
	hostDataDir := serverfs.HostServicePath(uuid)
	if err := serverfs.EnsureServiceDirectory(localDataDir); err != nil {
		return fmt.Errorf("prepare server data directory: %w", err)
	}
	defer func() {
		if err := serverfs.NormalizeTree(localDataDir); err != nil {
			log.Printf("[Installer] Warning: failed to normalize permissions for %s: %v", uuid, err)
		}
	}()

	// 3. Prepare Environment
	// Add default required env vars for the script
	env := append([]string{}, envVars...)
	env = append(env, "SERVER_UUID="+uuid)
	env = append(env, "mnt_server=/mnt/server") // Standard Pterodactyl path
	env = append(env, "HOME=/mnt/server")

	// 4. Create Ephemeral Container
	// We mount the host data dir to /mnt/server
	// Installer images expect root for package/bootstrap setup. The daemon
	// normalizes the completed tree to SERVER_UID:SERVER_GID before game start.
	containerConfig := &container.Config{
		Image:      installImage,
		Cmd:        []string{"/bin/ash", "-c", installScript}, // Default to ash (Alpine), script usually handles bash check
		Env:        env,
		Tty:        true,
		WorkingDir: "/mnt/server",
		User:       "0:0",
	}

	// For Windows Daemon, we assume C:\AtlasData is accessible to Docker
	hostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode("bridge"),
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: hostDataDir,
				Target: "/mnt/server",
			},
		},
		AutoRemove: false,
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
	report("Starting installer", 10)
	containerName := "install-" + uuid
	_ = i.Client.ContainerRemove(ctx, containerName, container.RemoveOptions{Force: true})
	resp, err := i.Client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, containerName)
	if err != nil {
		return fmt.Errorf("failed to create installer container: %v", err)
	}

	defer func() {
		i.Client.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})
	}()

	if err := i.Client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start installer container: %v", err)
	}

	statusCh, errCh := i.Client.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)

	// 6. Stream Logs to stdout and extract progress from installer output.
	out, err := i.Client.ContainerLogs(ctx, resp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err != nil {
		return fmt.Errorf("failed to stream installer logs: %v", err)
	}
	recentOutput := newLineTailWriter(8)
	if err := streamScriptOutput(out, io.MultiWriter(os.Stdout, recentOutput), reporter); err != nil {
		out.Close()
		return fmt.Errorf("failed to read installer logs: %v", err)
	}
	out.Close()

	// 7. Wait for completion
	log.Printf("[Installer] Waiting for completion of %s...", resp.ID)

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("error waiting for installer: %v", err)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			details := recentOutput.String()
			if details != "" {
				return fmt.Errorf("installer failed with exit code %d; recent output: %s", status.StatusCode, details)
			}
			return fmt.Errorf("installer failed with exit code %d", status.StatusCode)
		}
	}

	log.Printf("[Installer] Installation completed successfully for %s", uuid)
	if err := serverfs.NormalizeTree(localDataDir); err != nil {
		return fmt.Errorf("normalize installed files: %w", err)
	}
	report("Installation complete", 100)
	return nil
}
