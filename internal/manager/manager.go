package manager

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/honganh1206/smolk8s/internal/api"
	"github.com/honganh1206/smolk8s/internal/node"
	"github.com/honganh1206/smolk8s/internal/scheduler"
	"github.com/honganh1206/smolk8s/internal/task"
	"github.com/honganh1206/smolk8s/internal/worker"
)

type Manager struct {
	// Tasks will be placed upon first being submitted?
	Pending worker.Queue
	// In-memory database to store tasks
	TaskDb map[uuid.UUID]*task.Task
	// In-memory database to store task events
	EventDb map[uuid.UUID]*task.TaskEvent
	// A list of <hostname>:<port>
	Workers []string
	// Map of Worker name to task UUIDs
	WorkerTaskMap map[string][]uuid.UUID
	// Map of a task UUID to a Worker
	TaskWorkerMap map[uuid.UUID]string
	WorkerNodes   []*node.Node
	Scheduler     scheduler.Scheduler
}

func New(workers []string, schedulerType string) *Manager {
	taskDb := make(map[uuid.UUID]*task.Task)
	eventDb := make(map[uuid.UUID]*task.TaskEvent)
	workerTaskMap := make(map[string][]uuid.UUID)
	taskWorkerMap := make(map[uuid.UUID]string)

	var nodes []*node.Node
	for worker := range workers {
		workerTaskMap[workers[worker]] = []uuid.UUID{}
		nApi := fmt.Sprintf("http://%v", workers[worker])
		n := node.New(workers[worker], nApi, "worker")
		nodes = append(nodes, n)
	}

	var s scheduler.Scheduler
	switch schedulerType {
	case "roundrobin":
		s = &scheduler.RoundRobin{Name: "roundrobin"}
	default:
		s = &scheduler.RoundRobin{Name: "roundrobin"}
	}
	return &Manager{
		Pending:       *worker.NewQueue(),
		Workers:       workers,
		TaskDb:        taskDb,
		EventDb:       eventDb,
		WorkerTaskMap: workerTaskMap,
		TaskWorkerMap: taskWorkerMap,
		WorkerNodes:   nodes,
		Scheduler:     s,
	}
}

func (m *Manager) UpdateTasks() {
	for {
		log.Println("[manager] Checking for task updates from workers")
		m.updateTasks()
		log.Println("[manager] Task updates completed")
		log.Println("[manager] Sleeping for 15 seconds")
		time.Sleep(15 * time.Second)
	}
}

// updateTasks queries the workers for the tasks and update the state of the tasks
func (m *Manager) updateTasks() {
	for _, w := range m.Workers {
		log.Printf("[manager] Checking worker %v for task updates", w)

		url := fmt.Sprintf("http://%s/tasks", w)
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("[manager] Error connecting to %v: %v\n", w, err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			log.Printf("[manager] Error sending request: %v\n", err)
			continue
		}

		d := json.NewDecoder(resp.Body)
		var tasks []*task.Task
		err = d.Decode(&tasks)
		if err != nil {
			log.Printf("[manager] Error unmarshalling tasks: %s\n", err.Error())
		}

		// Sync task data from workers to manager
		for _, t := range tasks {
			log.Printf("[manager] Attempting to update task %v", t.ID)

			_, ok := m.TaskDb[t.ID]
			if !ok {
				log.Printf("[manager] Task with ID %s not found\n", t.ID)
				continue
			}
			if m.TaskDb[t.ID].State != t.State {
				m.TaskDb[t.ID].State = t.State
			}

			m.TaskDb[t.ID].StartTime = t.StartTime
			m.TaskDb[t.ID].FinishTime = t.FinishTime
			m.TaskDb[t.ID].ContainerID = t.ContainerID
			m.TaskDb[t.ID].HostPorts = t.HostPorts
		}
	}
}

func (m *Manager) addTask(te task.TaskEvent) {
	m.Pending.Enqueue(te)
}

func (m *Manager) getTasks() []*task.Task {
	tasks := []*task.Task{}
	for _, t := range m.TaskDb {
		tasks = append(tasks, t)
	}
	return tasks
}

func (m *Manager) ProcessTasks() {
	for {
		log.Println("[manager] Processing any tasks in the queue")
		m.sendWork()
		log.Println("[manager] Sleeping for 10 seconds")
		time.Sleep(10 * time.Second)
	}
}

// sendWork assigns the worker to the task
// TODO: A lot of maps that need locks
func (m *Manager) sendWork() {
	if m.Pending.Len() > 0 {
		// We might be popping off an existing task (running container)
		e := m.Pending.Dequeue()
		te := e.(task.TaskEvent)
		m.EventDb[te.ID] = &te
		log.Printf("[manager] Pulled %v off pending queue\n", te)

		// Guard check so we cannot stop existing task/running container
		taskWorker, ok := m.TaskWorkerMap[te.Task.ID]
		if ok {
			persistedTask := m.TaskDb[te.Task.ID]
			if te.State == task.Completed && task.ValidateTransition(persistedTask.State, te.State) {
				m.stopTask(taskWorker, te.Task.ID.String())
				return
			}
			log.Printf("[manager] invalid request: existing task %s is in state %v and cannot transition to the completed state\n", persistedTask.ID.String(), persistedTask.State)

			return
		}

		// Copy here so we can use address value
		// but we aren't going to modify it anyway?
		t := te.Task
		w, err := m.selectWorker(&t)
		if err != nil {
			log.Printf("[manager] error selecting worker for task %s: %v\n", t.ID, err)
		}

		// Map worker to task and vice versa
		log.Printf("[manager] selected worker %s for task %s", w.Name, t.ID)
		m.WorkerTaskMap[w.Name] = append(m.WorkerTaskMap[w.Name], te.Task.ID)
		m.TaskWorkerMap[t.ID] = w.Name

		// Save task to the in-memory DB
		t.State = task.Scheduled
		m.TaskDb[t.ID] = &t

		data, err := json.Marshal(te)
		if err != nil {
			log.Printf("[manager] Unable to marshal task object: %v.\n", t)
		}

		// Interact with the Workers
		url := fmt.Sprintf("http://%s/tasks", w.Name)
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
		if err != nil {
			log.Printf("[manager] Error connecting to worker %v: %v\n", w.Name, err)
			// Move task back to queue
			m.Pending.Enqueue(te)
			return
		}

		d := json.NewDecoder(resp.Body)
		if resp.StatusCode != http.StatusCreated {
			e := api.ErrResponse{}
			err := d.Decode(&e)
			if err != nil {
				fmt.Printf("[manager] Error decoding response: %s\n", err.Error())
				return
			}
			log.Printf("[manager] Response error (%d): %s", e.HTTPStatusCode, e.Message)
			return
		}

		// Decode into a separate var: m.TaskDb still holds &t, so reusing t
		// here would overwrite the stored task's state.
		respTask := task.Task{}
		err = d.Decode(&respTask)
		if err != nil {
			fmt.Printf("[manager] Error decoding response: %s\n", err.Error())
			return
		}

		log.Printf("[manager] %#v\n", respTask)
	} else {
		log.Println("[manager] No pending work in the queue")
	}
}

func (m *Manager) selectWorker(t *task.Task) (*node.Node, error) {
	candidates := m.Scheduler.SelectCandidateNodes(t, m.WorkerNodes)
	if candidates == nil {
		msg := fmt.Sprintf("No available candidates match resource request for task %v", t.ID)
		err := errors.New(msg)
		return nil, err
	}
	scores := m.Scheduler.Score(t, candidates)
	selectedNode := m.Scheduler.Pick(scores, candidates)
	return selectedNode, nil
}

func (m *Manager) DoHealthChecks() {
	for {
		log.Println("[manager] Performing task health check")
		m.doHealthChecks()
		log.Println("[manager] Task health checks completed")
		log.Println("[manager] Sleeping for 60 seconds")
		time.Sleep(60 * time.Second)
	}
}

func (m *Manager) doHealthChecks() {
	for _, t := range m.getTasks() {
		if t.State == task.Running && t.RestartCount < 3 {
			err := m.checkTaskHealth(t)
			if err != nil {
				if t.RestartCount < 3 {
					m.restartTask(t)
				}
			}
		} else if t.State == task.Failed && t.RestartCount < 3 {
			m.restartTask(t)
		}
	}
}

func (m *Manager) checkTaskHealth(t *task.Task) error {
	log.Printf("[manager] Calling health check for task %s: %s\n", t.ID, t.HealthCheck)

	w := m.TaskWorkerMap[t.ID]
	hostPort := getHostPort(t.HostPorts)
	if hostPort == "" {
		msg := fmt.Sprintf("[manager] No host port assigned for task %s yet", t.ID)
		log.Println(msg)
		return errors.New(msg)
	}
	worker := strings.Split(w, ":")
	url := fmt.Sprintf("http://%s:%s%s", worker[0], hostPort, t.HealthCheck)
	log.Printf("[manager] Calling health check for task %s: %s\n", t.ID, url)

	resp, err := http.Get(url)
	if err != nil {
		msg := fmt.Sprintf("[manager] Error connecting to health check %s", url)
		log.Println(msg)
		return errors.New(msg)
	}
	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("[manager] Error health check for task %s did not return 200\n", t.ID)
		log.Println(msg)
		return errors.New(msg)
	}

	log.Printf("[manager] Task %s health check response: %v\n", t.ID, resp.StatusCode)
	return nil
}

// restartTask resets the task state and increment the retry counter
func (m *Manager) restartTask(t *task.Task) {
	w := m.TaskWorkerMap[t.ID]

	// Move the task in the DB back to Scheduled
	t.State = task.Scheduled
	t.RestartCount++

	m.TaskDb[t.ID] = t

	te := task.TaskEvent{
		ID:        uuid.New(),
		State:     task.Running,
		Timestamp: time.Now(),
		Task:      *t,
	}

	data, err := json.Marshal(te)
	if err != nil {
		log.Printf("[manager] Unable to marshal task object: %v.", t)
		return
	}

	// Resend the task to the worker
	url := fmt.Sprintf("http://%s/tasks", w)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("[manager] Error connecting to %v: %v", w, err)
		m.Pending.Enqueue(te)
		return
	}

	d := json.NewDecoder(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		e := api.ErrResponse{}
		err := d.Decode(&e)
		if err != nil {
			fmt.Printf("[manager] Error decoding response: %s\n", err.Error())
			return
		}
		log.Printf("[manager] Response error (%d): %s", e.HTTPStatusCode, e.Message)
		return
	}

	newTask := task.Task{}
	err = d.Decode(&newTask)
	if err != nil {
		fmt.Printf("[manager] Error decoding response: %s\n", err.Error())
		return
	}

	log.Printf("[manager] %#v\n", t)
}

func (m *Manager) stopTask(worker, taskID string) {
	client := &http.Client{}
	url := fmt.Sprintf("http://%s/tasks/%s", worker, taskID)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		log.Printf("[manager] error creating request to delete task %s: %v\n", taskID, err)
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[manager] error connecting to worker at %s: %v\n", url, err)
		return
	}
	if resp.StatusCode != 204 {
		log.Printf("[manager] Error sending request: %v\n", err)
	}

	log.Printf("[manager] task %s has been scheduled to be stopped", taskID)
	return
}

func getHostPort(ports nat.PortMap) string {
	for k := range ports {
		return ports[k][0].HostPort
	}
	return ""
}
