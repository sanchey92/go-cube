package worker

import (
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sanchey92/go-cube/internal/services/stats"
	"github.com/sanchey92/go-cube/internal/services/task"
)

type Worker interface {
	AddTask(t *task.Task)
	GetTaskByID(taskID uuid.UUID) (*task.Task, bool)
	GetStats() *stats.Stats
	GetTasks() []*task.Task
}

type Server struct {
	address string
	port    int
	router  *chi.Mux
	worker  Worker
}

func NewHTTPServer(addr string, port int, worker Worker) *Server {
	srv := &Server{
		address: addr,
		port:    port,
		worker:  worker,
	}
	srv.initRouter()
	return srv
}

func (srv *Server) GetRouter() *chi.Mux {
	return srv.router
}
