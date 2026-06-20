package main

import (
	"fmt"
	"time"

	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/honganh1206/smolk8s/internal/task"
	"github.com/honganh1206/smolk8s/internal/worker"
)

func main() {
	db := make(map[uuid.UUID]*task.Task)
	// Should this be working on a different goroutine?
	w := worker.Worker{
		// How to gen worker name?
		Queue: *worker.NewQueue(),
		Db:    db,
	}

	t := task.Task{
		ID:    uuid.New(),
		Name:  "test-container-1",
		State: task.Scheduled,
		Image: "strm/helloworld-http",
	}

	fmt.Println("starting task")
	w.AddTask(t)

	result := w.RunTask()
	if result.Error != nil {
		panic(result.Error)
	}

	t.ContainerID = result.ContainerID
	fmt.Printf("task %s is running in container %s\n", t.ID, t.ContainerID)

	fmt.Println("Sleepy time")
	time.Sleep(time.Second * 30)

	fmt.Printf("stopping task %s\n", t.ID)
	t.State = task.Completed
	w.AddTask(t)
	result = w.RunTask()
	if result.Error != nil {
		panic(result.Error)
	}
}

