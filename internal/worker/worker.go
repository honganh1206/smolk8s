package worker

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/honganh1206/smolk8s/internal/task"
)

// Keep track of tasks
type Worker struct {
	Name string
	// Accept tasks to run from Manager,
	// describing the desired state of the task
	Queue Queue
	// Map UUIDs to tasks as datastore,
	// describing the current state of the task
	Db        map[uuid.UUID]*task.Task
	Stats     *Stats
	TaskCount int
}

func (w *Worker) RunTasks() {
	for {
		if w.Queue.Len() != 0 {
			result := w.runTask()
			if result.Error != nil {
				log.Printf("[worker] Error running task: %v\n", result.Error)
			}
		} else {
			log.Printf("[worker] No tasks to process currently.\n")
		}
		log.Println("[worker] Sleeping for 10 seconds.")
		time.Sleep(10 * time.Second)
	}
}

// runTask identifies the task's current state
// then either start or stop the task
func (w *Worker) runTask() task.DockerResult {
	t := w.Queue.Dequeue()
	if t == nil {
		log.Println("[worker] No tasks in queue")
		return task.DockerResult{Error: nil}
	}

	taskQueued := t.(task.Task)
	fmt.Printf("[worker] Found task in queue: %v:\n", taskQueued)

	// Check for task in current DB or not and duplication
	taskPersisted := w.Db[taskQueued.ID]
	if taskPersisted == nil {
		taskPersisted = &taskQueued
		w.Db[taskQueued.ID] = &taskQueued
	}

	var result task.DockerResult
	if task.ValidateTransition(taskPersisted.State, taskQueued.State) {
		switch taskQueued.State {
		case task.Scheduled:
			// Restart a task by removing the running container and create a new one
			if taskQueued.ContainerID != "" {
				result = w.stopTask(taskQueued)
				if result.Error != nil {
					log.Printf("[worker] %v\n", result.Error)
				}
			}
			result = w.startTask(taskQueued)
		case task.Completed:
			result = w.stopTask(taskQueued)
		default:
			result.Error = errors.New("we should not get here")
		}
	} else {
		err := fmt.Errorf("invalid transition from %v to %v", taskPersisted.State, taskQueued.State)
		result.Error = err
	}

	return result
}

func (w *Worker) addTask(t task.Task) {
	w.Queue.Enqueue(t)
}

func (w *Worker) getTasks() []*task.Task {
	tasks := []*task.Task{}
	for _, t := range w.Db {
		tasks = append(tasks, t)
	}
	return tasks
}

func New(name string) *Worker {
	return &Worker{
		Name:  name,
		Queue: *NewQueue(),
		Db:    make(map[uuid.UUID]*task.Task),
	}
}

func (w *Worker) CollectStats() {
	for {
		log.Println("[worker] Collecting stats...")
		w.Stats = GetStats()
		w.Stats.TaskCount = w.TaskCount
		// Trigger every 15 seconds
		time.Sleep(15 * time.Second)
	}
}

func (w *Worker) startTask(t task.Task) task.DockerResult {
	t.StartTime = time.Now().UTC()
	config := task.NewConfig(&t)
	d := task.NewDocker(config)
	result := d.Run()
	if result.Error != nil {
		log.Printf("[worker] Error running task %v: %v\n", t.ID, result.Error)
		t.State = task.Failed
		w.Db[t.ID] = &t
		return result
	}
	t.ContainerID = result.ContainerID
	t.State = task.Running
	w.Db[t.ID] = &t

	return result
}

func (w *Worker) stopTask(t task.Task) task.DockerResult {
	config := task.NewConfig(&t)
	d := task.NewDocker(config)

	result := d.Stop(t.ContainerID)
	if result.Error != nil {
		log.Printf("[worker] Error stopping container %v: %v\n", t.ContainerID, result.Error)
	}

	t.FinishTime = time.Now().UTC()
	t.State = task.Completed
	// TODO: Do we need mutex here?
	w.Db[t.ID] = &t
	log.Printf("[worker] Stopped and removed container %v for task %v\n", t.ContainerID, t.ID)
	return result
}

func (w *Worker) inspectTask(t *task.Task) task.DockerInspectResponse {
	config := task.NewConfig(t)
	d := task.NewDocker(config)
	return d.Inspect(t.ContainerID)
}

func (w *Worker) UpdateTasks() {
	for {
		log.Println("[worker] Checking status of tasks")
		w.updateTasks()
		log.Println("[worker] Task updates completed")
		log.Println("[worker] Sleeping for 15 seconds")
		time.Sleep(15 * time.Second)
	}
}

func (w *Worker) updateTasks() {
	for id, t := range w.Db {
		if t.State == task.Running {
			resp := w.inspectTask(t)
			if resp.Error != nil {
				fmt.Printf("[worker] ERROR: %v\n", resp.Error)
			}

			if resp.Container == nil {
				log.Printf("[worker] No container for running task %s\n", id)
				w.Db[id].State = task.Failed
			}

			if resp.Container.State.Status == "exited" {
				log.Printf("[worker] Container for task %s in non-running state %s", id, resp.Container.State.Status)
				w.Db[id].State = task.Failed
			}

			w.Db[id].HostPorts = resp.Container.NetworkSettings.Ports
		}
	}
}
