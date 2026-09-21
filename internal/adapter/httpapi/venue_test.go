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
	return `{"name":"Ace Tennis Club","city":"Hanoi","address":"12 Le Loi","timezone":"Asia/Ho_Chi_Minh"}`
}

func venueOwner() *entity.User {
	return &entity.User{
		ID:       venueOwnerID,
		Email:    "owner@example.com",
		Role:     entity.RoleOwner,
		IsActive: true,
	}
}

// signedInOwner returns a user mock whose session resolves to the venue owner.
func signedInOwner(t *testing.T) *mocks.MockUserUsecase {
	t.Helper()

	users := mocks.NewMockUserUsecase(t)
	users.EXPECT().Authenticate(mock.Anything, rawToken).Return(venueOwner(), nil).Once()

	return users
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
			body:       `{"name":"A","city":"B","address":"C","courts":9}`,
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid JSON body",
		},
		{
			name:       "rejects an owner_id in the body",
			body:       `{"owner_id":"` + uuid.New().String() + `","name":"A","city":"B","address":"C"}`,
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid JSON body",
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
			// Reachable when the account is deleted between the session check
			// and the write.
			name: "an owner that no longer exists becomes 404",
			body: validVenueBody(),
			setup: func(venues *mocks.MockVenueUsecase) {
				venues.EXPECT().Create(mock.Anything, mock.Anything).
					Return(nil, entity.ErrUserNotFound).Once()
			},
			wantStatus: http.StatusNotFound,
			wantErrMsg: entity.ErrUserNotFound.Error(),
		},
		{
			name: "an unusable timezone becomes 400",
			body: `{"name":"A","city":"B","address":"C","timezone":"Hanoi/Somewhere"}`,
			setup: func(venues *mocks.MockVenueUsecase) {
				venues.EXPECT().Create(mock.Anything, mock.Anything).
					Return(nil, entity.ErrVenueTimezoneInvalid).Once()
			},
			wantStatus: http.StatusBadRequest,
			wantErrMsg: entity.ErrVenueTimezoneInvalid.Error(),
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

			rec := doAuthed(t, deps{users: signedInOwner(t), venues: venues}, http.MethodPost, "/venues", tt.body)

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

// The owner comes from the session, so a caller cannot create a venue for
// somebody else.
func TestCreateVenueTakesTheOwnerFromTheSession(t *testing.T) {
	venues := mocks.NewMockVenueUsecase(t)

	venues.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(in entity.CreateVenueInput) bool {
			return in.OwnerID == venueOwnerID
		})).
		Return(createdVenue(), nil).
		Once()

	rec := doAuthed(t, deps{users: signedInOwner(t), venues: venues}, http.MethodPost, "/venues", validVenueBody())

	require.Equal(t, http.StatusCreated, rec.Code)
}

func TestListVenues(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(venues *mocks.MockVenueUsecase)
		wantStatus int
		wantErrMsg string
		check      func(t *testing.T, body []byte)
	}{
		{
			name: "returns the signed-in owner's venues",
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
			name: "an owner with no venues gets an empty array",
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
			name: "an unexpected error becomes 500 without leaking detail",
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
			tt.setup(venues)

			rec := doAuthed(t, deps{users: signedInOwner(t), venues: venues}, http.MethodGet, "/venues", "")

			require.Equal(t, tt.wantStatus, rec.Code)

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

func TestVenueRoutesRequireASession(t *testing.T) {
	tests := []struct {
		name   string
		method string
		body   string
	}{
		{"create", http.MethodPost, validVenueBody()},
		{"list", http.MethodGet, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := mocks.NewMockUserUsecase(t)
			users.EXPECT().Authenticate(mock.Anything, "").
				Return(nil, entity.ErrSessionInvalid).Once()

			venues := mocks.NewMockVenueUsecase(t)

			rec := do(t, deps{users: users, venues: venues}, tt.method, "/venues", tt.body)

			require.Equal(t, http.StatusUnauthorized, rec.Code)
			venues.AssertNotCalled(t, "Create")
			venues.AssertNotCalled(t, "ListByOwner")
		})
	}
}
