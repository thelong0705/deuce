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

func NewGateway(secretKey string) *Gateway {
	return &Gateway{client: stripeapi.NewClient(secretKey)}
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

// currencyOf is Stripe's spelling of a currency: lower case throughout.
func currencyOf(c entity.Currency) string {
	return strings.ToLower(string(c))
}
