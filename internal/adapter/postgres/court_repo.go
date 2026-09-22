package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/thelong0705/deuce/internal/domain/entity"
	"github.com/thelong0705/deuce/internal/domain/usecase"
)

var _ usecase.CourtRepo = (*CourtRepository)(nil)

// CourtRepository persists courts in Postgres.
type CourtRepository struct {
	q *Queries
}

func NewCourtRepository(q *Queries) *CourtRepository {
	return &CourtRepository{q: q}
}

func (r *CourtRepository) CreateCourt(ctx context.Context, in entity.CreateCourtInput) (*entity.Court, error) {
	currency, err := toCurrency(in.Currency)
	if err != nil {
		return nil, err
	}

	row, err := r.q.CreateCourt(ctx, CreateCourtParams{
		VenueID:      in.VenueID,
		Name:         in.Name,
		OpenHour:     int16(in.OpenHour),
		CloseHour:    int16(in.CloseHour),
		PricePerHour: int32(in.PricePerHour),
		Currency:     currency,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, entity.ErrCourtNameTaken
		}
		return nil, fmt.Errorf("create court: %w", err)
	}

	return toEntityCourt(row), nil
}

func (r *CourtRepository) ListCourtsByVenue(ctx context.Context, venueID uuid.UUID) ([]entity.Court, error) {
	rows, err := r.q.ListCourtsByVenue(ctx, venueID)
	if err != nil {
		return nil, fmt.Errorf("list courts by venue: %w", err)
	}

	courts := make([]entity.Court, 0, len(rows))
	for _, row := range rows {
		courts = append(courts, *toEntityCourt(row))
	}

	return courts, nil
}

func toCurrency(c entity.Currency) (Currency, error) {
	switch c {
	case entity.CurrencyVND:
		return CurrencyVND, nil
	default:
		return "", fmt.Errorf("unknown currency %q", c)
	}
}

func toEntityCourt(c Court) *entity.Court {
	return &entity.Court{
		ID:           c.ID,
		VenueID:      c.VenueID,
		Name:         c.Name,
		OpenHour:     int(c.OpenHour),
		CloseHour:    int(c.CloseHour),
		PricePerHour: int(c.PricePerHour),
		Currency:     entity.Currency(c.Currency),
		IsActive:     c.IsActive,
		CreatedAt:    c.CreatedAt.Time,
	}
}

func (r *CourtRepository) SearchCourts(ctx context.Context, city string) ([]entity.CourtAtVenue, error) {
	rows, err := r.q.SearchCourts(ctx, city)
	if err != nil {
		return nil, fmt.Errorf("search courts: %w", err)
	}

	courts := make([]entity.CourtAtVenue, 0, len(rows))
	for _, row := range rows {
		courts = append(courts, entity.CourtAtVenue{
			Court: *toEntityCourt(row.Court),
			Venue: *toEntityVenue(row.Venue),
		})
	}

	return courts, nil
}
