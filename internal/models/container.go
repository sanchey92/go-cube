package models

import "github.com/docker/docker/api/types/container"

type ContainerResult struct {
	Err         error
	Action      string
	ContainerID string
	Result      string
}

type InspectResponse struct {
	Err       error
	Container *container.InspectResponse
}
