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

	booking, err := s.bookings.Book(r.Context(), entity.BookSlotInput{
		PlayerID: player.ID,
		CourtID:  courtID,
		StartsAt: startsAt,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, newBookingResponse(*booking))
}
