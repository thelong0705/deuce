package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/thelong0705/deuce/internal/domain/entity"
	"github.com/thelong0705/deuce/internal/domain/usecase"
)

var (
	_ usecase.VenueRepo   = (*VenueRepository)(nil)
	_ usecase.VenueFinder = (*VenueRepository)(nil)
)

// VenueRepository persists venues in Postgres.
type VenueRepository struct {
	q *Queries
}

func NewVenueRepository(q *Queries) *VenueRepository {
	return &VenueRepository{q: q}
}

func (r *VenueRepository) CreateVenue(ctx context.Context, in entity.CreateVenueInput) (*entity.Venue, error) {
	row, err := r.q.CreateVenue(ctx, CreateVenueParams{
		OwnerID:  in.OwnerID,
		Name:     in.Name,
		City:     in.City,
		Address:  in.Address,
		Timezone: in.Timezone,
	})
	if err != nil {
		return nil, fmt.Errorf("create venue: %w", err)
	}

	return toEntityVenue(row), nil
}

func toEntityVenue(v Venue) *entity.Venue {
	return &entity.Venue{
		ID:        v.ID,
		OwnerID:   v.OwnerID,
		Name:      v.Name,
		City:      v.City,
		Address:   v.Address,
		Timezone:  v.Timezone,
		IsActive:  v.IsActive,
		CreatedAt: v.CreatedAt.Time,
	}
}

func (r *VenueRepository) GetVenue(ctx context.Context, id uuid.UUID) (*entity.Venue, error) {
	row, err := r.q.GetVenue(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, entity.ErrVenueNotFound
		}
		return nil, fmt.Errorf("get venue: %w", err)
	}

	return toEntityVenue(row), nil
}

func (r *VenueRepository) ListVenuesByOwner(ctx context.Context, ownerID uuid.UUID) ([]entity.Venue, error) {
	rows, err := r.q.ListVenuesByOwner(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list venues by owner: %w", err)
	}

	return toEntityVenues(rows), nil
}

func (r *VenueRepository) SearchVenuesByCity(ctx context.Context, city string) ([]entity.Venue, error) {
	rows, err := r.q.SearchVenuesByCity(ctx, city)
	if err != nil {
		return nil, fmt.Errorf("search venues by city: %w", err)
	}

	return toEntityVenues(rows), nil
}

func toEntityVenues(rows []Venue) []entity.Venue {
	venues := make([]entity.Venue, 0, len(rows))
	for _, row := range rows {
		venues = append(venues, *toEntityVenue(row))
	}

	return venues
}
