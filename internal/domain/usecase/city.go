package usecase

import (
	"context"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

type CityRepo interface {
	ListCities(ctx context.Context) ([]entity.City, error)
}

type City struct {
	cityRepo CityRepo
}

func NewCity(cityRepo CityRepo) *City {
	return &City{cityRepo: cityRepo}
}

func (s *City) List(ctx context.Context) ([]entity.City, error) {
	return s.cityRepo.ListCities(ctx)
}
