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

var bookingCourtID = uuid.New()

func bookingsPath() string {
	return "/courts/" + bookingCourtID.String() + "/bookings"
}

func bookedSlot() time.Time {
	return time.Date(2026, 9, 23, 9, 0, 0, 0, time.UTC)
}

func validBookingBody() string {
	return `{"starts_at":"` + bookedSlot().Format(time.RFC3339) + `"}`
}

func createdBooking() *entity.Booking {
	amount := 300000

	return &entity.Booking{
		ID:        uuid.New(),
		CourtID:   bookingCourtID,
		PlayerID:  venueOwnerID,
		StartsAt:  bookedSlot(),
		Status:    entity.StatusPendingPayment,
		Amount:    &amount,
		CreatedAt: time.Now(),
	}
}

// heldBooking is what Book returns: the slot, and the secret the browser pays
// with.
func heldBooking() *entity.HeldBooking {
	return &entity.HeldBooking{
		Booking:      *createdBooking(),
		ClientSecret: "pi_1_secret_abc",
	}
}

func TestCreateBooking(t *testing.T) {
	tests := []struct {
		name string
		// path defaults to the valid court path when empty
		path string
		body string
		// setup configures the mock; nil means a successful booking
		setup func(bookings *mocks.MockBookingUsecase)
		// wantNoCall asserts the use case was never reached
		wantNoCall bool
		wantStatus int
		wantErrMsg string
		check      func(t *testing.T, body []byte)
	}{
		{
			name:       "books the slot",
			body:       validBookingBody(),
			wantStatus: http.StatusCreated,
			check: func(t *testing.T, body []byte) {
				var got map[string]any
				require.NoError(t, json.Unmarshal(body, &got))

				require.Equal(t, bookingCourtID.String(), got["court_id"])
				require.Equal(t, venueOwnerID.String(), got["player_id"])
				require.NotEmpty(t, got["id"])
				// The browser needs the secret to pay, and this is the only
				// response that carries it.
				require.Equal(t, "pi_1_secret_abc", got["client_secret"])

				// A held slot is not a booking yet, and the amount says what
				// would settle it.
				require.Equal(t, "pending_payment", got["status"])
				require.Equal(t, float64(300000), got["amount"])

				startsAt, err := time.Parse(time.RFC3339, got["starts_at"].(string))
				require.NoError(t, err)
				require.True(t, bookedSlot().Equal(startsAt))

				// How long a slot runs is the server's to say, not the
				// client's to work out.
				endsAt, err := time.Parse(time.RFC3339, got["ends_at"].(string))
				require.NoError(t, err)
				require.True(t, bookedSlot().Add(entity.SlotDuration).Equal(endsAt))
			},
		},
		{
			name:       "rejects a court id that is not a uuid",
			path:       "/courts/nope/bookings",
			body:       validBookingBody(),
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "court id must be a valid uuid",
		},
		{
			name:       "rejects malformed json",
			body:       `{"starts_at":`,
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid JSON body",
		},
		{
			name:       "rejects an unknown field",
			body:       `{"starts_at":"2026-09-23T09:00:00Z","player_id":"someone-else"}`,
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid JSON body",
		},
		{
			name:       "rejects a start time that is not RFC 3339",
			body:       `{"starts_at":"tomorrow at nine"}`,
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "starts_at must be an RFC 3339 timestamp",
		},
		{
			name:       "rejects a missing start time",
			body:       `{}`,
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "starts_at must be an RFC 3339 timestamp",
		},
		{
			name: "a slot off the hour becomes 400",
			body: validBookingBody(),
			setup: func(bookings *mocks.MockBookingUsecase) {
				bookings.EXPECT().Book(mock.Anything, mock.Anything).
					Return(nil, entity.ErrSlotNotOnTheHour).Once()
			},
			wantStatus: http.StatusBadRequest,
			wantErrMsg: entity.ErrSlotNotOnTheHour.Error(),
		},
		{
			name: "a slot beyond the horizon becomes 400",
			body: validBookingBody(),
			setup: func(bookings *mocks.MockBookingUsecase) {
				bookings.EXPECT().Book(mock.Anything, mock.Anything).
					Return(nil, entity.ErrSlotTooFarAhead).Once()
			},
			wantStatus: http.StatusBadRequest,
			wantErrMsg: entity.ErrSlotTooFarAhead.Error(),
		},
		{
			name: "an unknown court becomes 404",
			body: validBookingBody(),
			setup: func(bookings *mocks.MockBookingUsecase) {
				bookings.EXPECT().Book(mock.Anything, mock.Anything).
					Return(nil, entity.ErrCourtNotFound).Once()
			},
			wantStatus: http.StatusNotFound,
			wantErrMsg: entity.ErrCourtNotFound.Error(),
		},
		{
			name: "an owner booking becomes 403",
			body: validBookingBody(),
			setup: func(bookings *mocks.MockBookingUsecase) {
				bookings.EXPECT().Book(mock.Anything, mock.Anything).
					Return(nil, entity.ErrNotAPlayer).Once()
			},
			wantStatus: http.StatusForbidden,
			wantErrMsg: entity.ErrNotAPlayer.Error(),
		},
		{
			// The race the unique index settles, seen from the losing side.
			name: "a slot somebody else got becomes 409",
			body: validBookingBody(),
			setup: func(bookings *mocks.MockBookingUsecase) {
				bookings.EXPECT().Book(mock.Anything, mock.Anything).
					Return(nil, entity.ErrSlotTaken).Once()
			},
			wantStatus: http.StatusConflict,
			wantErrMsg: entity.ErrSlotTaken.Error(),
		},
		{
			name: "an unexpected error becomes 500 without leaking detail",
			body: validBookingBody(),
			setup: func(bookings *mocks.MockBookingUsecase) {
				bookings.EXPECT().Book(mock.Anything, mock.Anything).
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
			bookings := mocks.NewMockBookingUsecase(t)

			switch {
			case tt.setup != nil:
				tt.setup(bookings)
			case !tt.wantNoCall:
				bookings.EXPECT().Book(mock.Anything, mock.Anything).
					Return(heldBooking(), nil).Once()
			}

			path := tt.path
			if path == "" {
				path = bookingsPath()
			}

			rec := doAuthed(t, deps{users: signedInOwner(t), bookings: bookings}, http.MethodPost, path, tt.body)

			require.Equal(t, tt.wantStatus, rec.Code)
			require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			if tt.wantNoCall {
				bookings.AssertNotCalled(t, "Book")
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

// The court comes from the path and the player from the session, and the offset
// the client sent is preserved as the instant it names.
func TestCreateBookingTakesTheCourtPlayerAndInstantFromTheRequest(t *testing.T) {
	bookings := mocks.NewMockBookingUsecase(t)

	// 16:00+07:00 is the same moment as 09:00Z.
	bookings.EXPECT().
		Book(mock.Anything, mock.MatchedBy(func(in entity.BookSlotInput) bool {
			return in.CourtID == bookingCourtID &&
				in.PlayerID == venueOwnerID &&
				in.StartsAt.Equal(bookedSlot())
		})).
		Return(heldBooking(), nil).
		Once()

	rec := doAuthed(t, deps{users: signedInOwner(t), bookings: bookings}, http.MethodPost, bookingsPath(),
		`{"starts_at":"2026-09-23T16:00:00+07:00"}`)

	require.Equal(t, http.StatusCreated, rec.Code)
}

func TestCreateBookingRequiresASession(t *testing.T) {
	users := mocks.NewMockUserUsecase(t)
	users.EXPECT().Authenticate(mock.Anything, "").
		Return(nil, entity.ErrSessionInvalid).Once()

	bookings := mocks.NewMockBookingUsecase(t)

	rec := do(t, deps{users: users, bookings: bookings}, http.MethodPost, bookingsPath(), validBookingBody())

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	bookings.AssertNotCalled(t, "Book")
}

func availabilityPath(query string) string {
	return "/courts/" + bookingCourtID.String() + "/availability" + query
}

func TestCourtAvailability(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		setup      func(bookings *mocks.MockBookingUsecase)
		wantNoCall bool
		wantStatus int
		wantCode   string
		check      func(t *testing.T, body []byte)
	}{
		{
			name: "lists the day's hours and whether each is free",
			path: availabilityPath("?date=2026-09-23"),
			setup: func(bookings *mocks.MockBookingUsecase) {
				bookings.EXPECT().
					Availability(mock.Anything, bookingCourtID, mock.MatchedBy(func(day time.Time) bool {
						y, m, d := day.Date()
						return y == 2026 && m == time.September && d == 23
					})).
					Return([]entity.Slot{
						{StartsAt: bookedSlot(), Available: true},
						{StartsAt: bookedSlot().Add(time.Hour), Available: false},
					}, nil).Once()
			},
			wantStatus: http.StatusOK,
			check: func(t *testing.T, body []byte) {
				var got struct {
					Slots []struct {
						StartsAt  string `json:"starts_at"`
						EndsAt    string `json:"ends_at"`
						Available bool   `json:"available"`
					} `json:"slots"`
				}
				require.NoError(t, json.Unmarshal(body, &got))
				require.Len(t, got.Slots, 2)
				require.True(t, got.Slots[0].Available)
				require.False(t, got.Slots[1].Available)
				require.Equal(t, bookedSlot().Format(time.RFC3339), got.Slots[0].StartsAt)

				// Each window carries its own end, so the client never has to
				// know how long a slot runs.
				require.Equal(t,
					bookedSlot().Add(entity.SlotDuration).Format(time.RFC3339),
					got.Slots[0].EndsAt,
				)
			},
		},
		{
			name: "a closed day is an empty list, not null",
			path: availabilityPath("?date=2026-09-23"),
			setup: func(bookings *mocks.MockBookingUsecase) {
				bookings.EXPECT().Availability(mock.Anything, mock.Anything, mock.Anything).
					Return(nil, nil).Once()
			},
			wantStatus: http.StatusOK,
			check: func(t *testing.T, body []byte) {
				require.JSONEq(t, `{"slots":[]}`, string(body))
			},
		},
		{
			name:       "a court id that is not a uuid is 400",
			path:       "/courts/not-a-uuid/availability?date=2026-09-23",
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_court_id",
		},
		{
			name:       "a missing date is 400",
			path:       availabilityPath(""),
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_date",
		},
		{
			name:       "a date that is not YYYY-MM-DD is 400",
			path:       availabilityPath("?date=23-09-2026"),
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_date",
		},
		{
			// A timestamp is not a calendar date, and guessing which day the
			// caller meant is worse than saying so.
			name:       "a full timestamp is 400",
			path:       availabilityPath("?date=2026-09-23T09:00:00Z"),
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_date",
		},
		{
			name: "an unknown court is 404",
			path: availabilityPath("?date=2026-09-23"),
			setup: func(bookings *mocks.MockBookingUsecase) {
				bookings.EXPECT().Availability(mock.Anything, mock.Anything, mock.Anything).
					Return(nil, entity.ErrCourtNotFound).Once()
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "court_not_found",
		},
		{
			name: "a slot mid-window is refused by the domain",
			path: availabilityPath("?date=2026-09-23"),
			setup: func(bookings *mocks.MockBookingUsecase) {
				bookings.EXPECT().Availability(mock.Anything, mock.Anything, mock.Anything).
					Return(nil, entity.ErrSlotNotOnTheGrid).Once()
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "slot_not_on_the_grid",
		},
		{
			name: "a deactivated venue is 403",
			path: availabilityPath("?date=2026-09-23"),
			setup: func(bookings *mocks.MockBookingUsecase) {
				bookings.EXPECT().Availability(mock.Anything, mock.Anything, mock.Anything).
					Return(nil, entity.ErrVenueInactive).Once()
			},
			wantStatus: http.StatusForbidden,
			wantCode:   "venue_inactive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bookings := mocks.NewMockBookingUsecase(t)
			if tt.setup != nil {
				tt.setup(bookings)
			}

			rec := doAuthed(t, deps{users: signedInOwner(t), bookings: bookings},
				http.MethodGet, tt.path, "")

			require.Equal(t, tt.wantStatus, rec.Code)

			if tt.wantNoCall {
				bookings.AssertNotCalled(t, "Availability")
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

func TestListBookings(t *testing.T) {
	playerBooking := entity.PlayerBooking{
		Booking: *createdBooking(),
		Court:   entity.Court{ID: bookingCourtID, Name: "Court 1", PricePerHour: 150},
		Venue:   entity.Venue{ID: uuid.New(), Name: "Ace Tennis Club", City: "Ha Noi"},
	}

	tests := []struct {
		name       string
		setup      func(bookings *mocks.MockBookingUsecase)
		wantStatus int
		check      func(t *testing.T, body []byte)
	}{
		{
			name: "carries the court and venue names, not just ids",
			setup: func(bookings *mocks.MockBookingUsecase) {
				bookings.EXPECT().ListForPlayer(mock.Anything, venueOwnerID).
					Return([]entity.PlayerBooking{playerBooking}, nil).Once()
			},
			wantStatus: http.StatusOK,
			check: func(t *testing.T, body []byte) {
				var got struct {
					Bookings []struct {
						ID       string `json:"id"`
						StartsAt string `json:"starts_at"`
						EndsAt   string `json:"ends_at"`
						Status   string `json:"status"`
						Amount   *int   `json:"amount"`
						Court    struct {
							Name         string `json:"name"`
							PricePerHour int    `json:"price_per_hour"`
						} `json:"court"`
						Venue struct {
							Name string `json:"name"`
							City string `json:"city"`
						} `json:"venue"`
					} `json:"bookings"`
				}
				require.NoError(t, json.Unmarshal(body, &got))
				require.Len(t, got.Bookings, 1)

				require.Equal(t, "Court 1", got.Bookings[0].Court.Name)
				require.Equal(t, 150, got.Bookings[0].Court.PricePerHour)
				require.Equal(t, "Ace Tennis Club", got.Bookings[0].Venue.Name)
				require.Equal(t, "Ha Noi", got.Bookings[0].Venue.City)

				// The embedded booking's fields stay at the top level.
				require.NotEmpty(t, got.Bookings[0].ID)
				// Without this a held slot reads as a booking.
				require.Equal(t, "pending_payment", got.Bookings[0].Status)
				require.NotNil(t, got.Bookings[0].Amount)
				require.Equal(t, bookedSlot().Format(time.RFC3339), got.Bookings[0].StartsAt)
				require.Equal(t,
					bookedSlot().Add(entity.SlotDuration).Format(time.RFC3339),
					got.Bookings[0].EndsAt,
				)
			},
		},
		{
			name: "nothing booked is an empty list, not null",
			setup: func(bookings *mocks.MockBookingUsecase) {
				bookings.EXPECT().ListForPlayer(mock.Anything, mock.Anything).Return(nil, nil).Once()
			},
			wantStatus: http.StatusOK,
			check: func(t *testing.T, body []byte) {
				require.JSONEq(t, `{"bookings":[]}`, string(body))
			},
		},
		{
			name: "an unexpected failure is 500 without leaking detail",
			setup: func(bookings *mocks.MockBookingUsecase) {
				bookings.EXPECT().ListForPlayer(mock.Anything, mock.Anything).
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
			bookings := mocks.NewMockBookingUsecase(t)
			tt.setup(bookings)

			rec := doAuthed(t, deps{users: signedInOwner(t), bookings: bookings},
				http.MethodGet, "/bookings", "")

			require.Equal(t, tt.wantStatus, rec.Code)
			tt.check(t, rec.Body.Bytes())
		})
	}
}

// The player is the session's, never the caller's to name.
func TestListBookingsRequiresASession(t *testing.T) {
	users := mocks.NewMockUserUsecase(t)
	users.EXPECT().Authenticate(mock.Anything, "").Return(nil, entity.ErrSessionInvalid).Once()

	bookings := mocks.NewMockBookingUsecase(t)

	rec := do(t, deps{users: users, bookings: bookings}, http.MethodGet, "/bookings", "")

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	bookings.AssertNotCalled(t, "ListForPlayer")
}
