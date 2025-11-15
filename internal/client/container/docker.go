package client

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/sanchey92/go-cube/internal/task"
)

type Docker struct {
	client *client.Client
}

type Result struct {
	Err         error
	Action      string
	ContainerID string
	Result      string
}

type InspectResponse struct {
	Err       error
	Container *container.InspectResponse
}

func NewDocker() (*Docker, error) {
	dc, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("cannot create container client: %v", err)
	}

	return &Docker{
		client: dc,
	}, nil
}

func (d *Docker) Run(ctx context.Context, t *task.Task) *Result {
	if err := d.pullImage(ctx, t.Image); err != nil {
		return &Result{Err: fmt.Errorf("pull image failed: %w", err)}
	}

	containerID, err := d.createContainer(ctx, t)
	if err != nil {
		return &Result{Err: fmt.Errorf("create container failed: %w", err)}
	}

	if err = d.startContainer(ctx, containerID); err != nil {
		_ = d.removeContainer(ctx, containerID)
		return &Result{Err: fmt.Errorf("start container failed: %w", err)}
	}

	if err = d.streamLogs(ctx, containerID); err != nil {
		log.Printf("Warning: failed to stream logs for container %s: %v\n", containerID, err)
	}

	return &Result{
		ContainerID: containerID,
		Action:      "start",
		Result:      "success",
	}
}

func (d *Docker) StopAndRemove(ctx context.Context, id string) *Result {
	log.Printf("Attempting to stop container: %v", id)

	var err error

	if err = d.client.ContainerStop(ctx, id, container.StopOptions{}); err != nil {
		log.Printf("Error stopping container %s: %v\n", id, err)
		return &Result{Err: err}
	}

	if err = d.removeContainer(ctx, id); err != nil {
		log.Printf("Error renoving container: %v", err)
		return &Result{Err: err}
	}

	return &Result{Action: "stop", Result: "success", Err: nil}
}

func (d *Docker) Inspect(ctx context.Context, containerID string) *InspectResponse {
	resp, err := d.client.ContainerInspect(ctx, containerID)
	if err != nil {
		log.Printf("Error inspectiong container: %s\n", err)
		return &InspectResponse{Err: err}
	}
	return &InspectResponse{Container: &resp}
}

func (d *Docker) pullImage(ctx context.Context, imageName string) error {
	reader, err := d.client.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", imageName, err)
	}
	defer reader.Close()

	if _, err = io.Copy(os.Stdout, reader); err != nil {
		log.Printf("Warning: failed to copy pull output: %v\n", err)
	}

	return nil
}

func (d *Docker) createContainer(ctx context.Context, t *task.Task) (string, error) {
	containerConfig := d.buildContainerConfig(t)
	hostConfig := d.buildHostConfig(t)

	resp, err := d.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, t.Name)
	if err != nil {
		return "", fmt.Errorf("failed to create container from image %s: %w", t.Image, err)
	}

	return resp.ID, nil
}

func (d *Docker) buildContainerConfig(t *task.Task) *container.Config {
	return &container.Config{
		Image:        t.Image,
		Tty:          false,
		Env:          t.Env,
		ExposedPorts: t.ExposedPorts,
	}
}

func (d *Docker) buildHostConfig(t *task.Task) *container.HostConfig {
	return &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyMode(t.RestartPolicy),
		},
		Resources: container.Resources{
			Memory:   t.Memory,
			NanoCPUs: int64(t.CPU * math.Pow(10, 9)),
		},
		PublishAllPorts: true,
	}
}

func (d *Docker) startContainer(ctx context.Context, containerID string) error {
	if err := d.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container %s: %w", containerID, err)
	}
	return nil
}

func (d *Docker) removeContainer(ctx context.Context, containerID string) error {
	return d.client.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force:         true,
		RemoveLinks:   false,
		RemoveVolumes: true,
	})
}

func (d *Docker) streamLogs(ctx context.Context, containerID string) error {
	out, err := d.client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return fmt.Errorf("failed to get logs for container %s: %w", containerID, err)
	}
	defer out.Close()

	if _, err := stdcopy.StdCopy(os.Stdout, os.Stderr, out); err != nil {
		return fmt.Errorf("failed to copy logs: %w", err)
	}

	return nil
}
