package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

type createVenueRequest struct {
	OwnerID string `json:"owner_id"`
	Name    string `json:"name"`
	City    string `json:"city"`
	Address string `json:"address"`
}

type venueResponse struct {
	ID        uuid.UUID `json:"id"`
	OwnerID   uuid.UUID `json:"owner_id"`
	Name      string    `json:"name"`
	City      string    `json:"city"`
	Address   string    `json:"address"`
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
		IsActive:  v.IsActive,
		CreatedAt: v.CreatedAt,
	}
}

func (s *Server) createVenue(w http.ResponseWriter, r *http.Request) {
	var req createVenueRequest

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	ownerID, err := uuid.Parse(req.OwnerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "owner_id must be a valid uuid")
		return
	}

	venue, err := s.venues.Create(r.Context(), entity.CreateVenueInput{
		OwnerID: ownerID,
		Name:    req.Name,
		City:    req.City,
		Address: req.Address,
	})
	if err != nil {
		writeCreateVenueError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, newVenueResponse(*venue))
}

func writeCreateVenueError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, entity.ErrNotAnOwner),
		errors.Is(err, entity.ErrOwnerInactive):
		writeError(w, http.StatusForbidden, err.Error())

	case errors.Is(err, entity.ErrUserNotFound),
		errors.Is(err, entity.ErrOwnerRequired),
		errors.Is(err, entity.ErrVenueNameRequired),
		errors.Is(err, entity.ErrVenueCityRequired),
		errors.Is(err, entity.ErrVenueAddressRequired):
		writeError(w, http.StatusBadRequest, err.Error())

	default:
		writeError(w, http.StatusInternalServerError, "could not create venue")
	}
}
