package task

import (
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
)

type State int

const (
	Pending State = iota
	Scheduled
	Running
	Completed
	Failed
)

// A process running on a single machine
type Task struct {
	ID          uuid.UUID
	ContainerID string
	Name        string
	// Docker image
	State State
	Image string
	// CPU usage in percentage?
	CPU    float64
	Memory int64
	Disk   int64
	// Ensure the machine allocate proper network ports for tasks
	ExposedPorts nat.PortSet
	PortBindings map[string]string
	// Tell the system what to do
	// when a task stops or fails unexpectedly
	// e.g., always, unless-stopped, on-failure
	RestartPolicy string
	StartTime     time.Time
	FinishTime    time.Time
}

type TaskEvent struct {
	// Tell the system when to stop the task
	ID        uuid.UUID
	State     State
	Timestamp time.Time
	Task      Task
}

// Configuration for orchestration tasks
type Config struct {
	Name string
	// 3 fundamental data streams of the OS
	// whenever a command or process starts executing
	AttachStdin   bool
	AttachStdout  bool
	AttachStderr  bool
	ExposedPorts  nat.PortSet
	Cmd           []string
	Image         string
	CPU           float64
	Memory        int64
	Disk          int64
	Env           []string
	RestartPolicy string
}

func NewConfig(t *Task) *Config {
	return &Config{
		Name:          t.Name,
		ExposedPorts:  t.ExposedPorts,
		Image:         t.Image,
		CPU:           t.CPU,
		Memory:        t.Memory,
		Disk:          t.Disk,
		RestartPolicy: t.RestartPolicy,
	}
}
