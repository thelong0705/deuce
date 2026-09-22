package entity_test

import (
	"testing"
	"time"

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
		Currency:     entity.CurrencyVND,
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

func validCourtSearch() entity.CourtSearch {
	return entity.CourtSearch{
		City:     "Hanoi",
		Date:     time.Now().AddDate(0, 0, 1),
		FromHour: 18,
		ToHour:   21,
	}
}

func TestCourtSearchValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(in *entity.CourtSearch)
		wantErr error
	}{
		{
			name:   "valid search",
			mutate: func(*entity.CourtSearch) {},
		},
		{
			name:   "the whole day",
			mutate: func(in *entity.CourtSearch) { in.FromHour, in.ToHour = 0, 24 },
		},
		{
			name:    "missing city",
			mutate:  func(in *entity.CourtSearch) { in.City = "  " },
			wantErr: entity.ErrSearchCityRequired,
		},
		{
			name:    "missing date",
			mutate:  func(in *entity.CourtSearch) { in.Date = time.Time{} },
			wantErr: entity.ErrSearchDateRequired,
		},
		{
			name:    "hours the wrong way round",
			mutate:  func(in *entity.CourtSearch) { in.FromHour, in.ToHour = 21, 18 },
			wantErr: entity.ErrSearchHoursInvalid,
		},
		{
			name:    "an empty window",
			mutate:  func(in *entity.CourtSearch) { in.FromHour, in.ToHour = 18, 18 },
			wantErr: entity.ErrSearchHoursInvalid,
		},
		{
			name:    "before midnight",
			mutate:  func(in *entity.CourtSearch) { in.FromHour = -1 },
			wantErr: entity.ErrSearchHoursInvalid,
		},
		{
			name:    "past midnight",
			mutate:  func(in *entity.CourtSearch) { in.ToHour = 25 },
			wantErr: entity.ErrSearchHoursInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validCourtSearch()
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

func TestCourtSlotsWithin(t *testing.T) {
	hcm, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	require.NoError(t, err)

	court := entity.Court{OpenHour: 6, CloseHour: 22, IsActive: true}

	day := time.Date(2026, 9, 23, 0, 0, 0, 0, hcm)
	now := time.Date(2026, 9, 22, 8, 0, 0, 0, hcm)

	tests := []struct {
		name      string
		fromHour  int
		toHour    int
		wantHours []int
	}{
		{
			// Slots sit on a two-hour grid from the opening hour, so 19:00 is
			// not one of them.
			name:      "an evening window",
			fromHour:  18,
			toHour:    21,
			wantHours: []int{18, 20},
		},
		{
			// A slot starting at toHour would run past the hour asked for.
			name:      "the window excludes its own end",
			fromHour:  18,
			toHour:    20,
			wantHours: []int{18},
		},
		{
			name:      "the whole day is the court's own hours",
			fromHour:  0,
			toHour:    24,
			wantHours: []int{6, 8, 10, 12, 14, 16, 18, 20},
		},
		{
			name:      "a window the court is closed for",
			fromHour:  2,
			toHour:    5,
			wantHours: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := court.SlotsWithin(day, hcm, now, tt.fromHour, tt.toHour)

			hours := make([]int, 0, len(got))
			for _, at := range got {
				hours = append(hours, at.In(hcm).Hour())
			}

			require.Equal(t, tt.wantHours, hours)
		})
	}
}

func TestCourtCurrency(t *testing.T) {
	tests := []struct {
		name      string
		currency  entity.Currency
		wantValid bool
	}{
		{name: "VND", currency: entity.CurrencyVND, wantValid: true},
		{name: "unset", currency: ""},
		{name: "a currency deuce does not take yet", currency: "USD"},
		{name: "the right currency in the wrong case", currency: "vnd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.wantValid, tt.currency.Valid())

			in := validCourtInput()
			in.Currency = tt.currency

			if tt.wantValid {
				require.NoError(t, in.Validate())
				return
			}
			require.ErrorIs(t, in.Validate(), entity.ErrCurrencyInvalid)
		})
	}
}

// Whatever a form offers has to be something Validate accepts, or the dropdown
// and the rule disagree.
func TestSupportedCurrenciesAreAllValid(t *testing.T) {
	supported := entity.SupportedCurrencies()
	require.NotEmpty(t, supported)

	for _, currency := range supported {
		require.True(t, currency.Valid(), "%s is offered but not accepted", currency)
	}
}
