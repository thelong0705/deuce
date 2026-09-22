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

// UserUsecase registers users and manages their sessions.
type UserUsecase interface {
	Register(ctx context.Context, in entity.CreateUserInput) (*entity.User, error)
	Login(ctx context.Context, in entity.LoginInput) (string, *entity.Session, error)
	Logout(ctx context.Context, token string) error
	Authenticate(ctx context.Context, token string) (*entity.User, error)
}

// VenueUsecase creates, lists and searches venues.
type VenueUsecase interface {
	Create(ctx context.Context, in entity.CreateVenueInput) (*entity.Venue, error)
	ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]entity.Venue, error)
	Search(ctx context.Context, city string) ([]entity.Venue, error)
}

// CourtUsecase registers and lists the courts at a venue.
type CourtUsecase interface {
	Create(ctx context.Context, in entity.CreateCourtInput) (*entity.Court, error)
	ListByVenue(ctx context.Context, venueID uuid.UUID) ([]entity.Court, error)
}

// CityUsecase reads the cities deuce operates in.
type CityUsecase interface {
	List(ctx context.Context) ([]entity.City, error)
}

// BookingUsecase books slots on a court and reads back what is booked.
type BookingUsecase interface {
	Book(ctx context.Context, in entity.BookSlotInput) (*entity.Booking, error)
	Availability(ctx context.Context, courtID uuid.UUID, day time.Time) ([]entity.Slot, error)
	ListForPlayer(ctx context.Context, playerID uuid.UUID) ([]entity.PlayerBooking, error)
}

type Server struct {
	users    UserUsecase
	venues   VenueUsecase
	courts   CourtUsecase
	bookings BookingUsecase
	cities   CityUsecase
	router   *chi.Mux
}

func NewServer(
	users UserUsecase,
	venues VenueUsecase,
	courts CourtUsecase,
	bookings BookingUsecase,
	cities CityUsecase,
) *Server {
	s := &Server{
		users:    users,
		venues:   venues,
		courts:   courts,
		bookings: bookings,
		cities:   cities,
		router:   chi.NewRouter(),
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
	s.router.Get("/openapi.yaml", s.openAPI)
	s.router.Get("/docs", s.docs)
	s.router.Post("/users", s.createUser)
	s.router.Post("/sessions", s.login)
	s.router.Delete("/sessions", s.logout)
	s.router.Group(func(r chi.Router) {
		r.Use(s.requireAuth)

		r.Get("/me", s.currentUser)
		r.Post("/venues", s.createVenue)
		r.Get("/venues", s.listVenues)
		r.Post("/venues/{venueID}/courts", s.createCourt)
		r.Post("/courts/{courtID}/bookings", s.createBooking)

		r.Get("/venues/search", s.searchVenues)
		r.Get("/venues/{venueID}/courts", s.listCourts)
		r.Get("/courts/{courtID}/availability", s.courtAvailability)
		r.Get("/bookings", s.listBookings)
		r.Get("/cities", s.listCities)
	})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
