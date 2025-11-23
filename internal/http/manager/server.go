package manager

import (
	"github.com/go-chi/chi/v5"

	"github.com/sanchey92/go-cube/internal/services/task"
)

type Manager interface {
	AddTask(te task.Event)
	GetTasks() ([]*task.Task, error)
	GetTaskByID(taskID string) (*task.Task, error)
}

type Server struct {
	address string
	port    int
	router  *chi.Mux
	manager Manager
}

func NewHTTPServer(addr string, port int, manager Manager) *Server {
	srv := &Server{
		address: addr,
		port:    port,
		manager: manager,
	}

	srv.initRouter()
	return srv
}

func (srv *Server) GetRouter() *chi.Mux {
	return srv.router
}
