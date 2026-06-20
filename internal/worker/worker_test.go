package worker

import (
	"testing"

	"github.com/google/uuid"
	"github.com/honganh1206/smolk8s/internal/task"
)

func newWorker() *Worker {
	return &Worker{
		Name:  "test-worker",
		Queue: *NewQueue(),
		Db:    make(map[uuid.UUID]*task.Task),
	}
}

func TestAddTaskEnqueues(t *testing.T) {
	w := newWorker()

	w.AddTask(task.Task{ID: uuid.New(), Name: "t1"})

	if got := w.Queue.Len(); got != 1 {
		t.Fatalf("queue length = %d, want 1", got)
	}
}

func TestRunTaskEmptyQueue(t *testing.T) {
	w := newWorker()

	result := w.RunTask()

	if result.Error != nil {
		t.Fatalf("error = %v, want nil", result.Error)
	}
}

func TestRunTaskInvalidTransition(t *testing.T) {
	w := newWorker()
	// Fresh task in Completed has no valid transitions (src == dst == Completed).
	tk := task.Task{ID: uuid.New(), Name: "done", State: task.Completed}
	w.AddTask(tk)

	result := w.RunTask()

	if result.Error == nil {
		t.Fatal("error = nil, want invalid transition error")
	}
	// Task must still be persisted to the datastore.
	if _, ok := w.Db[tk.ID]; !ok {
		t.Fatalf("task %v not persisted to Db", tk.ID)
	}
}

func TestRunTaskUnreachableState(t *testing.T) {
	w := newWorker()
	id := uuid.New()
	// Persist a Running task, then queue the same task still Running.
	// Running -> Running is valid, but the switch has no Running case.
	persisted := task.Task{ID: id, Name: "run", State: task.Running}
	w.Db[id] = &persisted
	w.AddTask(task.Task{ID: id, Name: "run", State: task.Running})

	result := w.RunTask()

	if result.Error == nil || result.Error.Error() != "we should not get here" {
		t.Fatalf("error = %v, want \"we should not get here\"", result.Error)
	}
}
