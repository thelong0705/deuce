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
	return `{"name":"Court 1","open_hour":6,"close_hour":22,"price_per_hour":120000,"currency":"VND"}`
}

func createdCourt() *entity.Court {
	return &entity.Court{
		ID:           uuid.New(),
		VenueID:      courtVenueID,
		Name:         "Court 1",
		OpenHour:     6,
		CloseHour:    22,
		PricePerHour: 120000,
		Currency:     entity.CurrencyVND,
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
				require.Equal(t, "VND", got["currency"])
				require.Equal(t, true, got["is_active"])
				require.NotEmpty(t, got["id"])
			},
		},
		{
			name: "passes the currency through to the use case",
			body: validCourtBody(),
			setup: func(courts *mocks.MockCourtUsecase) {
				courts.EXPECT().
					Create(mock.Anything, mock.MatchedBy(func(in entity.CreateCourtInput) bool {
						return in.Currency == entity.CurrencyVND
					})).
					Return(createdCourt(), nil).Once()
			},
			wantStatus: http.StatusCreated,
		},
		{
			// The handler does not police the value; the domain does.
			name: "an unsupported currency is refused by the domain",
			body: `{"name":"Court 1","open_hour":6,"close_hour":22,"price_per_hour":120000,"currency":"USD"}`,
			setup: func(courts *mocks.MockCourtUsecase) {
				courts.EXPECT().Create(mock.Anything, mock.Anything).
					Return(nil, entity.ErrCurrencyInvalid).Once()
			},
			wantStatus: http.StatusBadRequest,
			wantErrMsg: entity.ErrCurrencyInvalid.Error(),
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

func TestListCourts(t *testing.T) {
	tests := []struct {
		name string
		// path defaults to the valid venue path when empty
		path       string
		setup      func(courts *mocks.MockCourtUsecase)
		wantNoCall bool
		wantStatus int
		wantCode   string
		check      func(t *testing.T, body []byte)
	}{
		{
			name: "lists the venue's courts",
			setup: func(courts *mocks.MockCourtUsecase) {
				courts.EXPECT().ListByVenue(mock.Anything, courtVenueID).
					Return([]entity.Court{*createdCourt()}, nil).Once()
			},
			wantStatus: http.StatusOK,
			check: func(t *testing.T, body []byte) {
				var got struct {
					Courts []map[string]any `json:"courts"`
				}
				require.NoError(t, json.Unmarshal(body, &got))
				require.Len(t, got.Courts, 1)
				require.Equal(t, "Court 1", got.Courts[0]["name"])
				require.Equal(t, float64(6), got.Courts[0]["open_hour"])
			},
		},
		{
			name: "a venue with no courts is an empty list, not null",
			setup: func(courts *mocks.MockCourtUsecase) {
				courts.EXPECT().ListByVenue(mock.Anything, mock.Anything).Return(nil, nil).Once()
			},
			wantStatus: http.StatusOK,
			check: func(t *testing.T, body []byte) {
				require.JSONEq(t, `{"courts":[]}`, string(body))
			},
		},
		{
			name:       "a venue id that is not a uuid is 400",
			path:       "/venues/not-a-uuid/courts",
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_venue_id",
		},
		{
			name: "an unknown venue is 404, not an empty list",
			setup: func(courts *mocks.MockCourtUsecase) {
				courts.EXPECT().ListByVenue(mock.Anything, mock.Anything).
					Return(nil, entity.ErrVenueNotFound).Once()
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "venue_not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			courts := mocks.NewMockCourtUsecase(t)
			if tt.setup != nil {
				tt.setup(courts)
			}

			path := tt.path
			if path == "" {
				path = courtsPath()
			}

			rec := doAuthed(t, deps{users: signedInOwner(t), courts: courts}, http.MethodGet, path, "")

			require.Equal(t, tt.wantStatus, rec.Code)

			if tt.wantNoCall {
				courts.AssertNotCalled(t, "ListByVenue")
			}

			if tt.wantCode != "" {
				var got errorBody
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
				require.Equal(t, tt.wantCode, got.Code)
			}

			if tt.check != nil {
				tt.check(t, rec.Body.Bytes())
			}
		})
	}
}

func searchResult() entity.CourtAvailability {
	at := time.Date(2026, 9, 25, 18, 0, 0, 0, time.UTC)

	return entity.CourtAvailability{
		Court: *createdCourt(),
		Venue: *createdVenue(),
		Slots: []entity.Slot{
			{StartsAt: at, Available: true},
			{StartsAt: at.Add(entity.SlotDuration), Available: true},
		},
	}
}

func TestSearchCourts(t *testing.T) {
	tests := []struct {
		name  string
		query string
		setup func(courts *mocks.MockCourtUsecase)
		// wantNoCall asserts the use case was never reached
		wantNoCall bool
		wantStatus int
		wantErrMsg string
		check      func(t *testing.T, body []byte)
	}{
		{
			name:  "returns the courts with something free",
			query: "?city=Hanoi&date=2026-09-25&from_hour=18&to_hour=22",
			setup: func(courts *mocks.MockCourtUsecase) {
				courts.EXPECT().Search(mock.Anything, mock.Anything).
					Return([]entity.CourtAvailability{searchResult()}, nil).Once()
			},
			wantStatus: http.StatusOK,
			check: func(t *testing.T, body []byte) {
				var got struct {
					Courts []struct {
						Court map[string]any   `json:"court"`
						Venue map[string]any   `json:"venue"`
						Slots []map[string]any `json:"slots"`
					} `json:"courts"`
				}
				require.NoError(t, json.Unmarshal(body, &got))

				require.Len(t, got.Courts, 1)
				require.Equal(t, "Court 1", got.Courts[0].Court["name"])
				// The venue travels with the court; its name alone says nothing.
				require.Equal(t, "Ace Tennis Club", got.Courts[0].Venue["name"])
				require.Len(t, got.Courts[0].Slots, 2)
				require.Equal(t, true, got.Courts[0].Slots[0]["available"])
				require.NotEmpty(t, got.Courts[0].Slots[0]["ends_at"])
			},
		},
		{
			name:  "a city with nothing free gets an empty array",
			query: "?city=Hanoi&date=2026-09-25",
			setup: func(courts *mocks.MockCourtUsecase) {
				courts.EXPECT().Search(mock.Anything, mock.Anything).
					Return([]entity.CourtAvailability{}, nil).Once()
			},
			wantStatus: http.StatusOK,
			check: func(t *testing.T, body []byte) {
				// null would break clients that iterate the result.
				require.Contains(t, string(body), `"courts":[]`)
			},
		},
		{
			name:       "a missing date is rejected",
			query:      "?city=Hanoi",
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "date must be YYYY-MM-DD",
		},
		{
			name:       "a date that is not a date is rejected",
			query:      "?city=Hanoi&date=friday",
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "date must be YYYY-MM-DD",
		},
		{
			name:       "an hour that is not a number is rejected",
			query:      "?city=Hanoi&date=2026-09-25&from_hour=evening",
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "from_hour and to_hour must be whole hours between 0 and 24",
		},
		{
			name:  "a missing city becomes 400",
			query: "?date=2026-09-25",
			setup: func(courts *mocks.MockCourtUsecase) {
				courts.EXPECT().Search(mock.Anything, mock.Anything).
					Return(nil, entity.ErrSearchCityRequired).Once()
			},
			wantStatus: http.StatusBadRequest,
			wantErrMsg: entity.ErrSearchCityRequired.Error(),
		},
		{
			name:  "an impossible window becomes 400",
			query: "?city=Hanoi&date=2026-09-25&from_hour=22&to_hour=6",
			setup: func(courts *mocks.MockCourtUsecase) {
				courts.EXPECT().Search(mock.Anything, mock.Anything).
					Return(nil, entity.ErrSearchHoursInvalid).Once()
			},
			wantStatus: http.StatusBadRequest,
			wantErrMsg: entity.ErrSearchHoursInvalid.Error(),
		},
		{
			name:  "an unexpected error becomes 500 without leaking detail",
			query: "?city=Hanoi&date=2026-09-25",
			setup: func(courts *mocks.MockCourtUsecase) {
				courts.EXPECT().Search(mock.Anything, mock.Anything).
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
			if tt.setup != nil {
				tt.setup(courts)
			}

			rec := doAuthed(t, deps{users: signedInOwner(t), courts: courts},
				http.MethodGet, "/courts/search"+tt.query, "")

			require.Equal(t, tt.wantStatus, rec.Code)

			if tt.wantNoCall {
				courts.AssertNotCalled(t, "Search")
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

// An absent window is the whole day, so a player who only names a date gets
// everything the court is open for.
func TestSearchCourtsDefaultsToTheWholeDay(t *testing.T) {
	courts := mocks.NewMockCourtUsecase(t)

	courts.EXPECT().
		Search(mock.Anything, mock.MatchedBy(func(in entity.CourtSearch) bool {
			return in.City == "Hanoi" &&
				in.FromHour == 0 &&
				in.ToHour == 24 &&
				in.Date.Format(time.DateOnly) == "2026-09-25"
		})).
		Return([]entity.CourtAvailability{}, nil).
		Once()

	rec := doAuthed(t, deps{users: signedInOwner(t), courts: courts},
		http.MethodGet, "/courts/search?city=Hanoi&date=2026-09-25", "")

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestSearchCourtsRequiresASession(t *testing.T) {
	users := mocks.NewMockUserUsecase(t)
	users.EXPECT().Authenticate(mock.Anything, "").
		Return(nil, entity.ErrSessionInvalid).Once()

	courts := mocks.NewMockCourtUsecase(t)

	rec := do(t, deps{users: users, courts: courts},
		http.MethodGet, "/courts/search?city=Hanoi&date=2026-09-25", "")

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	courts.AssertNotCalled(t, "Search")
}
