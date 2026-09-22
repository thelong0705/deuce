package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/domain/entity"
	"github.com/thelong0705/deuce/internal/domain/usecase"
	"github.com/thelong0705/deuce/internal/domain/usecase/mocks"
)

func TestCityList(t *testing.T) {
	boom := errors.New("boom")

	tests := []struct {
		name    string
		setup   func(cities *mocks.MockCityRepo)
		want    []entity.City
		wantErr error
	}{
		{
			name: "returns what the repository has",
			setup: func(cities *mocks.MockCityRepo) {
				cities.EXPECT().ListCities(mock.Anything).
					Return([]entity.City{{Name: "Ha Noi"}, {Name: "Ho Chi Minh City"}}, nil).Once()
			},
			want: []entity.City{{Name: "Ha Noi"}, {Name: "Ho Chi Minh City"}},
		},
		{
			name: "an empty list is not an error",
			setup: func(cities *mocks.MockCityRepo) {
				cities.EXPECT().ListCities(mock.Anything).Return([]entity.City{}, nil).Once()
			},
			want: []entity.City{},
		},
		{
			name: "propagates a lookup failure",
			setup: func(cities *mocks.MockCityRepo) {
				cities.EXPECT().ListCities(mock.Anything).Return(nil, boom).Once()
			},
			wantErr: boom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cities := mocks.NewMockCityRepo(t)
			tt.setup(cities)

			got, err := usecase.NewCity(cities).List(context.Background())

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
