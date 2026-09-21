package postgres

import (
	"context"
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
