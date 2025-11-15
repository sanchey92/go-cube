package main

import (
	"context"
	"fmt"
	"log"
	"time"

	client "github.com/sanchey92/go-cube/internal/client/container"
	"github.com/sanchey92/go-cube/internal/task"
)

func main() {
	t := &task.Task{
		Name:  "test_container_1",
		Image: "postgres:13",
		Env: []string{
			"POSTGRES_USER=user",
			"POSTGRES_PASSWORD=password",
		},
	}

	dockerClient, err := client.NewDocker()
	if err != nil {
		log.Fatalf("failed to create container cclient: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result := dockerClient.Run(ctx, t)
	if result.Err != nil {
		log.Fatalf("cannot start container: %v", result.Err)
	}

	if result.Result == "success" {
		log.Printf("Success start container: %s", result.ContainerID)
	}

	time.Sleep(time.Second * 60)

	fmt.Printf("Stopping container: %s", result.ContainerID)
	_ = dockerClient.StopAndRemove(ctx, result.ContainerID)
}
