package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

// UserRegistrar is declared here, by the handler that uses it. usecase.User
// satisfies it without either package importing the other's concrete types.
type UserRegistrar interface {
	Register(ctx context.Context, in entity.CreateUserInput) (*entity.User, error)
}

type Server struct {
	users  UserRegistrar
	router *chi.Mux
}

func NewServer(users UserRegistrar) *Server {
	s := &Server{
		users:  users,
		router: chi.NewRouter(),
	}
	s.routes()
	return s
}

// Handler returns the router as a plain http.Handler, so nothing outside this
// package needs to know which router is used.
func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) routes() {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(30 * time.Second))

	s.router.Get("/healthz", s.health)
	s.router.Post("/users", s.createUser)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
