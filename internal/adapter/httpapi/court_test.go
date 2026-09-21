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

var courtVenueID = uuid.New()

func courtsPath() string {
	return "/venues/" + courtVenueID.String() + "/courts"
}

func validCourtBody() string {
	return `{"name":"Court 1","open_hour":6,"close_hour":22,"price_per_hour":120000}`
}

func createdCourt() *entity.Court {
	return &entity.Court{
		ID:           uuid.New(),
		VenueID:      courtVenueID,
		Name:         "Court 1",
		OpenHour:     6,
		CloseHour:    22,
		PricePerHour: 120000,
		IsActive:     true,
		CreatedAt:    time.Now(),
	}
}

func TestCreateCourt(t *testing.T) {
	tests := []struct {
		name string
		// path defaults to the valid venue path when empty
		path string
		body string
		// setup configures the mock; nil means a successful creation
		setup func(courts *mocks.MockCourtUsecase)
		// wantNoCall asserts the use case was never reached
		wantNoCall bool
		wantStatus int
		wantErrMsg string
		check      func(t *testing.T, body []byte)
	}{
		{
			name:       "registers a court",
			body:       validCourtBody(),
			wantStatus: http.StatusCreated,
			check: func(t *testing.T, body []byte) {
				var got map[string]any
				require.NoError(t, json.Unmarshal(body, &got))

				require.Equal(t, "Court 1", got["name"])
				require.Equal(t, courtVenueID.String(), got["venue_id"])
				require.Equal(t, float64(6), got["open_hour"])
				require.Equal(t, float64(22), got["close_hour"])
				require.Equal(t, float64(120000), got["price_per_hour"])
				require.Equal(t, true, got["is_active"])
				require.NotEmpty(t, got["id"])
			},
		},
		{
			name:       "rejects a venue id that is not a uuid",
			path:       "/venues/nope/courts",
			body:       validCourtBody(),
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "venue id must be a valid uuid",
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
			body:       `{"name":"Court 1","surface":"clay"}`,
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid JSON body",
		},
		{
			name: "a validation error becomes 400",
			body: validCourtBody(),
			setup: func(courts *mocks.MockCourtUsecase) {
				courts.EXPECT().Create(mock.Anything, mock.Anything).
					Return(nil, entity.ErrCourtHoursInvalid).Once()
			},
			wantStatus: http.StatusBadRequest,
			wantErrMsg: entity.ErrCourtHoursInvalid.Error(),
		},
		{
			name: "an unknown venue becomes 404",
			body: validCourtBody(),
			setup: func(courts *mocks.MockCourtUsecase) {
				courts.EXPECT().Create(mock.Anything, mock.Anything).
					Return(nil, entity.ErrVenueNotFound).Once()
			},
			wantStatus: http.StatusNotFound,
			wantErrMsg: entity.ErrVenueNotFound.Error(),
		},
		{
			name: "somebody else's venue becomes 403",
			body: validCourtBody(),
			setup: func(courts *mocks.MockCourtUsecase) {
				courts.EXPECT().Create(mock.Anything, mock.Anything).
					Return(nil, entity.ErrNotVenueOwner).Once()
			},
			wantStatus: http.StatusForbidden,
			wantErrMsg: entity.ErrNotVenueOwner.Error(),
		},
		{
			name: "a duplicate court name becomes 409",
			body: validCourtBody(),
			setup: func(courts *mocks.MockCourtUsecase) {
				courts.EXPECT().Create(mock.Anything, mock.Anything).
					Return(nil, entity.ErrCourtNameTaken).Once()
			},
			wantStatus: http.StatusConflict,
			wantErrMsg: entity.ErrCourtNameTaken.Error(),
		},
		{
			name: "an unexpected error becomes 500 without leaking detail",
			body: validCourtBody(),
			setup: func(courts *mocks.MockCourtUsecase) {
				courts.EXPECT().Create(mock.Anything, mock.Anything).
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
			courts := mocks.NewMockCourtUsecase(t)

			switch {
			case tt.setup != nil:
				tt.setup(courts)
			case !tt.wantNoCall:
				courts.EXPECT().Create(mock.Anything, mock.Anything).
					Return(createdCourt(), nil).Once()
			}

			path := tt.path
			if path == "" {
				path = courtsPath()
			}

			rec := doAuthed(t, deps{users: signedInOwner(t), courts: courts}, http.MethodPost, path, tt.body)

			require.Equal(t, tt.wantStatus, rec.Code)
			require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			if tt.wantNoCall {
				courts.AssertNotCalled(t, "Create")
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

// The venue comes from the path and the owner from the session, so neither is
// something the body can claim.
func TestCreateCourtTakesTheVenueAndOwnerFromTheRequest(t *testing.T) {
	courts := mocks.NewMockCourtUsecase(t)

	courts.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(in entity.CreateCourtInput) bool {
			return in.VenueID == courtVenueID && in.OwnerID == venueOwnerID
		})).
		Return(createdCourt(), nil).
		Once()

	rec := doAuthed(t, deps{users: signedInOwner(t), courts: courts}, http.MethodPost, courtsPath(), validCourtBody())

	require.Equal(t, http.StatusCreated, rec.Code)
}

func TestCreateCourtRequiresASession(t *testing.T) {
	users := mocks.NewMockUserUsecase(t)
	users.EXPECT().Authenticate(mock.Anything, "").
		Return(nil, entity.ErrSessionInvalid).Once()

	courts := mocks.NewMockCourtUsecase(t)

	rec := do(t, deps{users: users, courts: courts}, http.MethodPost, courtsPath(), validCourtBody())

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	courts.AssertNotCalled(t, "Create")
}
