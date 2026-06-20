package task

import (
	"context"
	"io"
	"log"
	"math"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type Docker struct {
	Client *client.Client
	Config Config
}

type DockerResult struct {
	Error error
	// Start/Stop
	Action      string
	ContainerID string
	Result      string
}

func NewDocker(config *Config) *Docker {
	dc, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Printf("Error initializing Docker client: %v\n", err)
		return &Docker{}
	}
	return &Docker{
		Client: dc,
		Config: *config,
	}
}

// Run pulls the image from Docker and copies to Stdout
func (d *Docker) Run() DockerResult {
	ctx := context.Background()
	reader, err := d.Client.ImagePull(ctx, d.Config.Image, image.PullOptions{})
	if err != nil {
		log.Printf("Error pulling image %s: %v\n", d.Config.Image, err)
		return DockerResult{Error: err}
	}
	if _, err := io.Copy(os.Stdout, reader); err != nil {
		log.Printf("Error copy content to stdout %s: %v\n", d.Config.Image, err)
		return DockerResult{Error: err}
	}

	rp := &container.RestartPolicy{
		Name: container.RestartPolicyMode(d.Config.RestartPolicy),
	}

	r := &container.Resources{
		Memory: d.Config.Memory,
		// 1e9 = 1 core
		NanoCPUs: int64(d.Config.CPU * math.Pow(10, 9)),
	}

	cc := &container.Config{
		Image: d.Config.Image,
		// No terminal interface?
		Tty:          false,
		Env:          d.Config.Env,
		ExposedPorts: d.Config.ExposedPorts,
	}

	hc := &container.HostConfig{
		RestartPolicy: *rp,
		Resources:     *r,
		// Randomly choose available ports on the host
		PublishAllPorts: true,
	}

	resp, err := d.Client.ContainerCreate(ctx, cc, hc, nil, nil, d.Config.Name)
	if err != nil {
		log.Printf("Error creating container using image %s: %v\n", d.Config.Name, err)
		return DockerResult{Error: err}
	}

	// TODO: Does this block anything?
	if err = d.Client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		log.Printf("Error starting container %s: %v\n", resp.ID, err)

		return DockerResult{Error: err}
	}

	// How does this get the logs from running containers?
	out, err := d.Client.ContainerLogs(
		ctx,
		resp.ID,
		container.LogsOptions{ShowStdout: true, ShowStderr: true},
	)
	if err != nil {
		log.Printf("Error getting logs from container %s: %v\n", resp.ID, err)
		return DockerResult{Error: err}
	}

	if _, err := stdcopy.StdCopy(os.Stdout, os.Stderr, out); err != nil {
		log.Printf("Error streaming container logs to stdout and stderr from container %s: %v\n", resp.ID, err)
		return DockerResult{Error: err}
	}

	return DockerResult{
		ContainerID: resp.ID,
		Action:      "start",
		Result:      "success",
	}
}

func (d *Docker) Stop(id string) DockerResult {
	log.Printf("Attempting to stop a container %v...", id)

	// TODO: Why creating a separate ctx here?
	// Can we just share the same context with Run()?
	ctx := context.Background()
	err := d.Client.ContainerStop(ctx, id, container.StopOptions{})
	if err != nil {
		log.Printf("Error stopping container %s: %v\n", id, err)
		return DockerResult{Error: err}
	}

	err = d.Client.ContainerRemove(ctx, id, container.RemoveOptions{
		RemoveVolumes: true,
		// What is a link?
		RemoveLinks: false,
		Force:       false,
	})
	if err != nil {
		log.Printf("Error removing container %s: %v\n", id, err)
		return DockerResult{Error: err}
	}

	return DockerResult{ContainerID: id, Action: "stop", Result: "success", Error: nil}
}
