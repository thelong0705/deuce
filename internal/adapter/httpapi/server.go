package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

// UserRegister registers a new user.
type UserRegister interface {
	Register(ctx context.Context, in entity.CreateUserInput) (*entity.User, error)
}

// VenueCreator creates a venue.
type VenueCreator interface {
	Create(ctx context.Context, in entity.CreateVenueInput) (*entity.Venue, error)
}

// VenueLister lists the venues owned by a user.
type VenueLister interface {
	ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]entity.Venue, error)
}

type Server struct {
	users        UserRegister
	venueCreator VenueCreator
	venueLister  VenueLister
	router       *chi.Mux
}

func NewServer(users UserRegister, venueCreator VenueCreator, venueLister VenueLister) *Server {
	s := &Server{
		users:        users,
		venueCreator: venueCreator,
		venueLister:  venueLister,
		router:       chi.NewRouter(),
	}
	s.routes()
	return s
}

// Handler returns the router as a plain http.Handler
func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) routes() {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(30 * time.Second))

	s.router.Get("/healthz", s.health)
	s.router.Post("/users", s.createUser)
	s.router.Post("/venues", s.createVenue)
	s.router.Get("/venues", s.listVenues)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
