package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

// BookingRepo stores bookings.
type BookingRepo interface {
	// HoldSlot writes the booking as pending payment. It is what takes the
	// slot, so a second caller for the same slot gets ErrSlotTaken.
	HoldSlot(ctx context.Context, in entity.BookSlotInput, amount int, holdExpiresAt time.Time) (*entity.Booking, error)
	// AttachPayment records which payment a held slot is waiting on.
	AttachPayment(ctx context.Context, bookingID uuid.UUID, paymentIntentID string) error
	// CancelBooking releases the slot.
	CancelBooking(ctx context.Context, bookingID uuid.UUID) error
	// ListBookedSlots returns the start times still held on a court, from
	// inclusive to exclusive.
	ListBookedSlots(ctx context.Context, courtID uuid.UUID, from, to time.Time) ([]time.Time, error)
	// ListPlayerBookings returns a player's uncancelled bookings from then on.
	ListPlayerBookings(ctx context.Context, playerID uuid.UUID, from time.Time) ([]entity.PlayerBooking, error)
}

// PaymentGateway collects money for a slot.
type PaymentGateway interface {
	CreatePayment(ctx context.Context, in entity.PaymentRequest) (*entity.Payment, error)
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
	payments PaymentGateway
}

func NewBooking(bookings BookingRepo, courts CourtFinder, users UserFinder, payments PaymentGateway) *Booking {
	return &Booking{bookings: bookings, courts: courts, users: users, payments: payments}
}

// Book holds a slot for a player. Whether the slot is free is decided by
// storage, not by a check here, so two players racing for it cannot both win.
func (s *Booking) Book(ctx context.Context, in entity.BookSlotInput) (*entity.HeldBooking, error) {
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

	now := time.Now()
	if err := court.ValidateSlot(in.StartsAt, venue.Location(), now); err != nil {
		return nil, err
	}

	// The slot is taken before anything is charged, so two players racing for
	// it still resolve in storage rather than at the gateway.
	held, err := s.bookings.HoldSlot(ctx, in, court.SlotPrice(), now.Add(entity.PaymentHold))
	if err != nil {
		return nil, err
	}

	payment, err := s.payments.CreatePayment(ctx, entity.PaymentRequest{
		BookingID: held.ID,
		Amount:    court.SlotPrice(),
		Currency:  court.Currency,
	})
	if err != nil {
		// Nothing can pay for this hold now, so it goes back rather than
		// sitting on the slot until it lapses.
		if cancelErr := s.bookings.CancelBooking(ctx, held.ID); cancelErr != nil {
			return nil, errors.Join(err, cancelErr)
		}

		return nil, err
	}

	if err := s.bookings.AttachPayment(ctx, held.ID, payment.IntentID); err != nil {
		return nil, err
	}

	held.PaymentIntentID = payment.IntentID

	return &entity.HeldBooking{Booking: *held, ClientSecret: payment.ClientSecret}, nil
}

// Availability reports the court's windows on one calendar date and whether
// each is still free. It is a snapshot, not a hold: Book settles the race.
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

	last := starts[len(starts)-1]
	taken, err := s.bookings.ListBookedSlots(ctx, courtID, starts[0], last.Add(entity.SlotDuration))
	if err != nil {
		return nil, err
	}

	// Keyed by instant, so an offset written elsewhere still matches.
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

// ListForPlayer returns the player's bookings from now on.
func (s *Booking) ListForPlayer(ctx context.Context, playerID uuid.UUID) ([]entity.PlayerBooking, error) {
	if playerID == uuid.Nil {
		return nil, entity.ErrPlayerRequired
	}

	return s.bookings.ListPlayerBookings(ctx, playerID, time.Now())
}
