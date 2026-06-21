package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
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

	w := worker.Worker{
		Queue: *worker.NewQueue(),
		Db:    make(map[uuid.UUID]*task.Task),
	}

	api := worker.Api{Address: host, Port: port, Worker: &w}

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
	}(&w)

	api.Start()
}
