package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/honganh1206/smolk8s/internal/apiserver"
	"github.com/honganh1206/smolk8s/internal/manager"
	"github.com/honganh1206/smolk8s/internal/task"
	"github.com/honganh1206/smolk8s/internal/worker"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	// Do we set this manually?
	host := os.Getenv("SMOLK8S_HOST")
	port, _ := strconv.Atoi(os.Getenv("SMOLK8S_PORT"))

	fmt.Println("Starting smolk8s worker")

	w := worker.New("")

	srv := apiserver.New(host, port, w)

	go func(w *worker.Worker) {
		// Check the worker's queue for tasks
		// then run the tasks
		for {
			if w.Queue.Len() != 0 {
				result := w.RunTask()
				if result.Error != nil {
					log.Printf("Error running task: %v\n", result.Error)
				}
			} else {
				log.Printf("No tasks to process currently\n")
			}
			// Wait for incoming tasks
			log.Println("Sleeping for 10 seconds")
			time.Sleep(10 * time.Second)
		}
	}(w)

	go w.CollectStats()
	// The API server runs on a separate goroutine
	// to not block the manager
	go srv.Start()

	// Include the worker instance we created earlier
	workers := []string{fmt.Sprintf("%s:%d", host, port)}
	m := manager.New(workers)

	// Simulate manager handing out tasks
	for i := 0; i < 3; i++ {
		t := task.Task{
			ID:    uuid.New(),
			Name:  fmt.Sprintf("test-container-%d", i),
			State: task.Scheduled,
			Image: "strm/helloworld-http",
		}
		te := task.TaskEvent{
			ID:    uuid.New(),
			State: task.Running,
			Task:  t,
		}
		m.AddTask(te)
		m.SendWork()
	}

	go func() {
		for {
			fmt.Printf("[Manager] Updating tasks from %d workers\n", len(m.Workers))
			m.UpdateTasks()
			time.Sleep(15 * time.Second)
		}
	}()

	for {
		for _, t := range m.TaskDb {
			fmt.Printf("[Manager] Task: id: %s, state: %d\n", t.ID, t.State)
			time.Sleep(15 * time.Second)
		}
	}

	// TODO: Containers are still running after creation
	// so task state never moves to Completed (we are not running one-shot jobs)
}
