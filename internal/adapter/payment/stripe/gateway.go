// Package stripe collects payments through Stripe.
package stripe

import (
	"context"
	"fmt"
	"strings"

	stripeapi "github.com/stripe/stripe-go/v82"

	"github.com/thelong0705/deuce/internal/domain/entity"
	"github.com/thelong0705/deuce/internal/domain/usecase"
)

var _ usecase.PaymentGateway = (*Gateway)(nil)

type Gateway struct {
	client *stripeapi.Client
}

type Option func(*options)

type options struct {
	baseURL string
}

func WithBaseURL(url string) Option {
	return func(o *options) { o.baseURL = url }
}

func NewGateway(secretKey string, opts ...Option) *Gateway {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	if o.baseURL == "" {
		return &Gateway{client: stripeapi.NewClient(secretKey)}
	}

	backend := stripeapi.GetBackendWithConfig(stripeapi.APIBackend, &stripeapi.BackendConfig{
		URL: stripeapi.String(o.baseURL),
	})

	return &Gateway{client: stripeapi.NewClient(secretKey, stripeapi.WithBackends(&stripeapi.Backends{
		API:     backend,
		Uploads: backend,
		Connect: backend,
	}))}
}

// CreatePayment opens a PaymentIntent for a held slot. The booking id travels
// in the metadata so a payment can be traced back by hand; the webhook matches
// on the intent id the booking stores.
func (g *Gateway) CreatePayment(ctx context.Context, in entity.PaymentRequest) (*entity.Payment, error) {
	intent, err := g.client.V1PaymentIntents.Create(ctx, &stripeapi.PaymentIntentCreateParams{
		Amount:   stripeapi.Int64(int64(in.Amount)),
		Currency: stripeapi.String(currencyOf(in.Currency)),
		Metadata: map[string]string{"booking_id": in.BookingID.String()},
	})
	if err != nil {
		return nil, fmt.Errorf("create payment intent: %w", err)
	}

	return &entity.Payment{
		IntentID:     intent.ID,
		ClientSecret: intent.ClientSecret,
	}, nil
}

// GetPayment reads back an open payment so a player can finish one they
// started. The client secret is stored nowhere, so the gateway is the only
// place left to ask.
func (g *Gateway) GetPayment(ctx context.Context, intentID string) (*entity.Payment, error) {
	intent, err := g.client.V1PaymentIntents.Retrieve(ctx, intentID, nil)
	if err != nil {
		return nil, fmt.Errorf("retrieve payment intent: %w", err)
	}

	return &entity.Payment{
		IntentID:     intent.ID,
		ClientSecret: intent.ClientSecret,
	}, nil
}

// currencyOf is Stripe's spelling of a currency: lower case throughout.
func currencyOf(c entity.Currency) string {
	return strings.ToLower(string(c))
}
