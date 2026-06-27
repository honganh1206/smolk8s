package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/honganh1206/smolk8s/internal/worker"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	host := os.Getenv("SMOLK8S_WORKER_HOST")
	port, _ := strconv.Atoi(os.Getenv("SMOLK8S_WORKER_PORT"))

	fmt.Println("Starting smolk8s worker")

	w := worker.New("")
	api := worker.NewServer(host, port, w)

	go w.RunTasks()
	go w.CollectStats()
	go w.UpdateTasks()

	// Block on the API server.
	api.Start()
}
