package postgres

import (
	"context"
	"slices"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

func validCourtInput(t *testing.T) entity.CreateCourtInput {
	t.Helper()

	venue := createRandomVenue(t)

	return entity.CreateCourtInput{
		OwnerID:      venue.OwnerID,
		VenueID:      venue.ID,
		Name:         gofakeit.Noun() + " " + gofakeit.LetterN(4),
		OpenHour:     6,
		CloseHour:    22,
		PricePerHour: 120000,
	}
}

func TestCourtRepositoryCreateCourt(t *testing.T) {
	repo := NewCourtRepository(testQueries)

	tests := []struct {
		name    string
		in      func(t *testing.T) entity.CreateCourtInput
		wantErr error
		// wantAnyErr covers failures Postgres reports but the domain has no
		// name for
		wantAnyErr bool
		check      func(t *testing.T, in entity.CreateCourtInput, got *entity.Court)
	}{
		{
			name: "creates a court",
			in:   validCourtInput,
			check: func(t *testing.T, in entity.CreateCourtInput, got *entity.Court) {
				require.Equal(t, in.VenueID, got.VenueID)
				require.Equal(t, in.Name, got.Name)
				require.Equal(t, in.OpenHour, got.OpenHour)
				require.Equal(t, in.CloseHour, got.CloseHour)
				require.Equal(t, in.PricePerHour, got.PricePerHour)
			},
		},
		{
			name: "storage assigns id, timestamp and active state",
			in:   validCourtInput,
			check: func(t *testing.T, _ entity.CreateCourtInput, got *entity.Court) {
				require.NotEqual(t, uuid.Nil, got.ID)
				require.False(t, got.CreatedAt.IsZero())
				require.True(t, got.IsActive)
			},
		},
		{
			name: "an unknown venue is rejected by the foreign key",
			in: func(t *testing.T) entity.CreateCourtInput {
				in := validCourtInput(t)
				in.VenueID = uuid.New()
				return in
			},
			wantAnyErr: true,
		},
		{
			name: "a name already used at the venue becomes ErrCourtNameTaken",
			in: func(t *testing.T) entity.CreateCourtInput {
				in := validCourtInput(t)

				_, err := repo.CreateCourt(context.Background(), in)
				require.NoError(t, err)

				return in
			},
			wantErr: entity.ErrCourtNameTaken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := tt.in(t)

			got, err := repo.CreateCourt(context.Background(), in)

			switch {
			case tt.wantErr != nil:
				require.ErrorIs(t, err, tt.wantErr)
				return
			case tt.wantAnyErr:
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			tt.check(t, in, got)
		})
	}
}

// The unique index is scoped to the venue, so the same court name at another
// venue is fine.
func TestCourtRepositoryCreateCourtAllowsTheSameNameAtAnotherVenue(t *testing.T) {
	repo := NewCourtRepository(testQueries)
	ctx := context.Background()

	first := validCourtInput(t)
	_, err := repo.CreateCourt(ctx, first)
	require.NoError(t, err)

	second := validCourtInput(t)
	second.Name = first.Name

	got, err := repo.CreateCourt(ctx, second)

	require.NoError(t, err)
	require.Equal(t, first.Name, got.Name)
	require.NotEqual(t, first.VenueID, got.VenueID)
}

func TestVenueRepositoryGetVenue(t *testing.T) {
	repo := NewVenueRepository(testQueries)

	tests := []struct {
		name    string
		venueID func(t *testing.T) uuid.UUID
		wantErr error
	}{
		{
			name:    "returns an existing venue",
			venueID: func(t *testing.T) uuid.UUID { return createRandomVenue(t).ID },
		},
		{
			name:    "an unknown id becomes ErrVenueNotFound",
			venueID: func(*testing.T) uuid.UUID { return uuid.New() },
			wantErr: entity.ErrVenueNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := tt.venueID(t)

			got, err := repo.GetVenue(context.Background(), id)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, id, got.ID)
			require.True(t, got.IsActive)
		})
	}
}

func TestCourtRepositoryListCourtsByVenue(t *testing.T) {
	repo := NewCourtRepository(testQueries)
	ctx := context.Background()

	venue := createRandomVenue(t)

	var want []string
	for _, name := range []string{"Zulu", "Alpha"} {
		court, err := testQueries.CreateCourt(ctx, CreateCourtParams{
			VenueID:      venue.ID,
			Name:         name + " " + gofakeit.LetterN(6),
			OpenHour:     6,
			CloseHour:    22,
			PricePerHour: 150,
		})
		require.NoError(t, err)
		want = append(want, court.Name)
	}
	slices.Sort(want)

	// Another venue's court, which must not leak into the results.
	_, err := testQueries.CreateCourt(ctx, CreateCourtParams{
		VenueID:      createRandomVenue(t).ID,
		Name:         "Elsewhere " + gofakeit.LetterN(6),
		OpenHour:     6,
		CloseHour:    22,
		PricePerHour: 150,
	})
	require.NoError(t, err)

	t.Run("the venue's courts, by name", func(t *testing.T) {
		got, err := repo.ListCourtsByVenue(ctx, venue.ID)
		require.NoError(t, err)

		names := make([]string, 0, len(got))
		for _, court := range got {
			require.Equal(t, venue.ID, court.VenueID)
			names = append(names, court.Name)
		}
		require.Equal(t, want, names)
	})

	t.Run("a venue with no courts is empty, not an error", func(t *testing.T) {
		got, err := repo.ListCourtsByVenue(ctx, createRandomVenue(t).ID)
		require.NoError(t, err)
		require.Empty(t, got)
	})
}

func TestCourtRepositorySearchCourts(t *testing.T) {
	repo := NewCourtRepository(testQueries)
	ctx := context.Background()

	city := supportedCity()

	venue, err := testQueries.CreateVenue(ctx, CreateVenueParams{
		OwnerID: createRandomOwner(t),
		Name:    gofakeit.Company() + " " + gofakeit.LetterN(6),
		City:    city,
		Address: gofakeit.Street(),
	})
	require.NoError(t, err)

	court, err := testQueries.CreateCourt(ctx, CreateCourtParams{
		VenueID:      venue.ID,
		Name:         gofakeit.Noun() + " " + gofakeit.LetterN(6),
		OpenHour:     6,
		CloseHour:    22,
		PricePerHour: 120000,
	})
	require.NoError(t, err)

	got, err := repo.SearchCourts(ctx, city)
	require.NoError(t, err)

	var found *entity.CourtAtVenue
	for i := range got {
		if got[i].Court.ID == court.ID {
			found = &got[i]
			break
		}
	}

	require.NotNil(t, found, "the court should be in its own city's results")
	require.Equal(t, venue.ID, found.Venue.ID)
	require.Equal(t, city, found.Venue.City)
	// The venue's zone comes back with it, which is what reading the court's
	// opening hours needs.
	require.NotEmpty(t, found.Venue.Timezone)
	require.Equal(t, 6, found.Court.OpenHour)
	require.Equal(t, 22, found.Court.CloseHour)
}

// A deactivated court, or one at a deactivated venue, is not somewhere anybody
// can book.
func TestCourtRepositorySearchCourtsSkipsWhatIsNotBookable(t *testing.T) {
	repo := NewCourtRepository(testQueries)
	ctx := context.Background()

	city := supportedCity()

	newVenue := func() Venue {
		v, err := testQueries.CreateVenue(ctx, CreateVenueParams{
			OwnerID: createRandomOwner(t),
			Name:    gofakeit.Company() + " " + gofakeit.LetterN(6),
			City:    city,
			Address: gofakeit.Street(),
		})
		require.NoError(t, err)
		return v
	}

	newCourt := func(venueID uuid.UUID) Court {
		c, err := testQueries.CreateCourt(ctx, CreateCourtParams{
			VenueID:      venueID,
			Name:         gofakeit.Noun() + gofakeit.LetterN(6),
			OpenHour:     6,
			CloseHour:    22,
			PricePerHour: 1,
		})
		require.NoError(t, err)
		return c
	}

	deadCourt := newCourt(newVenue().ID)
	// There is no query for this: an owner deactivating a court is not a
	// feature yet, but the search already has to allow for it.
	_, err := testPool.Exec(ctx, "UPDATE courts SET is_active = false WHERE id = $1", deadCourt.ID)
	require.NoError(t, err)

	deadVenue := newVenue()
	courtAtDeadVenue := newCourt(deadVenue.ID)
	_, err = testQueries.DeactivateVenue(ctx, deadVenue.ID)
	require.NoError(t, err)

	got, err := repo.SearchCourts(ctx, city)
	require.NoError(t, err)

	for _, cv := range got {
		require.NotEqual(t, deadCourt.ID, cv.Court.ID, "a deactivated court must not be offered")
		require.NotEqual(t, courtAtDeadVenue.ID, cv.Court.ID, "a court at a deactivated venue must not be offered")
	}
}
