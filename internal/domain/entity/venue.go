package entity

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
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
	ErrOwnerRequired        = errors.New("owner is required")
	ErrVenueNameRequired    = errors.New("venue name is required")
	ErrVenueCityRequired    = errors.New("venue city is required")
	ErrVenueAddressRequired = errors.New("venue address is required")
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
