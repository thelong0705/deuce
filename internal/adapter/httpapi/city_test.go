package httpapi_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/adapter/httpapi/mocks"
	"github.com/thelong0705/deuce/internal/domain/entity"
)

func TestListCities(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(cities *mocks.MockCityUsecase)
		wantStatus int
		check      func(t *testing.T, body []byte)
	}{
		{
			name: "lists the cities",
			setup: func(cities *mocks.MockCityUsecase) {
				cities.EXPECT().List(mock.Anything).
					Return([]entity.City{{Name: "Ha Noi"}, {Name: "Ho Chi Minh City"}}, nil).Once()
			},
			wantStatus: http.StatusOK,
			check: func(t *testing.T, body []byte) {
				var got struct {
					Cities []struct {
						Name string `json:"name"`
					} `json:"cities"`
				}
				require.NoError(t, json.Unmarshal(body, &got))
				require.Len(t, got.Cities, 2)
				require.Equal(t, "Ha Noi", got.Cities[0].Name)
				require.Equal(t, "Ho Chi Minh City", got.Cities[1].Name)
			},
		},
		{
			name: "an empty list is an empty array, not null",
			setup: func(cities *mocks.MockCityUsecase) {
				cities.EXPECT().List(mock.Anything).Return([]entity.City{}, nil).Once()
			},
			wantStatus: http.StatusOK,
			check: func(t *testing.T, body []byte) {
				require.JSONEq(t, `{"cities":[]}`, string(body))
			},
		},
		{
			name: "a failure is 500 without leaking detail",
			setup: func(cities *mocks.MockCityUsecase) {
				cities.EXPECT().List(mock.Anything).
					Return(nil, errors.New("pq: connection to 10.0.0.5 refused")).Once()
			},
			wantStatus: http.StatusInternalServerError,
			check: func(t *testing.T, body []byte) {
				require.NotContains(t, string(body), "10.0.0.5")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cities := mocks.NewMockCityUsecase(t)
			tt.setup(cities)

			rec := doAuthed(t, deps{users: signedInOwner(t), cities: cities},
				http.MethodGet, "/cities", "")

			require.Equal(t, tt.wantStatus, rec.Code)
			tt.check(t, rec.Body.Bytes())
		})
	}
}

func TestListCitiesRequiresASession(t *testing.T) {
	users := mocks.NewMockUserUsecase(t)
	users.EXPECT().Authenticate(mock.Anything, "").
		Return(nil, entity.ErrSessionInvalid).Once()

	cities := mocks.NewMockCityUsecase(t)

	rec := do(t, deps{users: users, cities: cities}, http.MethodGet, "/cities", "")

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	cities.AssertNotCalled(t, "List")
}
