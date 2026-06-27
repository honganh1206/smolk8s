package manager

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/honganh1206/smolk8s/internal/task"
	"github.com/honganh1206/smolk8s/internal/worker"
)

// newWorkerServer spins a real worker API backed by a real Worker and returns
// the worker, the test server, and the "host:port" address (no scheme) that the
// manager expects in its Workers list.
func newWorkerServer(t *testing.T) (*worker.Worker, *httptest.Server, string) {
	t.Helper()

	w := worker.New("")
	workerApi := worker.NewServer("", 0, w)
	workerApi.InitRouter()

	server := httptest.NewServer(workerApi.Router)
	addr := strings.TrimPrefix(server.URL, "http://")
	return w, server, addr
}

// TestSendWorkDeliversTaskToWorker exercises the manager -> worker POST /tasks
// seam: the manager marshals a TaskEvent, the worker decodes it (with
// DisallowUnknownFields) and enqueues the task. Asserts the task crossed the
// wire and the manager updated its own bookkeeping.
func TestSendWorkDeliversTaskToWorker(t *testing.T) {
	w, server, addr := newWorkerServer(t)
	defer server.Close()

	m := New([]string{addr})

	id := uuid.New()
	te := task.TaskEvent{
		ID:   uuid.New(),
		Task: task.Task{ID: id, Name: "t1", Image: "strm/helloworld-http"},
	}
	m.AddTask(te)
	m.sendWork()

	// Worker side: task landed on the queue (StartTaskHandler -> AddTask).
	if got := w.Queue.Len(); got != 1 {
		t.Fatalf("worker queue length = %d, want 1 (task did not cross the wire)", got)
	}

	// Manager side: bookkeeping reflects the dispatch.
	if got, ok := m.TaskWorkerMap[id]; !ok || got != addr {
		t.Errorf("TaskWorkerMap[%v] = %q (ok=%v), want %q", id, got, ok, addr)
	}
	if _, ok := m.EventDb[te.ID]; !ok {
		t.Errorf("EventDb missing event %v", te.ID)
	}
	if stored, ok := m.TaskDb[id]; !ok {
		t.Errorf("TaskDb missing task %v", id)
	} else if stored.State != task.Scheduled {
		t.Errorf("TaskDb[%v].State = %v, want %v", id, stored.State, task.Scheduled)
	}
	if got := len(m.WorkerTaskMap[addr]); got != 1 {
		t.Errorf("WorkerTaskMap[%q] has %d tasks, want 1", addr, got)
	}

	// Manager dequeued the work; nothing left pending.
	if got := m.Pending.Len(); got != 0 {
		t.Errorf("pending queue length = %d, want 0", got)
	}
}

// TestRestartTaskDeliversToWorker exercises the manager -> worker restart path
// against a real worker API: restartTask resends the task as a TaskEvent, the
// worker decodes and enqueues it. Asserts the task crossed the wire and the
// manager reset its state + bumped the retry counter.
func TestRestartTaskDeliversToWorker(t *testing.T) {
	w, server, addr := newWorkerServer(t)
	defer server.Close()

	id := uuid.New()
	tk := &task.Task{ID: id, Name: "t1", Image: "strm/helloworld-http", State: task.Failed}

	m := New([]string{addr})
	m.TaskDb[id] = tk
	m.TaskWorkerMap[id] = addr

	m.restartTask(tk)

	// Worker side: the restarted task was enqueued (StartTaskHandler -> AddTask).
	if got := w.Queue.Len(); got != 1 {
		t.Fatalf("worker queue length = %d, want 1 (restart did not cross the wire)", got)
	}

	// Manager side: task reset to Scheduled with restart count bumped.
	if tk.State != task.Scheduled {
		t.Errorf("State = %v, want %v", tk.State, task.Scheduled)
	}
	if tk.RestartCount != 1 {
		t.Errorf("RestartCount = %d, want 1", tk.RestartCount)
	}
	if m.TaskDb[id] != tk {
		t.Errorf("TaskDb[%v] not updated to the restarted task", id)
	}
	if got := m.Pending.Len(); got != 0 {
		t.Errorf("pending queue length = %d, want 0", got)
	}
}
