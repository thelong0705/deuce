package httpapi_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/adapter/httpapi/mocks"
	"github.com/thelong0705/deuce/internal/domain/entity"
)

var venueOwnerID = uuid.New()

func validVenueBody() string {
	return `{"owner_id":"` + venueOwnerID.String() + `","name":"Ace Tennis Club","city":"Hanoi","address":"12 Le Loi"}`
}

func createdVenue() *entity.Venue {
	return &entity.Venue{
		ID:        uuid.New(),
		OwnerID:   venueOwnerID,
		Name:      "Ace Tennis Club",
		City:      "Hanoi",
		Address:   "12 Le Loi",
		IsActive:  true,
		CreatedAt: time.Now(),
	}
}

func TestCreateVenue(t *testing.T) {
	tests := []struct {
		name string
		body string
		// setup configures the mock; nil means a successful creation
		setup func(venues *mocks.MockVenueUsecase)
		// wantNoCall asserts the use case was never reached
		wantNoCall bool
		wantStatus int
		wantErrMsg string
		check      func(t *testing.T, body []byte)
	}{
		{
			name:       "creates a venue",
			body:       validVenueBody(),
			wantStatus: http.StatusCreated,
			check: func(t *testing.T, body []byte) {
				var got map[string]any
				require.NoError(t, json.Unmarshal(body, &got))

				require.Equal(t, "Ace Tennis Club", got["name"])
				require.Equal(t, "Hanoi", got["city"])
				require.Equal(t, "12 Le Loi", got["address"])
				require.Equal(t, venueOwnerID.String(), got["owner_id"])
				require.Equal(t, true, got["is_active"])
				require.NotEmpty(t, got["id"])
			},
		},
		{
			name:       "rejects malformed json",
			body:       `{"name":`,
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid JSON body",
		},
		{
			name:       "rejects an unknown field",
			body:       `{"owner_id":"` + venueOwnerID.String() + `","name":"A","city":"B","address":"C","courts":9}`,
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid JSON body",
		},
		{
			name:       "rejects an owner_id that is not a uuid",
			body:       `{"owner_id":"not-a-uuid","name":"A","city":"B","address":"C"}`,
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "owner_id must be a valid uuid",
		},
		{
			name: "a validation error becomes 400",
			body: validVenueBody(),
			setup: func(venues *mocks.MockVenueUsecase) {
				venues.EXPECT().Create(mock.Anything, mock.Anything).
					Return(nil, entity.ErrVenueNameRequired).Once()
			},
			wantStatus: http.StatusBadRequest,
			wantErrMsg: entity.ErrVenueNameRequired.Error(),
		},
		{
			name: "an unknown owner becomes 404",
			body: validVenueBody(),
			setup: func(venues *mocks.MockVenueUsecase) {
				venues.EXPECT().Create(mock.Anything, mock.Anything).
					Return(nil, entity.ErrUserNotFound).Once()
			},
			wantStatus: http.StatusNotFound,
			wantErrMsg: entity.ErrUserNotFound.Error(),
		},
		{
			name: "a player becomes 403",
			body: validVenueBody(),
			setup: func(venues *mocks.MockVenueUsecase) {
				venues.EXPECT().Create(mock.Anything, mock.Anything).
					Return(nil, entity.ErrNotAnOwner).Once()
			},
			wantStatus: http.StatusForbidden,
			wantErrMsg: entity.ErrNotAnOwner.Error(),
		},
		{
			name: "a deactivated owner becomes 403",
			body: validVenueBody(),
			setup: func(venues *mocks.MockVenueUsecase) {
				venues.EXPECT().Create(mock.Anything, mock.Anything).
					Return(nil, entity.ErrOwnerInactive).Once()
			},
			wantStatus: http.StatusForbidden,
			wantErrMsg: entity.ErrOwnerInactive.Error(),
		},
		{
			name: "an unexpected error becomes 500 without leaking detail",
			body: validVenueBody(),
			setup: func(venues *mocks.MockVenueUsecase) {
				venues.EXPECT().Create(mock.Anything, mock.Anything).
					Return(nil, errors.New("pq: connection to 10.0.0.5 refused")).Once()
			},
			wantStatus: http.StatusInternalServerError,
			wantErrMsg: "internal error",
			check: func(t *testing.T, body []byte) {
				require.NotContains(t, string(body), "10.0.0.5")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			venues := mocks.NewMockVenueUsecase(t)

			switch {
			case tt.setup != nil:
				tt.setup(venues)
			case !tt.wantNoCall:
				venues.EXPECT().Create(mock.Anything, mock.Anything).
					Return(createdVenue(), nil).Once()
			}

			rec := do(t, mocks.NewMockUserUsecase(t), venues, http.MethodPost, "/venues", tt.body)

			require.Equal(t, tt.wantStatus, rec.Code)
			require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			if tt.wantNoCall {
				venues.AssertNotCalled(t, "Create")
			}

			if tt.wantErrMsg != "" {
				var got errorBody
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
				require.Equal(t, tt.wantErrMsg, got.Error)
			}

			if tt.check != nil {
				tt.check(t, rec.Body.Bytes())
			}
		})
	}
}

func TestCreateVenuePassesTheParsedOwner(t *testing.T) {
	venues := mocks.NewMockVenueUsecase(t)

	venues.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(in entity.CreateVenueInput) bool {
			return in.OwnerID == venueOwnerID
		})).
		Return(createdVenue(), nil).
		Once()

	rec := do(t, mocks.NewMockUserUsecase(t), venues, http.MethodPost, "/venues", validVenueBody())

	require.Equal(t, http.StatusCreated, rec.Code)
}

func TestListVenues(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		setup      func(venues *mocks.MockVenueUsecase)
		wantNoCall bool
		wantStatus int
		wantErrMsg string
		check      func(t *testing.T, body []byte)
	}{
		{
			name:  "returns the owner's venues",
			query: "?owner_id=" + venueOwnerID.String(),
			setup: func(venues *mocks.MockVenueUsecase) {
				venues.EXPECT().ListByOwner(mock.Anything, venueOwnerID).
					Return([]entity.Venue{*createdVenue(), *createdVenue()}, nil).Once()
			},
			wantStatus: http.StatusOK,
			check: func(t *testing.T, body []byte) {
				var got struct {
					Venues []map[string]any `json:"venues"`
				}
				require.NoError(t, json.Unmarshal(body, &got))
				require.Len(t, got.Venues, 2)
				require.Equal(t, "Ace Tennis Club", got.Venues[0]["name"])
			},
		},
		{
			name:  "an owner with no venues gets an empty array",
			query: "?owner_id=" + venueOwnerID.String(),
			setup: func(venues *mocks.MockVenueUsecase) {
				venues.EXPECT().ListByOwner(mock.Anything, venueOwnerID).
					Return([]entity.Venue{}, nil).Once()
			},
			wantStatus: http.StatusOK,
			check: func(t *testing.T, body []byte) {
				// null would break clients that iterate the result
				require.Contains(t, string(body), `"venues":[]`)
			},
		},
		{
			name:       "a missing owner_id is rejected",
			query:      "",
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "owner_id must be a valid uuid",
		},
		{
			name:       "an owner_id that is not a uuid is rejected",
			query:      "?owner_id=nope",
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "owner_id must be a valid uuid",
		},
		{
			name:  "an unexpected error becomes 500 without leaking detail",
			query: "?owner_id=" + venueOwnerID.String(),
			setup: func(venues *mocks.MockVenueUsecase) {
				venues.EXPECT().ListByOwner(mock.Anything, mock.Anything).
					Return(nil, errors.New("pq: connection to 10.0.0.5 refused")).Once()
			},
			wantStatus: http.StatusInternalServerError,
			wantErrMsg: "internal error",
			check: func(t *testing.T, body []byte) {
				require.NotContains(t, string(body), "10.0.0.5")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			venues := mocks.NewMockVenueUsecase(t)
			if tt.setup != nil {
				tt.setup(venues)
			}

			rec := do(t, mocks.NewMockUserUsecase(t), venues, http.MethodGet, "/venues"+tt.query, "")

			require.Equal(t, tt.wantStatus, rec.Code)

			if tt.wantNoCall {
				venues.AssertNotCalled(t, "ListByOwner")
			}

			if tt.wantErrMsg != "" {
				var got errorBody
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
				require.Equal(t, tt.wantErrMsg, got.Error)
			}

			if tt.check != nil {
				tt.check(t, rec.Body.Bytes())
			}
		})
	}
}
