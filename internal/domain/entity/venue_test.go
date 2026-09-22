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
		City:    "Ha Noi",
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

func TestVenueLocation(t *testing.T) {
	tests := []struct {
		name     string
		timezone string
		wantName string
	}{
		{
			name:     "an IANA name resolves",
			timezone: "Asia/Ho_Chi_Minh",
			wantName: "Asia/Ho_Chi_Minh",
		},
		{
			// Validation keeps these out, but a row written before the column
			// existed carries the default.
			name:     "an unknown name falls back to UTC",
			timezone: "Mars/Olympus_Mons",
			wantName: "UTC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := entity.Venue{Timezone: tt.timezone}.Location()

			require.Equal(t, tt.wantName, got.String())
		})
	}
}
