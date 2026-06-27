package manager

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/honganh1206/smolk8s/internal/task"
	"github.com/honganh1206/smolk8s/internal/worker"
)

func TestSelectWorkerRoundRobin(t *testing.T) {
	m := &Manager{
		Workers: []string{"w0", "w1", "w2"},
	}

	// LastWorker starts at 0, so the first pick advances to index 1
	// and the sequence wraps back to w0 after the last worker.
	want := []string{"w1", "w2", "w0", "w1", "w2", "w0"}
	for i, expected := range want {
		if got := m.SelectWorker(); got != expected {
			t.Fatalf("call %d: SelectWorker() = %q, want %q", i, got, expected)
		}
	}
}

func TestSelectWorkerSingle(t *testing.T) {
	m := &Manager{Workers: []string{"only"}}

	for i := 0; i < 3; i++ {
		if got := m.SelectWorker(); got != "only" {
			t.Fatalf("call %d: SelectWorker() = %q, want %q", i, got, "only")
		}
		if m.LastWorker != 0 {
			t.Fatalf("call %d: LastWorker = %d, want 0", i, m.LastWorker)
		}
	}
}

func TestSendWorkEmptyQueue(t *testing.T) {
	m := &Manager{
		Pending: *worker.NewQueue(),
	}

	// No pending work: must take the else branch without panic
	// and leave the queue empty.
	m.sendWork()

	if got := m.Pending.Len(); got != 0 {
		t.Fatalf("queue length = %d, want 0", got)
	}
}

func TestSendWorkReEnqueuesOnConnError(t *testing.T) {
	m := &Manager{
		Pending:       *worker.NewQueue(),
		TaskDb:        make(map[uuid.UUID]*task.Task),
		EventDb:       make(map[uuid.UUID]*task.TaskEvent),
		WorkerTaskMap: make(map[string][]uuid.UUID),
		TaskWorkerMap: make(map[uuid.UUID]string),
		// Un-routable address so the POST fails fast and the task is requeued.
		Workers: []string{"127.0.0.1:0"},
	}

	te := task.TaskEvent{
		ID:   uuid.New(),
		Task: task.Task{ID: uuid.New(), Name: "t1"},
	}
	m.Pending.Enqueue(te)

	m.sendWork()

	// Connection failed, so the task event goes back on the queue.
	if got := m.Pending.Len(); got != 1 {
		t.Fatalf("queue length = %d, want 1 (task should be requeued)", got)
	}
	// Bookkeeping maps were still populated before the failed POST.
	if _, ok := m.TaskWorkerMap[te.Task.ID]; !ok {
		t.Errorf("TaskWorkerMap missing entry for task %v", te.Task.ID)
	}
	if _, ok := m.EventDb[te.ID]; !ok {
		t.Errorf("EventDb missing entry for event %v", te.ID)
	}
}

func TestRestartTask(t *testing.T) {
	id := uuid.New()

	// Worker accepts the resent task: 201 + task body, like StartTaskHandler.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(task.Task{ID: id})
	}))
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")

	// A failed task that has already been restarted once.
	tk := &task.Task{ID: id, Name: "t1", State: task.Failed, RestartCount: 1}
	m := &Manager{
		Pending:       *worker.NewQueue(),
		TaskDb:        map[uuid.UUID]*task.Task{id: tk},
		EventDb:       make(map[uuid.UUID]*task.TaskEvent),
		WorkerTaskMap: make(map[string][]uuid.UUID),
		// restartTask looks up the worker for this task by ID.
		TaskWorkerMap: map[uuid.UUID]string{id: addr},
	}

	m.restartTask(tk)

	// Task reset to Scheduled with the retry counter bumped.
	if tk.State != task.Scheduled {
		t.Errorf("State = %v, want %v", tk.State, task.Scheduled)
	}
	if tk.RestartCount != 2 {
		t.Errorf("RestartCount = %d, want 2", tk.RestartCount)
	}
	if m.TaskDb[id] != tk {
		t.Errorf("TaskDb[%v] not updated to the restarted task", id)
	}
	// Retry succeeded, so no requeue
	if got := m.Pending.Len(); got != 0 {
		t.Errorf("pending queue length = %d, want 0", got)
	}
}

func TestUpdateTasksSyncsState(t *testing.T) {
	id := uuid.New()
	start := time.Now().UTC()
	finish := start.Add(time.Minute)

	// Worker reports the task as Running with timing + container info.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]*task.Task{
			{
				ID:          id,
				State:       task.Running,
				StartTime:   start,
				FinishTime:  finish,
				ContainerID: "abc123",
			},
		})
	}))
	defer server.Close()

	// UpdateTasks builds "http://%s/tasks", so pass host:port without scheme.
	addr := strings.TrimPrefix(server.URL, "http://")

	m := &Manager{
		TaskDb:  map[uuid.UUID]*task.Task{id: {ID: id, State: task.Pending}},
		Workers: []string{addr},
	}

	m.updateTasks()

	got := m.TaskDb[id]
	if got.State != task.Running {
		t.Errorf("State = %v, want %v", got.State, task.Running)
	}
	if got.ContainerID != "abc123" {
		t.Errorf("ContainerID = %q, want %q", got.ContainerID, "abc123")
	}
	if !got.StartTime.Equal(start) {
		t.Errorf("StartTime = %v, want %v", got.StartTime, start)
	}
	if !got.FinishTime.Equal(finish) {
		t.Errorf("FinishTime = %v, want %v", got.FinishTime, finish)
	}
}

