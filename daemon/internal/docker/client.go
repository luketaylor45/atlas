package docker

import (
	"context"
	"log"
	"time"

	"github.com/docker/docker/client"
)

var Client *client.Client

func Init() {
	var err error
	Client, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}

	// Ping to verify connection (with timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = Client.Ping(ctx)
	if err != nil {
		log.Printf("[WARNING] Docker Daemon not reachable: %v", err)
	} else {
		log.Println("Connected to Docker Engine")
	}
}
