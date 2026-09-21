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
