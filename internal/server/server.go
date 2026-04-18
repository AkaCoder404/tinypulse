package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"tinypulse/internal/db"
	"tinypulse/internal/monitor"
)

type Server struct {
	router   *chi.Mux
	db       *db.DB
	manager  *monitor.Manager
	password string
}

func New(database *db.DB, manager *monitor.Manager, password string) *Server {
	s := &Server{
		router:   chi.NewRouter(),
		db:       database,
		manager:  manager,
		password: password,
	}

	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)

	if s.password != "" {
		// Use a hardcoded username like "admin" since they only provide a password
		s.router.Use(middleware.BasicAuth("TinyPulse", map[string]string{
			"admin": s.password,
		}))
	}

	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) routes() {
	s.router.Get("/", serveIndex)
	s.router.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(StaticFiles()))))

	s.router.Route("/api", func(r chi.Router) {
		r.Use(middleware.SetHeader("Content-Type", "application/json"))

		// Endpoints
		r.Get("/endpoints", s.listEndpoints)
		r.Post("/endpoints", s.createEndpoint)
		r.Get("/endpoints/{id}", s.getEndpoint)
		r.Put("/endpoints/{id}", s.updateEndpoint)
		r.Delete("/endpoints/{id}", s.deleteEndpoint)
		r.Post("/endpoints/{id}/pause", s.pauseEndpoint)
		r.Get("/endpoints/{id}/checks", s.listChecks)
		r.Get("/endpoints/{id}/history", s.history)

		// Notifiers
		r.Get("/notifiers", s.listNotifiers)
		r.Post("/notifiers", s.createNotifier)
		r.Get("/notifiers/{id}", s.getNotifier)
		r.Put("/notifiers/{id}", s.updateNotifier)
		r.Delete("/notifiers/{id}", s.deleteNotifier)
		r.Post("/notifiers/{id}/test", s.testNotifier)
	})
}
