package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/thelong0705/deuce/internal/domain/entity"
	"github.com/thelong0705/deuce/internal/domain/usecase"
)

var (
	_ usecase.BookingRepo      = (*BookingRepository)(nil)
	_ usecase.BookedSlotFinder = (*BookingRepository)(nil)
	_ usecase.CourtFinder      = (*BookingRepository)(nil)
)

// BookingRepository persists bookings in Postgres.
type BookingRepository struct {
	q *Queries
}

func NewBookingRepository(q *Queries) *BookingRepository {
	return &BookingRepository{q: q}
}

// CreateBooking writes the booking without first checking the slot is free.
// The partial unique index on (court_id, starts_at) decides that, so two
// players racing for one slot cannot both succeed: the loser's insert violates
// the index and comes back as ErrSlotTaken.
func (r *BookingRepository) CreateBooking(ctx context.Context, in entity.BookSlotInput) (*entity.Booking, error) {
	row, err := r.q.CreateBooking(ctx, CreateBookingParams{
		CourtID:  in.CourtID,
		PlayerID: uuid.NullUUID{UUID: in.PlayerID, Valid: in.PlayerID != uuid.Nil},
		StartsAt: pgtype.Timestamptz{Time: in.StartsAt, Valid: true},
		Status:   BookingStatusConfirmed,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, entity.ErrSlotTaken
		}
		return nil, fmt.Errorf("create booking: %w", err)
	}

	return toEntityBooking(row), nil
}

func (r *BookingRepository) GetCourtWithVenue(ctx context.Context, id uuid.UUID) (*entity.Court, *entity.Venue, error) {
	row, err := r.q.GetCourtVenue(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, entity.ErrCourtNotFound
		}
		return nil, nil, fmt.Errorf("get court venue: %w", err)
	}

	return toEntityCourt(row.Court), toEntityVenue(row.Venue), nil
}

func (r *BookingRepository) ListBookedSlots(
	ctx context.Context, courtID uuid.UUID, from, to time.Time,
) ([]time.Time, error) {
	rows, err := r.q.ListBookedSlots(ctx, ListBookedSlotsParams{
		CourtID:    courtID,
		StartsAt:   pgtype.Timestamptz{Time: from, Valid: true},
		StartsAt_2: pgtype.Timestamptz{Time: to, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("list booked slots: %w", err)
	}

	slots := make([]time.Time, 0, len(rows))
	for _, row := range rows {
		slots = append(slots, row.Time)
	}

	return slots, nil
}

func (r *BookingRepository) ListPlayerBookings(
	ctx context.Context, playerID uuid.UUID, from time.Time,
) ([]entity.PlayerBooking, error) {
	rows, err := r.q.ListPlayerBookings(ctx, ListPlayerBookingsParams{
		PlayerID: uuid.NullUUID{UUID: playerID, Valid: playerID != uuid.Nil},
		StartsAt: pgtype.Timestamptz{Time: from, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("list player bookings: %w", err)
	}

	bookings := make([]entity.PlayerBooking, 0, len(rows))
	for _, row := range rows {
		bookings = append(bookings, entity.PlayerBooking{
			Booking: *toEntityBooking(row.Booking),
			Court:   *toEntityCourt(row.Court),
			Venue:   *toEntityVenue(row.Venue),
		})
	}

	return bookings, nil
}

func toEntityBooking(b Booking) *entity.Booking {
	out := &entity.Booking{
		ID:        b.ID,
		CourtID:   b.CourtID,
		IsBlock:   b.IsBlock,
		StartsAt:  b.StartsAt.Time,
		Status:    entity.BookingStatus(b.Status),
		CreatedAt: b.CreatedAt.Time,
	}

	if b.Amount.Valid {
		amount := int(b.Amount.Int32)
		out.Amount = &amount
	}

	if b.HoldExpiresAt.Valid {
		holdExpiresAt := b.HoldExpiresAt.Time
		out.HoldExpiresAt = &holdExpiresAt
	}

	if b.PaymentIntentID.Valid {
		out.PaymentIntentID = b.PaymentIntentID.String
	}

	if b.PlayerID.Valid {
		out.PlayerID = b.PlayerID.UUID
	}

	if b.CancelledAt.Valid {
		cancelledAt := b.CancelledAt.Time
		out.CancelledAt = &cancelledAt
	}

	return out
}

func (r *BookingRepository) ListBookedSlotsForCourts(ctx context.Context, courtIDs []uuid.UUID, from, to time.Time) (map[uuid.UUID][]time.Time, error) {
	rows, err := r.q.ListBookedSlotsForCourts(ctx, ListBookedSlotsForCourtsParams{
		CourtIds: courtIDs,
		FromTime: pgtype.Timestamptz{Time: from, Valid: true},
		ToTime:   pgtype.Timestamptz{Time: to, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("list booked slots for courts: %w", err)
	}

	byCourt := make(map[uuid.UUID][]time.Time, len(courtIDs))
	for _, row := range rows {
		byCourt[row.CourtID] = append(byCourt[row.CourtID], row.StartsAt.Time)
	}

	return byCourt, nil
}
