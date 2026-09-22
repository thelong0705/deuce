package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
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

var errInvalidHour = apperr.New(apperr.KindInvalid, "invalid_hour", "from_hour and to_hour must be whole hours between 0 and 24")

// courtSearchResponse is a court with the venue it stands at and what it has
// free. The venue is included because a court's name is no use without it.
type courtSearchResponse struct {
	Court courtResponse  `json:"court"`
	Venue venueResponse  `json:"venue"`
	Slots []slotResponse `json:"slots"`
}

type courtSearchListResponse struct {
	Courts []courtSearchResponse `json:"courts"`
}

func (s *Server) searchCourts(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	day, err := time.Parse(time.DateOnly, query.Get("date"))
	if err != nil {
		writeAppError(w, errInvalidDate)
		return
	}

	// An absent window is the whole day; the court's own hours narrow it.
	fromHour, err := hourParam(query.Get("from_hour"), 0)
	if err != nil {
		writeAppError(w, err)
		return
	}

	toHour, err := hourParam(query.Get("to_hour"), 24)
	if err != nil {
		writeAppError(w, err)
		return
	}

	found, err := s.courts.Search(r.Context(), entity.CourtSearch{
		City:     query.Get("city"),
		Date:     day,
		FromHour: fromHour,
		ToHour:   toHour,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}

	out := make([]courtSearchResponse, 0, len(found))
	for _, c := range found {
		slots := make([]slotResponse, 0, len(c.Slots))
		for _, slot := range c.Slots {
			slots = append(slots, slotResponse{
				StartsAt:  slot.StartsAt,
				EndsAt:    slot.EndsAt(),
				Available: slot.Available,
			})
		}

		out = append(out, courtSearchResponse{
			Court: newCourtResponse(c.Court),
			Venue: newVenueResponse(c.Venue),
			Slots: slots,
		})
	}

	writeJSON(w, http.StatusOK, courtSearchListResponse{Courts: out})
}

// hourParam reads an hour of the day, falling back when the parameter is
// absent. The range is checked by the use case; this only rejects what is not
// a number.
func hourParam(raw string, fallback int) (int, error) {
	if raw == "" {
		return fallback, nil
	}

	hour, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errInvalidHour
	}

	return hour, nil
}
