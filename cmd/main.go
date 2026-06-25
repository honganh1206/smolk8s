package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/honganh1206/smolk8s/internal/manager"
	"github.com/honganh1206/smolk8s/internal/worker"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	whost := os.Getenv("SMOLK8S_WORKER_HOST")
	wport, _ := strconv.Atoi(os.Getenv("SMOLK8S_WORKER_PORT"))

	mhost := os.Getenv("SMOLK8S_MANAGER_HOST")
	mport, _ := strconv.Atoi(os.Getenv("SMOLK8S_MANAGER_PORT"))

	fmt.Println("Starting smolk8s worker")

	w := worker.New("")

	wapi := worker.NewServer(whost, wport, w)

	go w.RunTasks()
	go w.CollectStats()
	go wapi.Start()

	fmt.Println("Starting smolk8s manager")

	workers := []string{fmt.Sprintf("%s:%d", whost, wport)}
	m := manager.New(workers)
	mapi := manager.NewServer(mhost, mport, m)

	go m.ProcessTasks()
	go m.UpdateTasks()
	mapi.Start()
}
