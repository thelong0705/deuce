package httpapi_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/adapter/httpapi/mocks"
	"github.com/thelong0705/deuce/internal/domain/entity"
)

func succeededEvent() *entity.PaymentEvent {
	return &entity.PaymentEvent{
		ID: "evt_1", Type: entity.PaymentSucceeded, IntentID: "pi_1",
	}
}

func TestPaymentWebhook(t *testing.T) {
	boom := errors.New("boom")

	tests := []struct {
		name  string
		setup func(webhooks *mocks.MockPaymentWebhook, bookings *mocks.MockBookingUsecase)
		// wantStatus is what the gateway is told
		wantStatus int
	}{
		{
			name: "a verified event is handled",
			setup: func(webhooks *mocks.MockPaymentWebhook, bookings *mocks.MockBookingUsecase) {
				webhooks.EXPECT().Verify(mock.Anything, "t=1,v1=abc").
					Return(succeededEvent(), nil).Once()
				bookings.EXPECT().HandlePaymentEvent(mock.Anything, *succeededEvent()).
					Return(nil).Once()
			},
			wantStatus: http.StatusNoContent,
		},
		{
			// Signed and genuine, but about something we do not act on.
			name: "an event we do not act on is accepted",
			setup: func(webhooks *mocks.MockPaymentWebhook, bookings *mocks.MockBookingUsecase) {
				webhooks.EXPECT().Verify(mock.Anything, mock.Anything).Return(nil, nil).Once()
			},
			wantStatus: http.StatusNoContent,
		},
		{
			// The signature is what stands in for a session here.
			name: "an unsigned body is refused before the use case",
			setup: func(webhooks *mocks.MockPaymentWebhook, bookings *mocks.MockBookingUsecase) {
				webhooks.EXPECT().Verify(mock.Anything, mock.Anything).
					Return(nil, errors.New("no signature")).Once()
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			// Retrying would reach the same conclusion, so the delivery ends
			// rather than repeating for ever.
			name: "money for a released hold ends the delivery",
			setup: func(webhooks *mocks.MockPaymentWebhook, bookings *mocks.MockBookingUsecase) {
				webhooks.EXPECT().Verify(mock.Anything, mock.Anything).
					Return(succeededEvent(), nil).Once()
				bookings.EXPECT().HandlePaymentEvent(mock.Anything, mock.Anything).
					Return(entity.ErrHoldLapsed).Once()
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "a payment for an unknown booking ends the delivery",
			setup: func(webhooks *mocks.MockPaymentWebhook, bookings *mocks.MockBookingUsecase) {
				webhooks.EXPECT().Verify(mock.Anything, mock.Anything).
					Return(succeededEvent(), nil).Once()
				bookings.EXPECT().HandlePaymentEvent(mock.Anything, mock.Anything).
					Return(entity.ErrBookingNotFound).Once()
			},
			wantStatus: http.StatusNoContent,
		},
		{
			// A database that is down may come good, so the gateway is asked
			// to try again.
			name: "an unexpected failure asks for a retry",
			setup: func(webhooks *mocks.MockPaymentWebhook, bookings *mocks.MockBookingUsecase) {
				webhooks.EXPECT().Verify(mock.Anything, mock.Anything).
					Return(succeededEvent(), nil).Once()
				bookings.EXPECT().HandlePaymentEvent(mock.Anything, mock.Anything).
					Return(boom).Once()
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webhooks := mocks.NewMockPaymentWebhook(t)
			bookings := mocks.NewMockBookingUsecase(t)
			tt.setup(webhooks, bookings)

			req := newRequest(http.MethodPost, "/stripe/webhook", `{"id":"evt_1"}`)
			req.Header.Set("Stripe-Signature", "t=1,v1=abc")

			rec := send(t, deps{webhooks: webhooks, bookings: bookings}, req)

			require.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

// The gateway has no session, so the route must not sit behind requireAuth.
func TestPaymentWebhookNeedsNoSession(t *testing.T) {
	webhooks := mocks.NewMockPaymentWebhook(t)
	bookings := mocks.NewMockBookingUsecase(t)

	webhooks.EXPECT().Verify(mock.Anything, mock.Anything).Return(succeededEvent(), nil).Once()
	bookings.EXPECT().HandlePaymentEvent(mock.Anything, mock.Anything).Return(nil).Once()

	rec := do(t, deps{webhooks: webhooks, bookings: bookings},
		http.MethodPost, "/stripe/webhook", `{"id":"evt_1"}`)

	require.Equal(t, http.StatusNoContent, rec.Code)
}

// The signature covers the exact bytes that arrived, so the body is handed
// over untouched.
func TestPaymentWebhookPassesTheRawBody(t *testing.T) {
	const body = `{"id":"evt_1","type":"payment_intent.succeeded"}`

	webhooks := mocks.NewMockPaymentWebhook(t)
	bookings := mocks.NewMockBookingUsecase(t)

	webhooks.EXPECT().
		Verify(mock.MatchedBy(func(payload []byte) bool {
			return string(payload) == body
		}), mock.Anything).
		Return(succeededEvent(), nil).
		Once()
	bookings.EXPECT().HandlePaymentEvent(mock.Anything, mock.Anything).Return(nil).Once()

	rec := do(t, deps{webhooks: webhooks, bookings: bookings},
		http.MethodPost, "/stripe/webhook", body)

	require.Equal(t, http.StatusNoContent, rec.Code)
}
