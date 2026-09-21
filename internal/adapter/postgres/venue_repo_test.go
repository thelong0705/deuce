package postgres

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

func validVenueInput(t *testing.T) entity.CreateVenueInput {
	t.Helper()

	return entity.CreateVenueInput{
		OwnerID: createRandomOwner(t),
		Name:    gofakeit.Company() + " Tennis Club",
		City:    gofakeit.City(),
		Address: gofakeit.Street(),
	}
}

func TestVenueRepositoryCreateVenue(t *testing.T) {
	repo := NewVenueRepository(testQueries)

	tests := []struct {
		name    string
		in      func(t *testing.T) entity.CreateVenueInput
		wantErr bool
		check   func(t *testing.T, in entity.CreateVenueInput, got *entity.Venue)
	}{
		{
			name: "creates a venue",
			in:   validVenueInput,
			check: func(t *testing.T, in entity.CreateVenueInput, got *entity.Venue) {
				require.Equal(t, in.OwnerID, got.OwnerID)
				require.Equal(t, in.Name, got.Name)
				require.Equal(t, in.City, got.City)
				require.Equal(t, in.Address, got.Address)
			},
		},
		{
			name: "storage assigns id, timestamp and active state",
			in:   validVenueInput,
			check: func(t *testing.T, _ entity.CreateVenueInput, got *entity.Venue) {
				require.NotEqual(t, uuid.Nil, got.ID)
				require.False(t, got.CreatedAt.IsZero())
				require.True(t, got.IsActive)
			},
		},
		{
			name: "an unknown owner is rejected by the foreign key",
			in: func(t *testing.T) entity.CreateVenueInput {
				in := validVenueInput(t)
				in.OwnerID = uuid.New()
				return in
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := tt.in(t)

			got, err := repo.CreateVenue(context.Background(), in)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			tt.check(t, in, got)
		})
	}
}

func TestUserRepositoryGetUser(t *testing.T) {
	repo := NewUserRepository(testQueries)

	tests := []struct {
		name    string
		userID  func(t *testing.T) uuid.UUID
		wantErr error
	}{
		{
			name:   "returns an existing user",
			userID: createRandomOwner,
		},
		{
			name:    "an unknown id becomes ErrUserNotFound",
			userID:  func(*testing.T) uuid.UUID { return uuid.New() },
			wantErr: entity.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := tt.userID(t)

			got, err := repo.GetUser(context.Background(), id)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, id, got.ID)
			require.Equal(t, entity.RoleOwner, got.Role)
			require.True(t, got.IsActive)
		})
	}
}
