package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestCreateVenue(t *testing.T) {
	tests := []struct {
		name    string
		arg     func(t *testing.T) CreateVenueParams
		wantErr bool
	}{
		{
			name: "valid venue",
			arg: func(t *testing.T) CreateVenueParams {
				return CreateVenueParams{
					OwnerID:  createRandomOwner(t),
					Name:     gofakeit.Company() + " Tennis Club",
					City:     gofakeit.City(),
					Address:  gofakeit.Street(),
					Timezone: "Asia/Ho_Chi_Minh",
				}
			},
		},
		{
			name: "unknown owner",
			arg: func(t *testing.T) CreateVenueParams {
				return CreateVenueParams{
					OwnerID:  uuid.New(),
					Name:     gofakeit.Company() + " Tennis Club",
					City:     gofakeit.City(),
					Address:  gofakeit.Street(),
					Timezone: "Asia/Ho_Chi_Minh",
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arg := tt.arg(t)

			venue, err := testQueries.CreateVenue(context.Background(), arg)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, arg.OwnerID, venue.OwnerID)
			require.Equal(t, arg.Name, venue.Name)
			require.Equal(t, arg.City, venue.City)
			require.Equal(t, arg.Address, venue.Address)

			require.NotEqual(t, uuid.Nil, venue.ID)
			require.True(t, venue.IsActive)
			require.True(t, venue.CreatedAt.Valid)
		})
	}
}

func TestGetVenue(t *testing.T) {
	tests := []struct {
		name    string
		venue   func(t *testing.T) Venue
		wantErr error
	}{
		{
			name: "existing venue",
			venue: func(t *testing.T) Venue {
				return createRandomVenue(t)
			},
		},
		{
			name: "unknown id",
			venue: func(t *testing.T) Venue {
				return Venue{ID: uuid.New()}
			},
			wantErr: pgx.ErrNoRows,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := tt.venue(t)

			got, err := testQueries.GetVenue(context.Background(), want.ID)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, want.ID, got.ID)
			require.Equal(t, want.OwnerID, got.OwnerID)
			require.Equal(t, want.Name, got.Name)
			require.Equal(t, want.City, got.City)
			require.Equal(t, want.Address, got.Address)
		})
	}
}

func TestListVenues(t *testing.T) {
	created := createRandomVenue(t)

	venues, err := testQueries.ListVenues(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, venues)

	found := false
	for _, v := range venues {
		if v.ID == created.ID {
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestUpdateVenue(t *testing.T) {
	tests := []struct {
		name    string
		newName string
		newCity string
		newAddr string
	}{
		{"rename only", "Renamed Club", "Hanoi", "12 Le Loi"},
		{"move city", "Ace Tennis Club", "Da Nang", "12 Le Loi"},
		{"change everything", "Deuce Club", "Ho Chi Minh", "99 New Road"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created := createRandomVenue(t)

			updated, err := testQueries.UpdateVenue(context.Background(), UpdateVenueParams{
				ID:      created.ID,
				Name:    tt.newName,
				City:    tt.newCity,
				Address: tt.newAddr,
			})
			require.NoError(t, err)

			require.Equal(t, created.ID, updated.ID)
			require.Equal(t, tt.newName, updated.Name)
			require.Equal(t, tt.newCity, updated.City)
			require.Equal(t, tt.newAddr, updated.Address)
		})
	}
}

func TestDeactivateVenue(t *testing.T) {
	tests := []struct {
		name    string
		venueID func(t *testing.T) uuid.UUID
		wantErr error
	}{
		{
			name: "active venue",
			venueID: func(t *testing.T) uuid.UUID {
				return createRandomVenue(t).ID
			},
		},
		{
			name: "already deactivated",
			venueID: func(t *testing.T) uuid.UUID {
				v := createRandomVenue(t)
				_, err := testQueries.DeactivateVenue(context.Background(), v.ID)
				require.NoError(t, err)
				return v.ID
			},
		},
		{
			name: "unknown id",
			venueID: func(t *testing.T) uuid.UUID {
				return uuid.New()
			},
			wantErr: pgx.ErrNoRows,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := tt.venueID(t)

			deactivated, err := testQueries.DeactivateVenue(context.Background(), id)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, id, deactivated.ID)
			require.False(t, deactivated.IsActive)
		})
	}
}

func randomEmail() string {
	return fmt.Sprintf("%s-%s@example.com", gofakeit.Username(), uuid.NewString())
}

// venues.owner_id is a foreign key, so every venue needs a user to own it.
func createRandomOwner(t *testing.T) uuid.UUID {
	t.Helper()
	return createRandomUser(t, UserRoleOwner).ID
}

func createRandomVenue(t *testing.T) Venue {
	t.Helper()

	venue, err := testQueries.CreateVenue(context.Background(), CreateVenueParams{
		OwnerID:  createRandomOwner(t),
		Name:     gofakeit.Company() + " Tennis Club",
		City:     gofakeit.City(),
		Address:  gofakeit.Street(),
		Timezone: "Asia/Ho_Chi_Minh",
	})
	require.NoError(t, err)

	return venue
}
