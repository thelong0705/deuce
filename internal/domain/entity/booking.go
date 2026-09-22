package entity

import (
	"time"

	"github.com/google/uuid"

	"github.com/thelong0705/deuce/internal/pkg/apperr"
)

const (
	// SlotDuration is how long one booking holds a court. Slots sit on a grid
	// of this width from the court's opening hour: 06:00 to 10:00 offers
	// 06:00–08:00 and 08:00–10:00 and nothing between.
	SlotDuration = 2 * time.Hour
	// slotHours is SlotDuration in the whole hours opening times are kept in.
	slotHours = int(SlotDuration / time.Hour)
	// BookingHorizon is how far ahead a slot can be booked.
	BookingHorizon = 14 * 24 * time.Hour
	// CancellationNotice is how long before the slot a player may still cancel.
	CancellationNotice = 24 * time.Hour
)

// Booking is one slot on a court. A block is a booking an owner made to keep
// the slot empty, and carries no player.
type Booking struct {
	ID          uuid.UUID
	CourtID     uuid.UUID
	PlayerID    uuid.UUID
	IsBlock     bool
	StartsAt    time.Time
	CancelledAt *time.Time
	CreatedAt   time.Time
}

// IsActive reports whether the booking still holds its slot.
func (b Booking) IsActive() bool {
	return b.CancelledAt == nil
}

// EndsAt is when the court frees up.
func (b Booking) EndsAt() time.Time {
	return b.StartsAt.Add(SlotDuration)
}

var (
	ErrPlayerRequired          = apperr.New(apperr.KindInvalid, "player_required", "player is required")
	ErrCourtRequired           = apperr.New(apperr.KindInvalid, "court_required", "court is required")
	ErrSlotRequired            = apperr.New(apperr.KindInvalid, "slot_required", "slot start time is required")
	ErrSlotNotOnTheHour        = apperr.New(apperr.KindInvalid, "slot_not_on_the_hour", "a slot starts on the hour")
	ErrSlotNotOnTheGrid        = apperr.New(apperr.KindInvalid, "slot_not_on_the_grid", "a slot starts every two hours from the court's opening time")
	ErrSlotInThePast           = apperr.New(apperr.KindInvalid, "slot_in_the_past", "that slot has already started")
	ErrSlotTooFarAhead         = apperr.New(apperr.KindInvalid, "slot_too_far_ahead", "slots can be booked up to two weeks ahead")
	ErrSlotOutsideOpeningHours = apperr.New(apperr.KindInvalid, "slot_outside_opening_hours", "the court is closed at that hour")
	ErrSlotTaken               = apperr.New(apperr.KindConflict, "slot_taken", "that slot is already taken")
	ErrCourtNotFound           = apperr.New(apperr.KindNotFound, "court_not_found", "court not found")
	ErrCourtInactive           = apperr.New(apperr.KindForbidden, "court_inactive", "court is not active")
	ErrNotAPlayer              = apperr.New(apperr.KindForbidden, "not_a_player", "only a player can book a court")
	ErrPlayerInactive          = apperr.New(apperr.KindForbidden, "player_inactive", "player account is not active")
)

// BookSlotInput is what a player supplies to book a slot.
type BookSlotInput struct {
	PlayerID uuid.UUID
	CourtID  uuid.UUID
	StartsAt time.Time
}

// Validate returns the first rule the input breaks. The rules that depend on
// the court live in Court.ValidateSlot.
func (in BookSlotInput) Validate() error {
	if in.PlayerID == uuid.Nil {
		return ErrPlayerRequired
	}
	if in.CourtID == uuid.Nil {
		return ErrCourtRequired
	}
	if in.StartsAt.IsZero() {
		return ErrSlotRequired
	}
	return nil
}

// Slot is one bookable window on a court and whether it can still be taken.
type Slot struct {
	StartsAt  time.Time
	Available bool
}

// EndsAt is when the window closes.
func (s Slot) EndsAt() time.Time {
	return s.StartsAt.Add(SlotDuration)
}

// PlayerBooking is a booking with the court and venue it is on.
type PlayerBooking struct {
	Booking
	Court Court
	Venue Venue
}

// SlotsOn returns the start of every window the court could take a booking
// for on one calendar date, earliest first. Only the date part of day is
// read; it is resolved in loc, the venue's zone.
//
// Candidates go through ValidateSlot, so what is offered and what would be
// accepted cannot drift apart. Windows already started or past the horizon
// are therefore absent, not present and unavailable.
func (c Court) SlotsOn(day time.Time, loc *time.Location, now time.Time) []time.Time {
	year, month, date := day.Date()

	var slots []time.Time
	// Stepping by the slot width is what puts the windows on a grid.
	for hour := c.OpenHour; hour+slotHours <= c.CloseHour; hour += slotHours {
		startsAt := time.Date(year, month, date, hour, 0, 0, 0, loc)
		if c.ValidateSlot(startsAt, loc, now) == nil {
			slots = append(slots, startsAt)
		}
	}

	return slots
}

// ValidateSlot checks a start time against the court: its opening hours, read
// in the venue's timezone, and whether the court is open for business at all.
// loc is the venue's location and now is the current time.
func (c Court) ValidateSlot(startsAt time.Time, loc *time.Location, now time.Time) error {
	if !c.IsActive {
		return ErrCourtInactive
	}

	// Minutes are checked in the venue's timezone because not every zone is a
	// whole number of hours from UTC.
	local := startsAt.In(loc)
	if local.Minute() != 0 || local.Second() != 0 || local.Nanosecond() != 0 {
		return ErrSlotNotOnTheHour
	}

	if !startsAt.After(now) {
		return ErrSlotInThePast
	}

	if startsAt.After(now.Add(BookingHorizon)) {
		return ErrSlotTooFarAhead
	}

	if local.Hour() < c.OpenHour || local.Hour()+slotHours > c.CloseHour {
		return ErrSlotOutsideOpeningHours
	}

	// Inside opening hours is not enough: a court open from 06:00 takes 08:00
	// but not 07:00.
	if (local.Hour()-c.OpenHour)%slotHours != 0 {
		return ErrSlotNotOnTheGrid
	}

	return nil
}
