package stripe

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	stripeapi "github.com/stripe/stripe-go/v82"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

// newTestGateway points a gateway at a stand-in for Stripe, so what is sent
// and what is made of the reply are both testable without a key.
func newTestGateway(t *testing.T, handler http.HandlerFunc) *Gateway {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	backend := stripeapi.GetBackendWithConfig(stripeapi.APIBackend, &stripeapi.BackendConfig{
		URL:           stripeapi.String(server.URL),
		LeveledLogger: &stripeapi.LeveledLogger{Level: stripeapi.LevelNull},
	})

	return &Gateway{
		client: stripeapi.NewClient("sk_test_fake", stripeapi.WithBackends(&stripeapi.Backends{
			API:     backend,
			Uploads: backend,
			Connect: backend,
		})),
	}
}

func TestCreatePayment(t *testing.T) {
	bookingID := uuid.New()

	var got url.Values

	gateway := newTestGateway(t, func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		got, err = url.ParseQuery(string(body))
		require.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"pi_123","object":"payment_intent","client_secret":"pi_123_secret_abc"}`))
	})

	payment, err := gateway.CreatePayment(context.Background(), entity.PaymentRequest{
		BookingID: bookingID,
		Amount:    240000,
		Currency:  entity.CurrencyVND,
	})

	require.NoError(t, err)
	require.Equal(t, "pi_123", payment.IntentID)
	require.Equal(t, "pi_123_secret_abc", payment.ClientSecret)

	require.Equal(t, "240000", got.Get("amount"))
	// Stripe spells currencies in lower case.
	require.Equal(t, "vnd", got.Get("currency"))
	// The booking travels along so a payment can be traced back by hand.
	require.Equal(t, bookingID.String(), got.Get("metadata[booking_id]"))
}

// A refusal from Stripe reaches the caller rather than being reported as a
// payment that never started.
func TestCreatePaymentPropagatesAFailure(t *testing.T) {
	gateway := newTestGateway(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","message":"amount must be positive"}}`))
	})

	payment, err := gateway.CreatePayment(context.Background(), entity.PaymentRequest{
		BookingID: uuid.New(),
		Amount:    -1,
		Currency:  entity.CurrencyVND,
	})

	require.Error(t, err)
	require.Nil(t, payment)
	require.Contains(t, err.Error(), "create payment intent")
}

func TestGetPayment(t *testing.T) {
	t.Run("reads back the secret for an open payment", func(t *testing.T) {
		var gotPath, gotMethod string

		gateway := newTestGateway(t, func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":"pi_1","client_secret":"pi_1_secret_abc"}`)
		})

		payment, err := gateway.GetPayment(context.Background(), "pi_1")
		require.NoError(t, err)

		require.Equal(t, "pi_1", payment.IntentID)
		require.Equal(t, "pi_1_secret_abc", payment.ClientSecret)

		// Reading an existing payment, not opening another one.
		require.Equal(t, http.MethodGet, gotMethod)
		require.Equal(t, "/v1/payment_intents/pi_1", gotPath)
	})

	t.Run("a gateway failure is wrapped", func(t *testing.T) {
		gateway := newTestGateway(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"error":{"message":"boom"}}`)
		})

		payment, err := gateway.GetPayment(context.Background(), "pi_1")

		require.Error(t, err)
		require.Nil(t, payment)
		require.Contains(t, err.Error(), "retrieve payment intent")
	})
}

func TestNewGatewayWithBaseURL(t *testing.T) {
	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"pi_1","client_secret":"pi_1_secret_abc"}`)
	}))
	t.Cleanup(server.Close)

	gateway := NewGateway("sk_test_fake", WithBaseURL(server.URL))

	payment, err := gateway.CreatePayment(context.Background(), entity.PaymentRequest{
		BookingID: uuid.New(),
		Amount:    1000,
		Currency:  entity.CurrencyVND,
	})

	require.NoError(t, err)
	require.Equal(t, "pi_1", payment.IntentID)
	require.Equal(t, "/v1/payment_intents", gotPath)
}

func TestNewGatewayWithoutBaseURL(t *testing.T) {
	require.NotNil(t, NewGateway("sk_test_fake", WithBaseURL("")))
	require.NotNil(t, NewGateway("sk_test_fake"))
}
