package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/thelong0705/deuce/internal/domain/entity"
	"github.com/thelong0705/deuce/internal/domain/usecase"
)

var (
	_ usecase.BookingRepo = (*BookingRepository)(nil)
	_ usecase.CourtFinder = (*BookingRepository)(nil)
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

func toEntityBooking(b Booking) *entity.Booking {
	out := &entity.Booking{
		ID:        b.ID,
		CourtID:   b.CourtID,
		IsBlock:   b.IsBlock,
		StartsAt:  b.StartsAt.Time,
		CreatedAt: b.CreatedAt.Time,
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
