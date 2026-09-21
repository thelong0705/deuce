package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/domain/entity"
	"github.com/thelong0705/deuce/internal/domain/usecase"
	"github.com/thelong0705/deuce/internal/domain/usecase/mocks"
)

var (
	bookingPlayerID = uuid.New()
	bookingCourtID  = uuid.New()
)

// nextSlot is the top of the next hour, which is always inside a 6-22 court's
// day in UTC except around midnight, so the fixtures open the court fully.
func nextSlot() time.Time {
	return time.Now().UTC().Truncate(time.Hour).Add(time.Hour)
}

func validBookSlotInput() entity.BookSlotInput {
	return entity.BookSlotInput{
		PlayerID: bookingPlayerID,
		CourtID:  bookingCourtID,
		StartsAt: nextSlot(),
	}
}

func activePlayer() *entity.User {
	return &entity.User{ID: bookingPlayerID, Role: entity.RolePlayer, IsActive: true}
}

func bookableCourt() *entity.Court {
	return &entity.Court{ID: bookingCourtID, OpenHour: 0, CloseHour: 24, IsActive: true}
}

func bookableVenue() *entity.Venue {
	return &entity.Venue{ID: uuid.New(), Timezone: "UTC", IsActive: true}
}

type bookingMocks struct {
	bookings *mocks.MockBookingRepo
	courts   *mocks.MockCourtFinder
	users    *mocks.MockUserFinder
}

// expectLookups sets up the two reads Book makes before it writes.
func (m bookingMocks) expectLookups() {
	m.users.EXPECT().GetUser(mock.Anything, bookingPlayerID).Return(activePlayer(), nil).Once()
	m.courts.EXPECT().GetCourtWithVenue(mock.Anything, bookingCourtID).
		Return(bookableCourt(), bookableVenue(), nil).Once()
}

func TestBookingBook(t *testing.T) {
	boom := errors.New("boom")

	tests := []struct {
		name string
		// mutate adjusts the valid input for this case
		mutate func(in *entity.BookSlotInput)
		// setup configures the mocks; nil means the happy path
		setup func(m bookingMocks)
		// wantErr is what Book must return
		wantErr error
		// wantNoStorage asserts the booking was never written
		wantNoStorage bool
	}{
		{
			name: "books the slot",
		},
		{
			name:          "rejects invalid input before looking anything up",
			mutate:        func(in *entity.BookSlotInput) { in.CourtID = uuid.Nil },
			setup:         func(bookingMocks) {},
			wantErr:       entity.ErrCourtRequired,
			wantNoStorage: true,
		},
		{
			name: "rejects an unknown player",
			setup: func(m bookingMocks) {
				m.users.EXPECT().GetUser(mock.Anything, bookingPlayerID).
					Return(nil, entity.ErrUserNotFound).Once()
			},
			wantErr:       entity.ErrUserNotFound,
			wantNoStorage: true,
		},
		{
			name: "an owner cannot book",
			setup: func(m bookingMocks) {
				owner := activePlayer()
				owner.Role = entity.RoleOwner
				m.users.EXPECT().GetUser(mock.Anything, bookingPlayerID).Return(owner, nil).Once()
			},
			wantErr:       entity.ErrNotAPlayer,
			wantNoStorage: true,
		},
		{
			name: "a deactivated player cannot book",
			setup: func(m bookingMocks) {
				player := activePlayer()
				player.IsActive = false
				m.users.EXPECT().GetUser(mock.Anything, bookingPlayerID).Return(player, nil).Once()
			},
			wantErr:       entity.ErrPlayerInactive,
			wantNoStorage: true,
		},
		{
			name: "rejects an unknown court",
			setup: func(m bookingMocks) {
				m.users.EXPECT().GetUser(mock.Anything, bookingPlayerID).Return(activePlayer(), nil).Once()
				m.courts.EXPECT().GetCourtWithVenue(mock.Anything, bookingCourtID).
					Return(nil, nil, entity.ErrCourtNotFound).Once()
			},
			wantErr:       entity.ErrCourtNotFound,
			wantNoStorage: true,
		},
		{
			name: "rejects a court at a deactivated venue",
			setup: func(m bookingMocks) {
				venue := bookableVenue()
				venue.IsActive = false
				m.users.EXPECT().GetUser(mock.Anything, bookingPlayerID).Return(activePlayer(), nil).Once()
				m.courts.EXPECT().GetCourtWithVenue(mock.Anything, bookingCourtID).
					Return(bookableCourt(), venue, nil).Once()
			},
			wantErr:       entity.ErrVenueInactive,
			wantNoStorage: true,
		},
		{
			name:   "rejects a slot outside the court's opening hours",
			mutate: func(in *entity.BookSlotInput) { in.StartsAt = nextSlot().Add(30 * time.Minute) },
			setup: func(m bookingMocks) {
				m.expectLookups()
			},
			wantErr:       entity.ErrSlotNotOnTheHour,
			wantNoStorage: true,
		},
		{
			name:   "rejects a slot in the past",
			mutate: func(in *entity.BookSlotInput) { in.StartsAt = nextSlot().Add(-24 * time.Hour) },
			setup: func(m bookingMocks) {
				m.expectLookups()
			},
			wantErr:       entity.ErrSlotInThePast,
			wantNoStorage: true,
		},
		{
			name:   "rejects a slot beyond the booking horizon",
			mutate: func(in *entity.BookSlotInput) { in.StartsAt = nextSlot().Add(entity.BookingHorizon) },
			setup: func(m bookingMocks) {
				m.expectLookups()
			},
			wantErr:       entity.ErrSlotTooFarAhead,
			wantNoStorage: true,
		},
		{
			// Storage decides who gets the slot, so this is the only place a
			// double booking is caught.
			name: "a slot taken by somebody else propagates",
			setup: func(m bookingMocks) {
				m.expectLookups()
				m.bookings.EXPECT().CreateBooking(mock.Anything, mock.Anything).
					Return(nil, entity.ErrSlotTaken).Once()
			},
			wantErr: entity.ErrSlotTaken,
		},
		{
			name: "propagates a storage failure",
			setup: func(m bookingMocks) {
				m.expectLookups()
				m.bookings.EXPECT().CreateBooking(mock.Anything, mock.Anything).
					Return(nil, boom).Once()
			},
			wantErr: boom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := bookingMocks{
				bookings: mocks.NewMockBookingRepo(t),
				courts:   mocks.NewMockCourtFinder(t),
				users:    mocks.NewMockUserFinder(t),
			}

			if tt.setup != nil {
				tt.setup(m)
			} else {
				m.expectLookups()
				m.bookings.EXPECT().CreateBooking(mock.Anything, mock.Anything).
					Return(&entity.Booking{CourtID: bookingCourtID}, nil).Once()
			}

			in := validBookSlotInput()
			if tt.mutate != nil {
				tt.mutate(&in)
			}

			svc := usecase.NewBooking(m.bookings, m.courts, m.users)
			_, err := svc.Book(context.Background(), in)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			if tt.wantNoStorage {
				m.bookings.AssertNotCalled(t, "CreateBooking")
			}
		})
	}
}

// The slot is handed to storage untouched, so the row holds the instant the
// player asked for.
func TestBookingBookStoresTheRequestedSlot(t *testing.T) {
	m := bookingMocks{
		bookings: mocks.NewMockBookingRepo(t),
		courts:   mocks.NewMockCourtFinder(t),
		users:    mocks.NewMockUserFinder(t),
	}

	in := validBookSlotInput()

	m.expectLookups()
	m.bookings.EXPECT().
		CreateBooking(mock.Anything, mock.MatchedBy(func(got entity.BookSlotInput) bool {
			return got.PlayerID == bookingPlayerID &&
				got.CourtID == bookingCourtID &&
				got.StartsAt.Equal(in.StartsAt)
		})).
		Return(&entity.Booking{}, nil).
		Once()

	svc := usecase.NewBooking(m.bookings, m.courts, m.users)
	_, err := svc.Book(context.Background(), in)

	require.NoError(t, err)
}
