package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

type createVenueRequest struct {
	Name     string `json:"name"`
	City     string `json:"city"`
	Address  string `json:"address"`
	Timezone string `json:"timezone"`
}

type venueResponse struct {
	ID        uuid.UUID `json:"id"`
	OwnerID   uuid.UUID `json:"owner_id"`
	Name      string    `json:"name"`
	City      string    `json:"city"`
	Address   string    `json:"address"`
	Timezone  string    `json:"timezone"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

func newVenueResponse(v entity.Venue) venueResponse {
	return venueResponse{
		ID:        v.ID,
		OwnerID:   v.OwnerID,
		Name:      v.Name,
		City:      v.City,
		Address:   v.Address,
		Timezone:  v.Timezone,
		IsActive:  v.IsActive,
		CreatedAt: v.CreatedAt,
	}
}

func (s *Server) createVenue(w http.ResponseWriter, r *http.Request) {
	owner, ok := UserFromContext(r.Context())
	if !ok {
		writeInternalError(w)
		return
	}

	var req createVenueRequest

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeBody(w, http.StatusBadRequest, invalidBodyResponse)
		return
	}

	venue, err := s.venues.Create(r.Context(), entity.CreateVenueInput{
		OwnerID:  owner.ID,
		Name:     req.Name,
		City:     req.City,
		Address:  req.Address,
		Timezone: req.Timezone,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, newVenueResponse(*venue))
}

type venueListResponse struct {
	Venues []venueResponse `json:"venues"`
}

func (s *Server) listVenues(w http.ResponseWriter, r *http.Request) {
	owner, ok := UserFromContext(r.Context())
	if !ok {
		writeInternalError(w)
		return
	}

	venues, err := s.venues.ListByOwner(r.Context(), owner.ID)
	if err != nil {
		writeAppError(w, err)
		return
	}

	out := make([]venueResponse, 0, len(venues))
	for _, v := range venues {
		out = append(out, newVenueResponse(v))
	}

	writeJSON(w, http.StatusOK, venueListResponse{Venues: out})
}
