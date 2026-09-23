//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

const (
	city          = "Ha Noi"
	timezone      = "Asia/Ho_Chi_Minh"
	password      = "e2e-password-1234"
	webhookSecret = "whsec_e2e"
)

var (
	baseURL string
	pool    *pgxpool.Pool
)

func TestMain(m *testing.M) {
	baseURL = envOr("E2E_BASE_URL", "http://localhost:18080")
	dsn := envOr("E2E_DB_URL", "postgres://deuce:deuce@localhost:15432/deuce?sslmode=disable")

	if err := waitForHealth(90 * time.Second); err != nil {
		log.Fatalf("api never became healthy at %s: %v", baseURL, err)
	}

	ctx := context.Background()

	var err error
	if pool, err = pgxpool.New(ctx, dsn); err != nil {
		log.Fatalf("parse db config: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("connect to db at %s: %v", dsn, err)
	}

	code := m.Run()

	pool.Close()
	os.Exit(code)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}

func waitForHealth(within time.Duration) error {
	deadline := time.Now().Add(within)

	var lastErr error

	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/healthz")
		if err == nil {
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				return nil
			}

			lastErr = fmt.Errorf("status %d", resp.StatusCode)
		} else {
			lastErr = err
		}

		time.Sleep(250 * time.Millisecond)
	}

	return lastErr
}

type client struct {
	t    *testing.T
	http *http.Client
}

func newClient(t *testing.T) *client {
	t.Helper()

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	return &client{t: t, http: &http.Client{Jar: jar, Timeout: 30 * time.Second}}
}

func (c *client) do(method, path string, body any) (int, []byte) {
	c.t.Helper()

	var reader io.Reader

	if body != nil {
		encoded, err := json.Marshal(body)
		require.NoError(c.t, err)

		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, baseURL+path, reader)
	require.NoError(c.t, err)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	require.NoError(c.t, err)

	defer resp.Body.Close()

	got, err := io.ReadAll(resp.Body)
	require.NoError(c.t, err)

	return resp.StatusCode, got
}

func (c *client) expect(want int, method, path string, body, into any) {
	c.t.Helper()

	status, got := c.do(method, path, body)
	require.Equal(c.t, want, status, "%s %s: %s", method, path, got)

	if into != nil {
		require.NoError(c.t, json.Unmarshal(got, into), "decoding %s", got)
	}
}

type user struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
}

func (c *client) signUpAndLogIn(role string) user {
	c.t.Helper()

	var u user

	c.expect(http.StatusCreated, http.MethodPost, "/users", map[string]string{
		"email":        "e2e-" + uuid.NewString() + "@example.com",
		"password":     password,
		"phone_number": phoneNumber(),
		"role":         role,
	}, &u)

	c.expect(http.StatusCreated, http.MethodPost, "/sessions", map[string]string{
		"email":    u.Email,
		"password": password,
	}, nil)

	return u
}

type venue struct {
	ID uuid.UUID `json:"id"`
}

func (c *client) createVenue() venue {
	c.t.Helper()

	var got venue

	c.expect(http.StatusCreated, http.MethodPost, "/venues", map[string]string{
		"name":    "Court Club " + uuid.NewString()[:8],
		"city":    city,
		"address": "1 Test Street",
	}, &got)

	return got
}

type court struct {
	ID uuid.UUID `json:"id"`
}

func (c *client) createCourt(v venue) court {
	c.t.Helper()

	var got court

	c.expect(http.StatusCreated, http.MethodPost, "/venues/"+v.ID.String()+"/courts", map[string]any{
		"name":           "Court 1",
		"open_hour":      6,
		"close_hour":     22,
		"price_per_hour": 100000,
		"currency":       "VND",
	}, &got)

	return got
}

type booking struct {
	ID           uuid.UUID `json:"id"`
	Status       string    `json:"status"`
	StartsAt     time.Time `json:"starts_at"`
	ClientSecret string    `json:"client_secret"`
}

type slot struct {
	StartsAt  time.Time `json:"starts_at"`
	Available bool      `json:"available"`
}

type availability struct {
	Slots []slot `json:"slots"`
}

func (c *client) availabilityOn(ct court, day time.Time) availability {
	c.t.Helper()

	var got availability

	c.expect(http.StatusOK, http.MethodGet,
		"/courts/"+ct.ID.String()+"/availability?date="+day.Format(time.DateOnly), nil, &got)

	return got
}

func freeSlot(t *testing.T) time.Time {
	t.Helper()

	loc, err := time.LoadLocation(timezone)
	require.NoError(t, err)

	tomorrow := time.Now().In(loc).AddDate(0, 0, 1)

	return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 8, 0, 0, 0, loc)
}

func phoneNumber() string {
	return fmt.Sprintf("+84%013d", time.Now().UnixNano()%1e13)
}

func (c *client) book(ct court, at time.Time) booking {
	c.t.Helper()

	var held booking

	c.expect(http.StatusCreated, http.MethodPost, "/courts/"+ct.ID.String()+"/bookings",
		map[string]string{"starts_at": at.Format(time.RFC3339)}, &held)

	return held
}

func (c *client) bookings() []booking {
	c.t.Helper()

	var got struct {
		Bookings []booking `json:"bookings"`
	}

	c.expect(http.StatusOK, http.MethodGet, "/bookings", nil, &got)

	return got.Bookings
}

func paymentIntentOf(t *testing.T, bookingID uuid.UUID) string {
	t.Helper()

	var intentID *string

	err := pool.QueryRow(context.Background(),
		"SELECT payment_intent_id FROM bookings WHERE id = $1", bookingID).Scan(&intentID)
	require.NoError(t, err)
	require.NotNil(t, intentID, "the booking has no payment intent")

	return *intentID
}

func deliverWebhook(t *testing.T, eventType, intentID string) (int, []byte) {
	t.Helper()

	payload := fmt.Appendf(nil,
		`{"id":%q,"type":%q,"data":{"object":{"id":%q,"object":"payment_intent"}}}`,
		"evt_"+uuid.NewString(), eventType, intentID)

	at := time.Now()

	mac := hmac.New(sha256.New, []byte(webhookSecret))
	_, err := fmt.Fprintf(mac, "%d.%s", at.Unix(), payload)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, baseURL+"/stripe/webhook", bytes.NewReader(payload))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Stripe-Signature",
		fmt.Sprintf("t=%d,v1=%s", at.Unix(), hex.EncodeToString(mac.Sum(nil))))

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp.StatusCode, body
}
