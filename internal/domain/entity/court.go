package entity

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/thelong0705/deuce/internal/pkg/apperr"
)

// Court is one playable court at a venue.
type Court struct {
	ID           uuid.UUID
	VenueID      uuid.UUID
	Name         string
	OpenHour     int
	CloseHour    int
	PricePerHour int
	IsActive     bool
	CreatedAt    time.Time
}

var (
	ErrVenueRequired      = apperr.New(apperr.KindInvalid, "venue_required", "venue is required")
	ErrCourtNameRequired  = apperr.New(apperr.KindInvalid, "court_name_required", "court name is required")
	ErrCourtHoursInvalid  = apperr.New(apperr.KindInvalid, "court_hours_invalid", "opening hour must be before closing hour, both between 0 and 24")
	ErrCourtPriceNegative = apperr.New(apperr.KindInvalid, "court_price_negative", "price per hour cannot be negative")
	ErrCourtNameTaken     = apperr.New(apperr.KindConflict, "court_name_taken", "the venue already has a court with that name")
	ErrVenueNotFound      = apperr.New(apperr.KindNotFound, "venue_not_found", "venue not found")
	ErrNotVenueOwner      = apperr.New(apperr.KindForbidden, "not_venue_owner", "only the venue owner can add a court")
	ErrVenueInactive      = apperr.New(apperr.KindForbidden, "venue_inactive", "venue is not active")
)

// CreateCourtInput is what an owner supplies to register a court.
type CreateCourtInput struct {
	// OwnerID is the caller, checked against the venue's owner.
	OwnerID      uuid.UUID
	VenueID      uuid.UUID
	Name         string
	OpenHour     int
	CloseHour    int
	PricePerHour int
}

// Validate returns the first rule the input breaks.
func (in CreateCourtInput) Validate() error {
	if in.OwnerID == uuid.Nil {
		return ErrOwnerRequired
	}
	if in.VenueID == uuid.Nil {
		return ErrVenueRequired
	}
	if strings.TrimSpace(in.Name) == "" {
		return ErrCourtNameRequired
	}
	if in.OpenHour < 0 || in.CloseHour > 24 || in.OpenHour >= in.CloseHour {
		return ErrCourtHoursInvalid
	}
	if in.PricePerHour < 0 {
		return ErrCourtPriceNegative
	}
	return nil
}
