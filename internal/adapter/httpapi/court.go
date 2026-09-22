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

var errInvalidVenueID = apperr.New(apperr.KindInvalid, "invalid_venue_id", "venue id must be a valid uuid")

type createCourtRequest struct {
	Name         string `json:"name"`
	OpenHour     int    `json:"open_hour"`
	CloseHour    int    `json:"close_hour"`
	PricePerHour int    `json:"price_per_hour"`
}

type courtResponse struct {
	ID           uuid.UUID `json:"id"`
	VenueID      uuid.UUID `json:"venue_id"`
	Name         string    `json:"name"`
	OpenHour     int       `json:"open_hour"`
	CloseHour    int       `json:"close_hour"`
	PricePerHour int       `json:"price_per_hour"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
}

func newCourtResponse(c entity.Court) courtResponse {
	return courtResponse{
		ID:           c.ID,
		VenueID:      c.VenueID,
		Name:         c.Name,
		OpenHour:     c.OpenHour,
		CloseHour:    c.CloseHour,
		PricePerHour: c.PricePerHour,
		IsActive:     c.IsActive,
		CreatedAt:    c.CreatedAt,
	}
}

func (s *Server) createCourt(w http.ResponseWriter, r *http.Request) {
	owner, ok := UserFromContext(r.Context())
	if !ok {
		writeInternalError(w)
		return
	}

	venueID, err := uuid.Parse(chi.URLParam(r, "venueID"))
	if err != nil {
		writeAppError(w, errInvalidVenueID)
		return
	}

	var req createCourtRequest

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeBody(w, http.StatusBadRequest, invalidBodyResponse)
		return
	}

	court, err := s.courts.Create(r.Context(), entity.CreateCourtInput{
		OwnerID:      owner.ID,
		VenueID:      venueID,
		Name:         req.Name,
		OpenHour:     req.OpenHour,
		CloseHour:    req.CloseHour,
		PricePerHour: req.PricePerHour,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, newCourtResponse(*court))
}

type courtListResponse struct {
	Courts []courtResponse `json:"courts"`
}

func (s *Server) listCourts(w http.ResponseWriter, r *http.Request) {
	venueID, err := uuid.Parse(chi.URLParam(r, "venueID"))
	if err != nil {
		writeAppError(w, errInvalidVenueID)
		return
	}

	courts, err := s.courts.ListByVenue(r.Context(), venueID)
	if err != nil {
		writeAppError(w, err)
		return
	}

	out := make([]courtResponse, 0, len(courts))
	for _, c := range courts {
		out = append(out, newCourtResponse(c))
	}

	writeJSON(w, http.StatusOK, courtListResponse{Courts: out})
}
