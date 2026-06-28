package manager

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"github.com/honganh1206/smolk8s/internal/api"
	"github.com/honganh1206/smolk8s/internal/task"
)

type Server struct {
	Address string
	Port    int
	Manager *Manager
	Router  *chi.Mux
}

func NewServer(address string, port int, m *Manager) *Server {
	return &Server{
		Address: address,
		Port:    port,
		Manager: m,
	}
}

func (s *Server) InitRouter() {
	s.Router = chi.NewRouter()
	s.Router.Route("/tasks", func(r chi.Router) {
		r.Post("/", s.StartTaskHandler)
		r.Get("/", s.GetTasksHandler)
		r.Route("/{taskID}", func(r chi.Router) {
			r.Delete("/", s.StopTaskHandler)
		})
	})
}

func (s *Server) Start() {
	s.InitRouter()
	http.ListenAndServe(fmt.Sprintf("%s:%d", s.Address, s.Port), s.Router)
}

func (s *Server) StartTaskHandler(w http.ResponseWriter, r *http.Request) {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	te := task.TaskEvent{}

	err := d.Decode(&te)
	if err != nil {
		msg := fmt.Sprintf("Error unmarshalling body: %v\n", err)
		log.Print(msg)
		w.WriteHeader(400)
		e := api.ErrResponse{
			HTTPStatusCode: 400,
			Message:        msg,
		}
		json.NewEncoder(w).Encode(e)
		return
	}

	s.Manager.addTask(te)
	log.Printf("Added task %v\n", te.Task.ID)
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(te.Task)
}

func (s *Server) GetTasksHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(s.Manager.getTasks())
}

func (s *Server) StopTaskHandler(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	if taskID == "" {
		log.Printf("No taskID passed in request.\n")
		w.WriteHeader(400)
	}
	tID, _ := uuid.Parse(taskID)

	taskToStop, ok := s.Manager.TaskDb[tID]
	if !ok {
		log.Printf("No task with ID %v found", tID)
		w.WriteHeader(404)
	}

	te := task.TaskEvent{
		ID:        uuid.New(),
		State:     task.Completed,
		Timestamp: time.Now(),
	}

	// Do NOT modify the task instance in the DB
	taskCopy := *taskToStop
	taskCopy.State = task.Completed
	te.Task = taskCopy

	s.Manager.addTask(te)
	log.Printf("Added task event %v to stop task %v\n", te.ID, taskToStop.ID)
	w.WriteHeader(204)
}
