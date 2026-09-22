package entity

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/thelong0705/deuce/internal/pkg/apperr"
)

// Currency is what a court's price is quoted in. Only one is supported so
// far; the type exists so adding another is a compiler's problem rather than
// a search for string literals.
type Currency string

const CurrencyVND Currency = "VND"

func (c Currency) Valid() bool {
	return c == CurrencyVND
}

func (c Currency) String() string { return string(c) }

// SupportedCurrencies is what a caller may choose from, for a form to offer.
func SupportedCurrencies() []Currency {
	return []Currency{CurrencyVND}
}

// Court is one playable court at a venue.
type Court struct {
	ID           uuid.UUID
	VenueID      uuid.UUID
	Name         string
	OpenHour     int
	CloseHour    int
	PricePerHour int
	Currency     Currency
	IsActive     bool
	CreatedAt    time.Time
}

var (
	ErrVenueRequired      = apperr.New(apperr.KindInvalid, "venue_required", "venue is required")
	ErrCourtNameRequired  = apperr.New(apperr.KindInvalid, "court_name_required", "court name is required")
	ErrCourtHoursInvalid  = apperr.New(apperr.KindInvalid, "court_hours_invalid", "opening hour must be before closing hour, both between 0 and 24")
	ErrCourtPriceNegative = apperr.New(apperr.KindInvalid, "court_price_negative", "price per hour cannot be negative")
	ErrCurrencyInvalid    = apperr.New(apperr.KindInvalid, "currency_invalid", "currency must be VND")
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
	Currency     Currency
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
	if !in.Currency.Valid() {
		return ErrCurrencyInvalid
	}
	return nil
}

var (
	ErrSearchCityRequired = apperr.New(apperr.KindInvalid, "search_city_required", "city is required")
	ErrSearchDateRequired = apperr.New(apperr.KindInvalid, "search_date_required", "date is required")
	ErrSearchHoursInvalid = apperr.New(apperr.KindInvalid, "search_hours_invalid", "from_hour must be before to_hour, both between 0 and 24")
)

// CourtSearch is somewhere to play in a city, on a date, between two hours.
type CourtSearch struct {
	City string
	// Date is read for its calendar date only, in each venue's own timezone.
	Date time.Time
	// ToHour is excluded: a slot starting on it runs past the hour asked for.
	FromHour int
	ToHour   int
}

// Validate returns the first rule the search breaks.
func (in CourtSearch) Validate() error {
	if strings.TrimSpace(in.City) == "" {
		return ErrSearchCityRequired
	}
	if in.Date.IsZero() {
		return ErrSearchDateRequired
	}
	if in.FromHour < 0 || in.ToHour > 24 || in.FromHour >= in.ToHour {
		return ErrSearchHoursInvalid
	}
	return nil
}

// CourtAtVenue is a court with the venue it stands at, whose timezone its
// opening hours are read in.
type CourtAtVenue struct {
	Court Court
	Venue Venue
}

// CourtAvailability is a court that has something free, and what.
type CourtAvailability struct {
	Court Court
	Venue Venue
	Slots []Slot
}

// SlotsWithin is SlotsOn narrowed to local starts between fromHour and toHour.
func (c Court) SlotsWithin(day time.Time, loc *time.Location, now time.Time, fromHour, toHour int) []time.Time {
	var within []time.Time

	for _, at := range c.SlotsOn(day, loc, now) {
		if hour := at.In(loc).Hour(); hour >= fromHour && hour < toHour {
			within = append(within, at)
		}
	}

	return within
}
