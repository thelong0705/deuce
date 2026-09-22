package stripe

import (
	"encoding/json"
	"fmt"

	stripeapi "github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

// Verifier turns a signed webhook body into a payment event. It is separate
// from the Gateway because it holds a different secret and is used by a
// different caller.
type Verifier struct {
	signingSecret string
}

func NewVerifier(signingSecret string) *Verifier {
	return &Verifier{signingSecret: signingSecret}
}

// Verify checks the signature over the raw body and reads the event out of it.
// The body must be exactly what arrived: the signature covers those bytes, so
// decoding and re-encoding first would break it.
//
// An event about something we do not act on comes back nil, nil.
func (v *Verifier) Verify(payload []byte, signature string) (*entity.PaymentEvent, error) {
	// The account's API version is set in Stripe's dashboard and need not match
	// the one this SDK was built against. Refusing on that would reject every
	// genuine delivery; the three fields read below are stable across
	// versions.
	event, err := webhook.ConstructEventWithOptions(payload, signature, v.signingSecret,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true})
	if err != nil {
		return nil, fmt.Errorf("verify webhook signature: %w", err)
	}

	eventType, ok := paymentEventType(event.Type)
	if !ok {
		return nil, nil
	}

	var intent stripeapi.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &intent); err != nil {
		return nil, fmt.Errorf("read payment intent from %s: %w", event.Type, err)
	}

	return &entity.PaymentEvent{
		ID:       event.ID,
		Type:     eventType,
		IntentID: intent.ID,
	}, nil
}

func paymentEventType(t stripeapi.EventType) (entity.PaymentEventType, bool) {
	switch t {
	case stripeapi.EventTypePaymentIntentSucceeded:
		return entity.PaymentSucceeded, true
	case stripeapi.EventTypePaymentIntentPaymentFailed:
		return entity.PaymentFailed, true
	default:
		return "", false
	}
}
