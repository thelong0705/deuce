package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

// BookingRepo stores bookings.
type BookingRepo interface {
	CreateBooking(ctx context.Context, in entity.BookSlotInput) (*entity.Booking, error)
	// ListBookedSlots returns the start times still held on a court between
	// from inclusive and to exclusive. Cancelled bookings are not held.
	ListBookedSlots(ctx context.Context, courtID uuid.UUID, from, to time.Time) ([]time.Time, error)
	// ListPlayerBookings returns a player's uncancelled bookings starting at
	// or after from, earliest first.
	ListPlayerBookings(ctx context.Context, playerID uuid.UUID, from time.Time) ([]entity.PlayerBooking, error)
}

// CourtFinder looks up a court together with the venue it belongs to, whose
// timezone the court's opening hours are read in.
type CourtFinder interface {
	GetCourtWithVenue(ctx context.Context, id uuid.UUID) (*entity.Court, *entity.Venue, error)
}

type Booking struct {
	bookings BookingRepo
	courts   CourtFinder
	users    UserFinder
}

func NewBooking(bookings BookingRepo, courts CourtFinder, users UserFinder) *Booking {
	return &Booking{bookings: bookings, courts: courts, users: users}
}

// Book holds a slot for a player. Whether the slot is free is decided by
// storage, not by a check here, so two players racing for it cannot both win.
func (s *Booking) Book(ctx context.Context, in entity.BookSlotInput) (*entity.Booking, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}

	player, err := s.users.GetUser(ctx, in.PlayerID)
	if err != nil {
		return nil, err
	}

	if player.Role != entity.RolePlayer {
		return nil, entity.ErrNotAPlayer
	}

	if !player.IsActive {
		return nil, entity.ErrPlayerInactive
	}

	court, venue, err := s.courts.GetCourtWithVenue(ctx, in.CourtID)
	if err != nil {
		return nil, err
	}

	if !venue.IsActive {
		return nil, entity.ErrVenueInactive
	}

	if err := court.ValidateSlot(in.StartsAt, venue.Location(), time.Now()); err != nil {
		return nil, err
	}

	return s.bookings.CreateBooking(ctx, in)
}

// Availability reports the court's hours on one calendar date, resolved in the
// venue's timezone, and whether each is still free.
//
// It is a snapshot, not a reservation: a slot reported free can be taken
// before the player acts on it. Book is where that race is settled.
func (s *Booking) Availability(ctx context.Context, courtID uuid.UUID, day time.Time) ([]entity.Slot, error) {
	if courtID == uuid.Nil {
		return nil, entity.ErrCourtRequired
	}

	court, venue, err := s.courts.GetCourtWithVenue(ctx, courtID)
	if err != nil {
		return nil, err
	}

	if !venue.IsActive {
		return nil, entity.ErrVenueInactive
	}

	starts := court.SlotsOn(day, venue.Location(), time.Now())
	if len(starts) == 0 {
		return []entity.Slot{}, nil
	}

	// One query for the whole day rather than one per hour.
	last := starts[len(starts)-1]
	taken, err := s.bookings.ListBookedSlots(ctx, courtID, starts[0], last.Add(entity.SlotDuration))
	if err != nil {
		return nil, err
	}

	// Keyed by instant, so a booking stored in one offset still matches a slot
	// built in another.
	held := make(map[int64]bool, len(taken))
	for _, at := range taken {
		held[at.UnixNano()] = true
	}

	slots := make([]entity.Slot, 0, len(starts))
	for _, at := range starts {
		slots = append(slots, entity.Slot{StartsAt: at, Available: !held[at.UnixNano()]})
	}

	return slots, nil
}

// ListForPlayer returns the player's bookings from now on. Slots already
// played are not what the list is for.
func (s *Booking) ListForPlayer(ctx context.Context, playerID uuid.UUID) ([]entity.PlayerBooking, error) {
	if playerID == uuid.Nil {
		return nil, entity.ErrPlayerRequired
	}

	return s.bookings.ListPlayerBookings(ctx, playerID, time.Now())
}
