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
	heldBookingID   = uuid.New()
)

// nextSlot is the start of the next window on a court that opens at midnight,
// which is what the fixtures use. Truncating to the slot width rather than to
// the hour is what keeps it on the grid whatever time the suite runs at.
func nextSlot() time.Time {
	return time.Now().UTC().Truncate(entity.SlotDuration).Add(entity.SlotDuration)
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
	payments *mocks.MockPaymentGateway
	events   *mocks.MockPaymentEventLog
}

func newBookingMocks(t *testing.T) bookingMocks {
	t.Helper()

	return bookingMocks{
		bookings: mocks.NewMockBookingRepo(t),
		courts:   mocks.NewMockCourtFinder(t),
		users:    mocks.NewMockUserFinder(t),
		payments: mocks.NewMockPaymentGateway(t),
		events:   mocks.NewMockPaymentEventLog(t),
	}
}

func (m bookingMocks) svc() *usecase.Booking {
	return usecase.NewBooking(m.bookings, m.courts, m.users, m.payments, m.events)
}

// expectHoldAndPay sets up the write path: the slot is taken, a payment is
// opened for it, and the intent is recorded against the row.
func (m bookingMocks) expectHoldAndPay() {
	m.bookings.EXPECT().HoldSlot(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&entity.Booking{ID: heldBookingID, CourtID: bookingCourtID}, nil).Once()
	m.payments.EXPECT().CreatePayment(mock.Anything, mock.Anything).
		Return(&entity.Payment{IntentID: "pi_1", ClientSecret: "pi_1_secret"}, nil).Once()
	m.bookings.EXPECT().AttachPayment(mock.Anything, heldBookingID, "pi_1").Return(nil).Once()
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
				m.bookings.EXPECT().HoldSlot(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, entity.ErrSlotTaken).Once()
			},
			wantErr: entity.ErrSlotTaken,
		},
		{
			name: "propagates a storage failure",
			setup: func(m bookingMocks) {
				m.expectLookups()
				m.bookings.EXPECT().HoldSlot(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, boom).Once()
			},
			wantErr: boom,
		},
		{
			// Nothing can pay for the hold now, so it goes back rather than
			// sitting on the slot until it lapses.
			name: "a gateway failure releases the slot",
			setup: func(m bookingMocks) {
				m.expectLookups()
				m.bookings.EXPECT().HoldSlot(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(&entity.Booking{ID: heldBookingID}, nil).Once()
				m.payments.EXPECT().CreatePayment(mock.Anything, mock.Anything).
					Return(nil, boom).Once()
				m.bookings.EXPECT().CancelBooking(mock.Anything, heldBookingID).Return(nil).Once()
			},
			wantErr: boom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newBookingMocks(t)

			if tt.setup != nil {
				tt.setup(m)
			} else {
				m.expectLookups()
				m.expectHoldAndPay()
			}

			in := validBookSlotInput()
			if tt.mutate != nil {
				tt.mutate(&in)
			}

			_, err := m.svc().Book(context.Background(), in)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			if tt.wantNoStorage {
				m.bookings.AssertNotCalled(t, "HoldSlot")
			}
		})
	}
}

// The slot is handed to storage untouched, so the row holds the instant the
// player asked for, priced at what the court charges for a slot.
func TestBookingBookHoldsTheRequestedSlot(t *testing.T) {
	m := newBookingMocks(t)

	in := validBookSlotInput()

	m.expectLookups()
	m.bookings.EXPECT().
		HoldSlot(mock.Anything, mock.MatchedBy(func(got entity.BookSlotInput) bool {
			return got.PlayerID == bookingPlayerID &&
				got.CourtID == bookingCourtID &&
				got.StartsAt.Equal(in.StartsAt)
		}), bookableCourt().SlotPrice(), mock.Anything).
		Return(&entity.Booking{ID: heldBookingID}, nil).
		Once()
	m.payments.EXPECT().CreatePayment(mock.Anything, mock.Anything).
		Return(&entity.Payment{IntentID: "pi_1", ClientSecret: "pi_1_secret"}, nil).Once()
	m.bookings.EXPECT().AttachPayment(mock.Anything, heldBookingID, "pi_1").Return(nil).Once()

	_, err := m.svc().Book(context.Background(), in)

	require.NoError(t, err)
}

// The secret reaches the caller and the intent is on the booking, which is
// what the webhook later matches on.
func TestBookingBookReturnsTheClientSecret(t *testing.T) {
	m := newBookingMocks(t)

	m.expectLookups()
	m.expectHoldAndPay()

	held, err := m.svc().Book(context.Background(), validBookSlotInput())

	require.NoError(t, err)
	require.Equal(t, "pi_1_secret", held.ClientSecret)
	require.Equal(t, "pi_1", held.Booking.PaymentIntentID)
}

// The gateway is told what the court charges, in the court's own currency.
func TestBookingBookChargesTheCourtsPrice(t *testing.T) {
	m := newBookingMocks(t)

	court := bookableCourt()

	m.expectLookups()
	m.bookings.EXPECT().HoldSlot(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&entity.Booking{ID: heldBookingID}, nil).Once()
	m.payments.EXPECT().
		CreatePayment(mock.Anything, mock.MatchedBy(func(in entity.PaymentRequest) bool {
			return in.BookingID == heldBookingID &&
				in.Amount == court.SlotPrice() &&
				in.Currency == court.Currency
		})).
		Return(&entity.Payment{IntentID: "pi_1", ClientSecret: "pi_1_secret"}, nil).
		Once()
	m.bookings.EXPECT().AttachPayment(mock.Anything, heldBookingID, "pi_1").Return(nil).Once()

	_, err := m.svc().Book(context.Background(), validBookSlotInput())

	require.NoError(t, err)
}

func TestBookingAvailability(t *testing.T) {
	boom := errors.New("boom")
	// Tomorrow, so no hour of the day is refused for having passed.
	day := time.Now().UTC().AddDate(0, 0, 1)

	tests := []struct {
		name  string
		setup func(m bookingMocks)
		// wantAvailable is the availability flag per returned slot
		wantAvailable []bool
		wantErr       error
	}{
		{
			name: "every window, all free",
			setup: func(m bookingMocks) {
				m.courts.EXPECT().GetCourtWithVenue(mock.Anything, bookingCourtID).
					Return(&entity.Court{ID: bookingCourtID, OpenHour: 6, CloseHour: 12, IsActive: true},
						bookableVenue(), nil).Once()
				m.bookings.EXPECT().ListBookedSlots(mock.Anything, bookingCourtID, mock.Anything, mock.Anything).
					Return(nil, nil).Once()
			},
			wantAvailable: []bool{true, true, true},
		},
		{
			name: "a booked window comes back unavailable, and stays listed",
			setup: func(m bookingMocks) {
				m.courts.EXPECT().GetCourtWithVenue(mock.Anything, bookingCourtID).
					Return(&entity.Court{ID: bookingCourtID, OpenHour: 6, CloseHour: 12, IsActive: true},
						bookableVenue(), nil).Once()
				taken := time.Date(day.Year(), day.Month(), day.Day(), 8, 0, 0, 0, time.UTC)
				m.bookings.EXPECT().ListBookedSlots(mock.Anything, bookingCourtID, mock.Anything, mock.Anything).
					Return([]time.Time{taken}, nil).Once()
			},
			wantAvailable: []bool{true, false, true},
		},
		{
			name: "a booking stored in another offset still matches its hour",
			setup: func(m bookingMocks) {
				m.courts.EXPECT().GetCourtWithVenue(mock.Anything, bookingCourtID).
					Return(&entity.Court{ID: bookingCourtID, OpenHour: 6, CloseHour: 10, IsActive: true},
						bookableVenue(), nil).Once()
				// The same instant as 06:00 UTC, written down somewhere else.
				elsewhere := time.FixedZone("UTC+7", 7*3600)
				taken := time.Date(day.Year(), day.Month(), day.Day(), 13, 0, 0, 0, elsewhere)
				m.bookings.EXPECT().ListBookedSlots(mock.Anything, bookingCourtID, mock.Anything, mock.Anything).
					Return([]time.Time{taken}, nil).Once()
			},
			wantAvailable: []bool{false, true},
		},
		{
			name: "a closed court is not asked about bookings",
			setup: func(m bookingMocks) {
				m.courts.EXPECT().GetCourtWithVenue(mock.Anything, bookingCourtID).
					Return(&entity.Court{ID: bookingCourtID, OpenHour: 6, CloseHour: 12}, bookableVenue(), nil).Once()
			},
			wantAvailable: []bool{},
		},
		{
			name: "a deactivated venue is refused",
			setup: func(m bookingMocks) {
				venue := bookableVenue()
				venue.IsActive = false
				m.courts.EXPECT().GetCourtWithVenue(mock.Anything, bookingCourtID).
					Return(bookableCourt(), venue, nil).Once()
			},
			wantErr: entity.ErrVenueInactive,
		},
		{
			name: "an unknown court is refused",
			setup: func(m bookingMocks) {
				m.courts.EXPECT().GetCourtWithVenue(mock.Anything, bookingCourtID).
					Return(nil, nil, entity.ErrCourtNotFound).Once()
			},
			wantErr: entity.ErrCourtNotFound,
		},
		{
			name: "propagates a storage failure",
			setup: func(m bookingMocks) {
				m.courts.EXPECT().GetCourtWithVenue(mock.Anything, bookingCourtID).
					Return(bookableCourt(), bookableVenue(), nil).Once()
				m.bookings.EXPECT().ListBookedSlots(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
			tt.setup(m)

			svc := m.svc()
			slots, err := svc.Availability(context.Background(), bookingCourtID, day)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, slots)
				return
			}

			require.NoError(t, err)

			available := make([]bool, 0, len(slots))
			for _, slot := range slots {
				available = append(available, slot.Available)
			}
			require.Equal(t, tt.wantAvailable, available)
		})
	}
}

func TestBookingAvailabilityRejectsAnEmptyCourt(t *testing.T) {
	svc := newBookingMocks(t).svc()

	_, err := svc.Availability(context.Background(), uuid.Nil, time.Now())
	require.ErrorIs(t, err, entity.ErrCourtRequired)
}

func TestBookingListForPlayer(t *testing.T) {
	boom := errors.New("boom")

	tests := []struct {
		name     string
		playerID uuid.UUID
		setup    func(m *mocks.MockBookingRepo)
		wantLen  int
		wantErr  error
	}{
		{
			name:     "returns what storage holds",
			playerID: bookingPlayerID,
			setup: func(m *mocks.MockBookingRepo) {
				m.EXPECT().ListPlayerBookings(mock.Anything, bookingPlayerID, mock.Anything).
					Return([]entity.PlayerBooking{{}, {}}, nil).Once()
			},
			wantLen: 2,
		},
		{
			name:     "an empty player is refused without a lookup",
			playerID: uuid.Nil,
			setup:    func(*mocks.MockBookingRepo) {},
			wantErr:  entity.ErrPlayerRequired,
		},
		{
			name:     "propagates a storage failure",
			playerID: bookingPlayerID,
			setup: func(m *mocks.MockBookingRepo) {
				m.EXPECT().ListPlayerBookings(mock.Anything, mock.Anything, mock.Anything).
					Return(nil, boom).Once()
			},
			wantErr: boom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bookings := mocks.NewMockBookingRepo(t)
			tt.setup(bookings)

			svc := usecase.NewBooking(bookings, mocks.NewMockCourtFinder(t), mocks.NewMockUserFinder(t), mocks.NewMockPaymentGateway(t), mocks.NewMockPaymentEventLog(t))
			got, err := svc.ListForPlayer(context.Background(), tt.playerID)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Len(t, got, tt.wantLen)
		})
	}
}

// The window asked of storage has to cover every slot offered, or an hour
// booked at the edge of the day would be reported free.
func TestBookingAvailabilityAsksForTheWholeDay(t *testing.T) {
	day := time.Now().UTC().AddDate(0, 0, 1)

	bookings := mocks.NewMockBookingRepo(t)
	courts := mocks.NewMockCourtFinder(t)

	courts.EXPECT().GetCourtWithVenue(mock.Anything, bookingCourtID).
		Return(&entity.Court{ID: bookingCourtID, OpenHour: 6, CloseHour: 12, IsActive: true},
			bookableVenue(), nil).Once()

	var from, to time.Time
	bookings.EXPECT().ListBookedSlots(mock.Anything, bookingCourtID, mock.Anything, mock.Anything).
		Run(func(_ context.Context, _ uuid.UUID, f, t time.Time) { from, to = f, t }).
		Return(nil, nil).Once()

	svc := usecase.NewBooking(bookings, courts, mocks.NewMockUserFinder(t), mocks.NewMockPaymentGateway(t), mocks.NewMockPaymentEventLog(t))
	slots, err := svc.Availability(context.Background(), bookingCourtID, day)
	require.NoError(t, err)
	require.Len(t, slots, 3)

	require.Equal(t, slots[0].StartsAt, from)
	// Exclusive, and an hour past the last slot's start, so the last slot's
	// own booking still falls inside.
	require.Equal(t, slots[2].StartsAt.Add(entity.SlotDuration), to)
	require.True(t, to.After(slots[2].StartsAt))
}

func paymentEvent(t entity.PaymentEventType) entity.PaymentEvent {
	return entity.PaymentEvent{ID: "evt_1", Type: t, IntentID: "pi_1"}
}

func heldBooking(status entity.BookingStatus) *entity.Booking {
	return &entity.Booking{ID: heldBookingID, Status: status, PaymentIntentID: "pi_1"}
}

func TestBookingHandlePaymentEvent(t *testing.T) {
	boom := errors.New("boom")
	cancelled := time.Now()

	tests := []struct {
		name  string
		event entity.PaymentEvent
		setup func(m bookingMocks)
		// wantErr is what HandlePaymentEvent must return
		wantErr error
	}{
		{
			name:  "a successful payment confirms the booking",
			event: paymentEvent(entity.PaymentSucceeded),
			setup: func(m bookingMocks) {
				m.events.EXPECT().RecordEvent(mock.Anything, "evt_1", "payment_succeeded").
					Return(true, nil).Once()
				m.bookings.EXPECT().GetBookingByPayment(mock.Anything, "pi_1").
					Return(heldBooking(entity.StatusPendingPayment), nil).Once()
				m.bookings.EXPECT().ConfirmBooking(mock.Anything, heldBookingID).Return(nil).Once()
			},
		},
		{
			// The gateway delivers at least once, so the second copy must not
			// confirm anything again.
			name:  "an event already seen is dropped",
			event: paymentEvent(entity.PaymentSucceeded),
			setup: func(m bookingMocks) {
				m.events.EXPECT().RecordEvent(mock.Anything, "evt_1", "payment_succeeded").
					Return(false, nil).Once()
			},
		},
		{
			// Belt and braces: a different event id for a booking already
			// settled is still not confirmed twice.
			name:  "a booking already confirmed is left alone",
			event: paymentEvent(entity.PaymentSucceeded),
			setup: func(m bookingMocks) {
				m.events.EXPECT().RecordEvent(mock.Anything, mock.Anything, mock.Anything).
					Return(true, nil).Once()
				m.bookings.EXPECT().GetBookingByPayment(mock.Anything, "pi_1").
					Return(heldBooking(entity.StatusConfirmed), nil).Once()
			},
		},
		{
			// Confirming would hand out a slot that may belong to somebody
			// else by now.
			name:  "money for a released hold is refused rather than confirmed",
			event: paymentEvent(entity.PaymentSucceeded),
			setup: func(m bookingMocks) {
				booking := heldBooking(entity.StatusPendingPayment)
				booking.CancelledAt = &cancelled

				m.events.EXPECT().RecordEvent(mock.Anything, mock.Anything, mock.Anything).
					Return(true, nil).Once()
				m.bookings.EXPECT().GetBookingByPayment(mock.Anything, "pi_1").
					Return(booking, nil).Once()
			},
			wantErr: entity.ErrHoldLapsed,
		},
		{
			// The player is still at the form with time on the clock. Taking
			// the slot away now is what would strand a retry, so the event is
			// recorded and nothing else happens: no lookup, no cancel.
			name:  "a declined card leaves the hold alone",
			event: paymentEvent(entity.PaymentFailed),
			setup: func(m bookingMocks) {
				m.events.EXPECT().RecordEvent(mock.Anything, "evt_1", "payment_failed").
					Return(true, nil).Once()
			},
		},
		{
			// The whole point of leaving the hold: the second card confirms
			// the same booking the first one failed on.
			name:  "a second card after a decline confirms the booking",
			event: paymentEvent(entity.PaymentSucceeded),
			setup: func(m bookingMocks) {
				m.events.EXPECT().RecordEvent(mock.Anything, "evt_1", "payment_succeeded").
					Return(true, nil).Once()
				m.bookings.EXPECT().GetBookingByPayment(mock.Anything, "pi_1").
					Return(heldBooking(entity.StatusPendingPayment), nil).Once()
				m.bookings.EXPECT().ConfirmBooking(mock.Anything, heldBookingID).Return(nil).Once()
			},
		},
		{
			name:  "an unknown payment propagates",
			event: paymentEvent(entity.PaymentSucceeded),
			setup: func(m bookingMocks) {
				m.events.EXPECT().RecordEvent(mock.Anything, mock.Anything, mock.Anything).
					Return(true, nil).Once()
				m.bookings.EXPECT().GetBookingByPayment(mock.Anything, "pi_1").
					Return(nil, entity.ErrBookingNotFound).Once()
			},
			wantErr: entity.ErrBookingNotFound,
		},
		{
			// Nothing is acted on until the event is recorded, so a failure
			// here cannot half-handle it.
			name:  "a failure recording the event stops everything",
			event: paymentEvent(entity.PaymentSucceeded),
			setup: func(m bookingMocks) {
				m.events.EXPECT().RecordEvent(mock.Anything, mock.Anything, mock.Anything).
					Return(false, boom).Once()
			},
			wantErr: boom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newBookingMocks(t)
			tt.setup(m)

			err := m.svc().HandlePaymentEvent(context.Background(), tt.event)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestBookingResumePayment(t *testing.T) {
	boom := errors.New("boom")
	bookingID := uuid.New()

	// held is a slot this player still owes money on.
	held := func() *entity.Booking {
		expires := time.Now().Add(10 * time.Minute)
		return &entity.Booking{
			ID:              bookingID,
			PlayerID:        bookingPlayerID,
			CourtID:         bookingCourtID,
			Status:          entity.StatusPendingPayment,
			HoldExpiresAt:   &expires,
			PaymentIntentID: "pi_1",
		}
	}

	tests := []struct {
		name     string
		playerID uuid.UUID
		setup    func(m bookingMocks)
		wantErr  error
	}{
		{
			name:     "hands back the secret for a standing hold",
			playerID: bookingPlayerID,
			setup: func(m bookingMocks) {
				m.bookings.EXPECT().GetBooking(mock.Anything, bookingID).Return(held(), nil).Once()
				m.payments.EXPECT().GetPayment(mock.Anything, "pi_1").
					Return(&entity.Payment{IntentID: "pi_1", ClientSecret: "pi_1_secret"}, nil).Once()
			},
		},
		{
			// Not forbidden: whether it exists is not theirs to learn.
			name:     "somebody else's booking is not found",
			playerID: uuid.New(),
			setup: func(m bookingMocks) {
				m.bookings.EXPECT().GetBooking(mock.Anything, bookingID).Return(held(), nil).Once()
			},
			wantErr: entity.ErrBookingNotFound,
		},
		{
			name:     "an unknown booking is not found",
			playerID: bookingPlayerID,
			setup: func(m bookingMocks) {
				m.bookings.EXPECT().GetBooking(mock.Anything, bookingID).
					Return(nil, entity.ErrBookingNotFound).Once()
			},
			wantErr: entity.ErrBookingNotFound,
		},
		{
			name:     "a booking already paid for is refused",
			playerID: bookingPlayerID,
			setup: func(m bookingMocks) {
				booking := held()
				booking.Status = entity.StatusConfirmed
				m.bookings.EXPECT().GetBooking(mock.Anything, bookingID).Return(booking, nil).Once()
			},
			wantErr: entity.ErrAlreadyPaid,
		},
		{
			// No fresh payment is opened: the slot is gone.
			name:     "a lapsed hold is refused",
			playerID: bookingPlayerID,
			setup: func(m bookingMocks) {
				booking := held()
				expired := time.Now().Add(-time.Minute)
				booking.HoldExpiresAt = &expired
				m.bookings.EXPECT().GetBooking(mock.Anything, bookingID).Return(booking, nil).Once()
			},
			wantErr: entity.ErrHoldLapsed,
		},
		{
			name:     "a cancelled booking is refused",
			playerID: bookingPlayerID,
			setup: func(m bookingMocks) {
				booking := held()
				cancelled := time.Now()
				booking.CancelledAt = &cancelled
				m.bookings.EXPECT().GetBooking(mock.Anything, bookingID).Return(booking, nil).Once()
			},
			wantErr: entity.ErrHoldLapsed,
		},
		{
			name:     "an empty player is refused without a lookup",
			playerID: uuid.Nil,
			setup:    func(bookingMocks) {},
			wantErr:  entity.ErrPlayerRequired,
		},
		{
			name:     "propagates a gateway failure",
			playerID: bookingPlayerID,
			setup: func(m bookingMocks) {
				m.bookings.EXPECT().GetBooking(mock.Anything, bookingID).Return(held(), nil).Once()
				m.payments.EXPECT().GetPayment(mock.Anything, mock.Anything).Return(nil, boom).Once()
			},
			wantErr: boom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newBookingMocks(t)
			tt.setup(m)

			got, err := m.svc().ResumePayment(context.Background(), tt.playerID, bookingID)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, got)
				return
			}

			require.NoError(t, err)
			require.Equal(t, "pi_1_secret", got.ClientSecret)
			require.Equal(t, bookingID, got.Booking.ID)
		})
	}
}
