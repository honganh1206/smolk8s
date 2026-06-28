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

	w1 := worker.New("")
	wapi1 := worker.NewServer(host, port, w1)

	go w1.RunTasks()
	go w1.CollectStats()
	go w1.UpdateTasks()

	go wapi1.Start()

	w2 := worker.New("")
	wapi2 := worker.NewServer(host, port+1, w2)

	go w2.RunTasks()
	go w2.CollectStats()
	go w2.UpdateTasks()

	go wapi2.Start()

	w3 := worker.New("")
	wapi3 := worker.NewServer(host, port+2, w3)

	go w3.RunTasks()
	go w3.CollectStats()
	go w3.UpdateTasks()

	wapi3.Start()
}
