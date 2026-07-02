package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/honganh1206/smolk8s/internal/manager"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	mhost := os.Getenv("SMOLK8S_MANAGER_HOST")
	mport, _ := strconv.Atoi(os.Getenv("SMOLK8S_MANAGER_PORT"))

	whost := os.Getenv("SMOLK8S_WORKER_HOST")
	wport, _ := strconv.Atoi(os.Getenv("SMOLK8S_WORKER_PORT"))

	workers := []string{
		fmt.Sprintf("%s:%d", whost, wport),
		fmt.Sprintf("%s:%d", whost, wport+1),
		fmt.Sprintf("%s:%d", whost, wport+2),
	}

	fmt.Printf("Starting smolk8s manager with %d worker(s): %v\n", len(workers), workers)

	m := manager.New(workers, "epvm")
	mapi := manager.NewServer(mhost, mport, m)

	go m.ProcessTasks()
	go m.UpdateTasks()
	go m.DoHealthChecks()

	// Block on the API server.
	mapi.Start()
}
