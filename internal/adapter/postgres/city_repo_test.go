package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

func TestCityRepositoryListCities(t *testing.T) {
	repo := NewCityRepository(testQueries)

	cities, err := repo.ListCities(context.Background())
	require.NoError(t, err)

	names := make([]string, 0, len(cities))
	for _, c := range cities {
		names = append(names, c.Name)
	}
	require.ElementsMatch(t, seededCities, names)
}

func TestVenueRepositoryCreateVenueRejectsAnUnsupportedCity(t *testing.T) {
	repo := NewVenueRepository(testQueries)

	_, err := repo.CreateVenue(context.Background(), entity.CreateVenueInput{
		OwnerID: createRandomOwner(t),
		Name:    "Nowhere Tennis Club",
		City:    "Atlantis",
		Address: "1 Test Street",
	})

	require.ErrorIs(t, err, entity.ErrVenueCityUnsupported)
}
