package entity

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/thelong0705/deuce/internal/pkg/apperr"
)

// Venue is a site with courts, owned by one user.
type Venue struct {
	ID        uuid.UUID
	OwnerID   uuid.UUID
	Name      string
	City      string
	Address   string
	IsActive  bool
	CreatedAt time.Time
}

var (
	ErrOwnerRequired        = apperr.New(apperr.KindInvalid, "owner_required", "owner is required")
	ErrVenueNameRequired    = apperr.New(apperr.KindInvalid, "venue_name_required", "venue name is required")
	ErrVenueCityRequired    = apperr.New(apperr.KindInvalid, "venue_city_required", "venue city is required")
	ErrVenueAddressRequired = apperr.New(apperr.KindInvalid, "venue_address_required", "venue address is required")
)

// CreateVenueInput is what an owner supplies to register a venue.
type CreateVenueInput struct {
	OwnerID uuid.UUID
	Name    string
	City    string
	Address string
}

// Validate returns the first rule the input breaks.
func (in CreateVenueInput) Validate() error {
	if in.OwnerID == uuid.Nil {
		return ErrOwnerRequired
	}
	if strings.TrimSpace(in.Name) == "" {
		return ErrVenueNameRequired
	}
	if strings.TrimSpace(in.City) == "" {
		return ErrVenueCityRequired
	}
	if strings.TrimSpace(in.Address) == "" {
		return ErrVenueAddressRequired
	}
	return nil
}
