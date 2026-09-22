package postgres

import (
	"context"
	"fmt"

	"github.com/thelong0705/deuce/internal/domain/entity"
	"github.com/thelong0705/deuce/internal/domain/usecase"
)

var _ usecase.CityRepo = (*CityRepository)(nil)

type CityRepository struct {
	q *Queries
}

func NewCityRepository(q *Queries) *CityRepository {
	return &CityRepository{q: q}
}

func (r *CityRepository) ListCities(ctx context.Context) ([]entity.City, error) {
	rows, err := r.q.ListCities(ctx)
	if err != nil {
		return nil, fmt.Errorf("list cities: %w", err)
	}

	cities := make([]entity.City, 0, len(rows))
	for _, row := range rows {
		cities = append(cities, entity.City{Name: row.Name, Timezone: row.Timezone})
	}

	return cities, nil
}
