package entity_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

func validVenueInput() entity.CreateVenueInput {
	return entity.CreateVenueInput{
		OwnerID: uuid.New(),
		Name:    "Ace Tennis Club",
		City:    "Hanoi",
		Address: "12 Le Loi",
	}
}

func TestCreateVenueInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(in *entity.CreateVenueInput)
		wantErr error
	}{
		{
			name:   "valid venue",
			mutate: func(*entity.CreateVenueInput) {},
		},
		{
			name:    "missing owner",
			mutate:  func(in *entity.CreateVenueInput) { in.OwnerID = uuid.Nil },
			wantErr: entity.ErrOwnerRequired,
		},
		{
			name:    "empty name",
			mutate:  func(in *entity.CreateVenueInput) { in.Name = "" },
			wantErr: entity.ErrVenueNameRequired,
		},
		{
			name:    "name of only whitespace",
			mutate:  func(in *entity.CreateVenueInput) { in.Name = "   " },
			wantErr: entity.ErrVenueNameRequired,
		},
		{
			name:    "empty city",
			mutate:  func(in *entity.CreateVenueInput) { in.City = "" },
			wantErr: entity.ErrVenueCityRequired,
		},
		{
			name:    "city of only whitespace",
			mutate:  func(in *entity.CreateVenueInput) { in.City = "\t\n" },
			wantErr: entity.ErrVenueCityRequired,
		},
		{
			name:    "empty address",
			mutate:  func(in *entity.CreateVenueInput) { in.Address = "" },
			wantErr: entity.ErrVenueAddressRequired,
		},
		{
			name:    "address of only whitespace",
			mutate:  func(in *entity.CreateVenueInput) { in.Address = " " },
			wantErr: entity.ErrVenueAddressRequired,
		},
		{
			name:   "non-ascii name is allowed",
			mutate: func(in *entity.CreateVenueInput) { in.Name = "Sân Tennis Hà Nội" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validVenueInput()
			tt.mutate(&in)

			err := in.Validate()

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
