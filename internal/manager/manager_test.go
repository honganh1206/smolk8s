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
)

func TestSendWorkEmptyQueue(t *testing.T) {
	m := New([]string{}, "")

	// No pending work: must take the else branch without panic
	// and leave the queue empty.
	m.sendWork()

	if got := m.Pending.Len(); got != 0 {
		t.Fatalf("queue length = %d, want 0", got)
	}
}

func TestSendWorkReEnqueuesOnConnError(t *testing.T) {
	// Un-routable address so the POST fails fast and the task is re-queued.
	addr := "127.0.0.1:0"
	m := New([]string{addr}, "")

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
	m := New([]string{addr}, "")
	m.TaskDb[id] = tk
	// restartTask looks up the worker for this task by ID.
	m.TaskWorkerMap[id] = addr

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

	m := New([]string{addr}, "")
	m.TaskDb[id] = &task.Task{ID: id, State: task.Pending}

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
