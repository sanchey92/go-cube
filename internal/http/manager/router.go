package manager

import "github.com/go-chi/chi/v5"

func (srv *Server) initRouter() {
	srv.router = chi.NewRouter()

	srv.router.Route("/tasks", func(r chi.Router) {
		r.Post("/", srv.startTaskHandler)
		r.Get("/", srv.getTasksHandler)
		r.Route("/{taskID}", func(r chi.Router) {
			r.Delete("/", srv.stopTaskHandler)
		})
	})
}
