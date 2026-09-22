package stripe

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

const signingSecret = "whsec_test_secret"

// sign builds the header Stripe sends: a timestamp and an HMAC over
// "timestamp.body" keyed with the signing secret.
func sign(t *testing.T, payload []byte, secret string, at time.Time) string {
	t.Helper()

	mac := hmac.New(sha256.New, []byte(secret))
	_, err := fmt.Fprintf(mac, "%d.%s", at.Unix(), payload)
	require.NoError(t, err)

	return fmt.Sprintf("t=%d,v1=%s", at.Unix(), hex.EncodeToString(mac.Sum(nil)))
}

func eventBody(id, eventType, intentID string) []byte {
	return fmt.Appendf(nil,
		`{"id":%q,"type":%q,"data":{"object":{"id":%q,"object":"payment_intent"}}}`,
		id, eventType, intentID)
}

func TestVerify(t *testing.T) {
	tests := []struct {
		name      string
		body      []byte
		wantEvent *entity.PaymentEvent
	}{
		{
			name: "a successful payment",
			body: eventBody("evt_1", "payment_intent.succeeded", "pi_1"),
			wantEvent: &entity.PaymentEvent{
				ID: "evt_1", Type: entity.PaymentSucceeded, IntentID: "pi_1",
			},
		},
		{
			name: "a failed payment",
			body: eventBody("evt_2", "payment_intent.payment_failed", "pi_2"),
			wantEvent: &entity.PaymentEvent{
				ID: "evt_2", Type: entity.PaymentFailed, IntentID: "pi_2",
			},
		},
		{
			// Signed and genuine, but about something the domain has no rules
			// for. Nothing to act on is not an error.
			name: "an event we do not act on",
			body: eventBody("evt_3", "charge.refunded", "pi_3"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewVerifier(signingSecret).
				Verify(tt.body, sign(t, tt.body, signingSecret, time.Now()))

			require.NoError(t, err)
			require.Equal(t, tt.wantEvent, got)
		})
	}
}

// The signature is what stands in for a session on this endpoint, so anything
// that does not carry a good one is refused.
func TestVerifyRefusesABadSignature(t *testing.T) {
	body := eventBody("evt_1", "payment_intent.succeeded", "pi_1")

	tests := []struct {
		name      string
		signature func(t *testing.T) string
	}{
		{
			name:      "no signature at all",
			signature: func(*testing.T) string { return "" },
		},
		{
			name: "signed with the wrong secret",
			signature: func(t *testing.T) string {
				return sign(t, body, "whsec_someone_else", time.Now())
			},
		},
		{
			name: "signed long enough ago to be a replay",
			signature: func(t *testing.T) string {
				return sign(t, body, signingSecret, time.Now().Add(-time.Hour))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewVerifier(signingSecret).Verify(body, tt.signature(t))

			require.Error(t, err)
			require.Nil(t, got)
		})
	}
}

// The signature covers the exact bytes that arrived, so a body changed in
// flight no longer matches.
func TestVerifyRefusesATamperedBody(t *testing.T) {
	body := eventBody("evt_1", "payment_intent.succeeded", "pi_1")
	signature := sign(t, body, signingSecret, time.Now())

	tampered := eventBody("evt_1", "payment_intent.succeeded", "pi_somebody_elses")

	got, err := NewVerifier(signingSecret).Verify(tampered, signature)

	require.Error(t, err)
	require.Nil(t, got)
}
