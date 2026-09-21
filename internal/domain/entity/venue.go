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
	Timezone  string
	IsActive  bool
	CreatedAt time.Time
}

// Location resolves the venue's timezone, falling back to UTC if the stored
// name is not one the runtime knows.
func (v Venue) Location() *time.Location {
	loc, err := time.LoadLocation(v.Timezone)
	if err != nil {
		return time.UTC
	}

	return loc
}

var (
	ErrOwnerRequired        = apperr.New(apperr.KindInvalid, "owner_required", "owner is required")
	ErrVenueNameRequired    = apperr.New(apperr.KindInvalid, "venue_name_required", "venue name is required")
	ErrVenueCityRequired    = apperr.New(apperr.KindInvalid, "venue_city_required", "venue city is required")
	ErrVenueAddressRequired = apperr.New(apperr.KindInvalid, "venue_address_required", "venue address is required")
	ErrVenueTimezoneInvalid = apperr.New(apperr.KindInvalid, "venue_timezone_invalid", "timezone must be an IANA name such as Asia/Ho_Chi_Minh")
	ErrNotAnOwner           = apperr.New(apperr.KindForbidden, "not_an_owner", "only an owner can register a venue")
	ErrOwnerInactive        = apperr.New(apperr.KindForbidden, "owner_inactive", "owner account is not active")
)

// CreateVenueInput is what an owner supplies to register a venue.
type CreateVenueInput struct {
	OwnerID uuid.UUID
	Name    string
	City    string
	Address string
	// Timezone is an IANA name; the courts' opening hours are read in it.
	Timezone string
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
	if _, err := time.LoadLocation(in.Timezone); err != nil || in.Timezone == "" || in.Timezone == "Local" {
		return ErrVenueTimezoneInvalid
	}
	return nil
}
