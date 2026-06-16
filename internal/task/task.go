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
	ID    uuid.UUID
	Name  string
	State State
	// Docker image
	Image string
	// Resources for tasks
	Memory int
	Disk   int
	// Ensure the machine allocate proper network ports for tasks
	ExposedPorts nat.PortSet
	PortBindings map[string]string
	// Tell the system what to do
	// when a task stops or fails unexpectedly
	RestartPolicy string
	StartTime     time.Time
	FinishTIme    time.Time
}

// Tell the system when to stop the task
type TaskEvent struct {
	ID        uuid.UUID
	State     State
	Timestamp time.Time
	Task      Task
}
