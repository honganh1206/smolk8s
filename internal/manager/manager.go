package manager

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/honganh1206/smolk8s/internal/task"
	"github.com/honganh1206/smolk8s/internal/worker"
)

type Manager struct {
	// Tasks will be placed upon first being submitted?
	Pending worker.Queue
	// In-memory database to store tasks
	TaskDb map[string][]*task.Task
	// In-memory database to store task events
	EventDb map[string][]*task.TaskEvent
	Workers []string
	// Map of Workers' names to tasks' UUIDs
	WorkerTaskMap map[string][]uuid.UUID
	// Map of a task's UUID to a Worker
	TaskWorkerMap map[uuid.UUID]string
}

func (m *Manager) SelectWorker() {
	fmt.Println("I will select an appropriate worker")
}

func (m *Manager) UpdateTasks() {
	fmt.Println("I will update tasks")
}

func (m *Manager) SendWork() {
	fmt.Println("I will send work to workers")
}
