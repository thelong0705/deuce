package postgres

import (
	"context"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

// seededCities are the rows migration 000006 inserts. The foreign key rejects
// anything else, so fixtures pick from here.
var seededCities = []string{"Ha Noi", "Ho Chi Minh City"}

func supportedCity() string {
	return seededCities[rand.Intn(len(seededCities))]
}

func TestListCities(t *testing.T) {
	cities, err := testQueries.ListCities(context.Background())
	require.NoError(t, err)

	names := make([]string, 0, len(cities))
	for _, c := range cities {
		names = append(names, c.Name)
		require.NotEmpty(t, c.Timezone, "a city decides its venues' timezone, so it must have one")
	}

	require.ElementsMatch(t, seededCities, names)
	require.IsNonDecreasing(t, names)
}

func TestVenueCityIsStoredCanonically(t *testing.T) {
	// The insert selects cities.name rather than echoing the input back.
	venue, err := testQueries.CreateVenue(context.Background(), CreateVenueParams{
		OwnerID: createRandomOwner(t),
		Name:    "Lowercase City Club",
		City:    "ha noi",
		Address: "1 Test Street",
	})
	require.NoError(t, err)
	require.Equal(t, "Ha Noi", venue.City)

	require.Equal(t, "Asia/Ho_Chi_Minh", venue.Timezone)

	found, err := testQueries.SearchVenuesByCity(context.Background(), "Ha Noi")
	require.NoError(t, err)

	var ids []string
	for _, v := range found {
		ids = append(ids, v.ID.String())
	}
	require.Contains(t, ids, venue.ID.String())
}

func TestCreateVenueRejectsAnUnsupportedCity(t *testing.T) {
	_, err := testQueries.CreateVenue(context.Background(), CreateVenueParams{
		OwnerID: createRandomOwner(t),
		Name:    "Nowhere Tennis Club",
		City:    "Atlantis",
		Address: "1 Test Street",
	})
	require.Error(t, err)
}
