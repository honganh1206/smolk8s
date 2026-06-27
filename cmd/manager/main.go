package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/honganh1206/smolk8s/internal/manager"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	host := os.Getenv("SMOLK8S_MANAGER_HOST")
	port, _ := strconv.Atoi(os.Getenv("SMOLK8S_MANAGER_PORT"))

	workers := workerList()
	if len(workers) == 0 {
		log.Fatal("No workers configured: set SMOLK8S_WORKERS or SMOLK8S_WORKER_HOST/PORT")
	}

	fmt.Printf("Starting smolk8s manager with %d worker(s): %v\n", len(workers), workers)

	m := manager.New(workers)
	api := manager.NewServer(host, port, m)

	go m.ProcessTasks()
	go m.UpdateTasks()
	go m.DoHealthChecks()

	// Block on the API server.
	api.Start()
}

// workerList reads worker addresses from SMOLK8S_WORKERS (comma-separated
// host:port list). Falls back to a single worker built from
// SMOLK8S_WORKER_HOST/SMOLK8S_WORKER_PORT for local single-node setups.
func workerList() []string {
	if raw := os.Getenv("SMOLK8S_WORKERS"); raw != "" {
		var workers []string
		for _, w := range strings.Split(raw, ",") {
			if w = strings.TrimSpace(w); w != "" {
				workers = append(workers, w)
			}
		}
		return workers
	}

	host := os.Getenv("SMOLK8S_WORKER_HOST")
	port := os.Getenv("SMOLK8S_WORKER_PORT")
	if host == "" || port == "" {
		return nil
	}
	return []string{fmt.Sprintf("%s:%s", host, port)}
}
