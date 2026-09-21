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
	return &entity.Booking{
		ID:        uuid.New(),
		CourtID:   bookingCourtID,
		PlayerID:  venueOwnerID,
		StartsAt:  bookedSlot(),
		CreatedAt: time.Now(),
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

				startsAt, err := time.Parse(time.RFC3339, got["starts_at"].(string))
				require.NoError(t, err)
				require.True(t, bookedSlot().Equal(startsAt))

				// The hour is the client's to display, not to work out.
				endsAt, err := time.Parse(time.RFC3339, got["ends_at"].(string))
				require.NoError(t, err)
				require.True(t, bookedSlot().Add(time.Hour).Equal(endsAt))
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
					Return(createdBooking(), nil).Once()
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
		Return(createdBooking(), nil).
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
