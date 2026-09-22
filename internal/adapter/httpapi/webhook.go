package httpapi

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

// maxWebhookBody caps what is read from an unauthenticated endpoint. Stripe's
// events are a few kilobytes.
const maxWebhookBody = 256 << 10

// PaymentWebhook turns a signed request body into a payment event. A body it
// verifies but does not care about comes back nil, nil.
type PaymentWebhook interface {
	Verify(payload []byte, signature string) (*entity.PaymentEvent, error)
}

// paymentWebhook is the only unauthenticated route that changes anything. The
// signature is what stands in for a session: without a valid one the body is
// refused before it reaches the use case.
func (s *Server) paymentWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBody))
	if err != nil {
		writeBody(w, http.StatusBadRequest, invalidBodyResponse)
		return
	}

	event, err := s.webhooks.Verify(payload, r.Header.Get("Stripe-Signature"))
	if err != nil {
		slog.Warn("rejected webhook", "error", err)
		writeBody(w, http.StatusBadRequest, invalidBodyResponse)
		return
	}

	// Verified, but about something we do not act on. Saying so ends the
	// delivery rather than inviting a retry.
	if event == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch err := s.bookings.HandlePaymentEvent(r.Context(), *event); {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)

	// Both of these are settled: the gateway retrying would reach the same
	// conclusion, so the delivery ends here and a person has to look.
	case errors.Is(err, entity.ErrHoldLapsed):
		slog.Error("paid for a slot that was already released",
			"event", event.ID, "payment_intent", event.IntentID)
		w.WriteHeader(http.StatusNoContent)

	case errors.Is(err, entity.ErrBookingNotFound):
		slog.Error("payment for an unknown booking",
			"event", event.ID, "payment_intent", event.IntentID)
		w.WriteHeader(http.StatusNoContent)

	default:
		// Anything else may come good, so the gateway is asked to try again.
		slog.Error("handle payment event", "event", event.ID, "error", err)
		writeInternalError(w)
	}
}
