package entity_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

func validCourtInput() entity.CreateCourtInput {
	return entity.CreateCourtInput{
		OwnerID:      uuid.New(),
		VenueID:      uuid.New(),
		Name:         "Court 1",
		OpenHour:     6,
		CloseHour:    22,
		PricePerHour: 120000,
	}
}

func TestCreateCourtInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(in *entity.CreateCourtInput)
		wantErr error
	}{
		{
			name:   "valid court",
			mutate: func(*entity.CreateCourtInput) {},
		},
		{
			name:   "a free court is allowed",
			mutate: func(in *entity.CreateCourtInput) { in.PricePerHour = 0 },
		},
		{
			name:   "open all day",
			mutate: func(in *entity.CreateCourtInput) { in.OpenHour, in.CloseHour = 0, 24 },
		},
		{
			name:    "missing owner",
			mutate:  func(in *entity.CreateCourtInput) { in.OwnerID = uuid.Nil },
			wantErr: entity.ErrOwnerRequired,
		},
		{
			name:    "missing venue",
			mutate:  func(in *entity.CreateCourtInput) { in.VenueID = uuid.Nil },
			wantErr: entity.ErrVenueRequired,
		},
		{
			name:    "empty name",
			mutate:  func(in *entity.CreateCourtInput) { in.Name = "" },
			wantErr: entity.ErrCourtNameRequired,
		},
		{
			name:    "name of only whitespace",
			mutate:  func(in *entity.CreateCourtInput) { in.Name = "  \t" },
			wantErr: entity.ErrCourtNameRequired,
		},
		{
			name:    "closing before opening",
			mutate:  func(in *entity.CreateCourtInput) { in.OpenHour, in.CloseHour = 20, 8 },
			wantErr: entity.ErrCourtHoursInvalid,
		},
		{
			name:    "closing at the opening hour",
			mutate:  func(in *entity.CreateCourtInput) { in.OpenHour, in.CloseHour = 9, 9 },
			wantErr: entity.ErrCourtHoursInvalid,
		},
		{
			name:    "opening before midnight",
			mutate:  func(in *entity.CreateCourtInput) { in.OpenHour = -1 },
			wantErr: entity.ErrCourtHoursInvalid,
		},
		{
			name:    "closing after midnight",
			mutate:  func(in *entity.CreateCourtInput) { in.CloseHour = 25 },
			wantErr: entity.ErrCourtHoursInvalid,
		},
		{
			name:    "negative price",
			mutate:  func(in *entity.CreateCourtInput) { in.PricePerHour = -1 },
			wantErr: entity.ErrCourtPriceNegative,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validCourtInput()
			tt.mutate(&in)

			err := in.Validate()

			if tt.wantErr == nil {
				require.NoError(t, err)
				return
			}

			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}
