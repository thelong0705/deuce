package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/thelong0705/deuce/internal/domain/entity"
	"github.com/thelong0705/deuce/internal/pkg/apperr"
)

var (
	errInvalidCourtID  = apperr.New(apperr.KindInvalid, "invalid_court_id", "court id must be a valid uuid")
	errInvalidStartsAt = apperr.New(apperr.KindInvalid, "invalid_starts_at", "starts_at must be an RFC 3339 timestamp")
)

type createBookingRequest struct {
	StartsAt string `json:"starts_at"`
}

type heldBookingResponse struct {
	bookingResponse
	// ClientSecret is what the browser completes the payment with. It is not
	// stored, and this is the only time it is sent.
	ClientSecret string `json:"client_secret"`
}

type bookingResponse struct {
	ID        uuid.UUID `json:"id"`
	CourtID   uuid.UUID `json:"court_id"`
	PlayerID  uuid.UUID `json:"player_id"`
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	CreatedAt time.Time `json:"created_at"`
}

func newBookingResponse(b entity.Booking) bookingResponse {
	return bookingResponse{
		ID:        b.ID,
		CourtID:   b.CourtID,
		PlayerID:  b.PlayerID,
		StartsAt:  b.StartsAt,
		EndsAt:    b.EndsAt(),
		CreatedAt: b.CreatedAt,
	}
}

func (s *Server) createBooking(w http.ResponseWriter, r *http.Request) {
	player, ok := UserFromContext(r.Context())
	if !ok {
		writeInternalError(w)
		return
	}

	courtID, err := uuid.Parse(chi.URLParam(r, "courtID"))
	if err != nil {
		writeAppError(w, errInvalidCourtID)
		return
	}

	var req createBookingRequest

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeBody(w, http.StatusBadRequest, invalidBodyResponse)
		return
	}

	// The offset the player sends is kept: 09:00+07:00 and 02:00Z are the same
	// instant, and the slot rules read it in the venue's timezone either way.
	startsAt, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil {
		writeAppError(w, errInvalidStartsAt)
		return
	}

	held, err := s.bookings.Book(r.Context(), entity.BookSlotInput{
		PlayerID: player.ID,
		CourtID:  courtID,
		StartsAt: startsAt,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, heldBookingResponse{
		bookingResponse: newBookingResponse(held.Booking),
		ClientSecret:    held.ClientSecret,
	})
}

var errInvalidDate = apperr.New(apperr.KindInvalid, "invalid_date", "date must be YYYY-MM-DD")

type slotResponse struct {
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	Available bool      `json:"available"`
}

type availabilityResponse struct {
	Slots []slotResponse `json:"slots"`
}

func (s *Server) courtAvailability(w http.ResponseWriter, r *http.Request) {
	courtID, err := uuid.Parse(chi.URLParam(r, "courtID"))
	if err != nil {
		writeAppError(w, errInvalidCourtID)
		return
	}

	// A bare date names a calendar day, not an instant; the use case resolves
	// it in the venue's timezone.
	day, err := time.Parse(time.DateOnly, r.URL.Query().Get("date"))
	if err != nil {
		writeAppError(w, errInvalidDate)
		return
	}

	slots, err := s.bookings.Availability(r.Context(), courtID, day)
	if err != nil {
		writeAppError(w, err)
		return
	}

	out := make([]slotResponse, 0, len(slots))
	for _, slot := range slots {
		out = append(out, slotResponse{
			StartsAt:  slot.StartsAt,
			EndsAt:    slot.EndsAt(),
			Available: slot.Available,
		})
	}

	writeJSON(w, http.StatusOK, availabilityResponse{Slots: out})
}

// playerBookingResponse is a booking with enough of the court and venue to
// read it.
type playerBookingResponse struct {
	bookingResponse
	Court playerBookingCourt `json:"court"`
	Venue playerBookingVenue `json:"venue"`
}

type playerBookingCourt struct {
	Name         string `json:"name"`
	PricePerHour int    `json:"price_per_hour"`
	Currency     string `json:"currency"`
}

type playerBookingVenue struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	City string    `json:"city"`
}

type bookingListResponse struct {
	Bookings []playerBookingResponse `json:"bookings"`
}

func (s *Server) listBookings(w http.ResponseWriter, r *http.Request) {
	player, ok := UserFromContext(r.Context())
	if !ok {
		writeInternalError(w)
		return
	}

	bookings, err := s.bookings.ListForPlayer(r.Context(), player.ID)
	if err != nil {
		writeAppError(w, err)
		return
	}

	out := make([]playerBookingResponse, 0, len(bookings))
	for _, b := range bookings {
		out = append(out, playerBookingResponse{
			bookingResponse: newBookingResponse(b.Booking),
			Court: playerBookingCourt{
				Name:         b.Court.Name,
				PricePerHour: b.Court.PricePerHour,
				Currency:     b.Court.Currency.String(),
			},
			Venue: playerBookingVenue{
				ID:   b.Venue.ID,
				Name: b.Venue.Name,
				City: b.Venue.City,
			},
		})
	}

	writeJSON(w, http.StatusOK, bookingListResponse{Bookings: out})
}
