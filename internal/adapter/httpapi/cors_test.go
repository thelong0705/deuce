package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/adapter/httpapi"
)

const allowedOrigin = "https://thelong0705.dev"

func corsHandler(reached *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		*reached = true
		w.WriteHeader(http.StatusOK)
	})
}

func TestCORS(t *testing.T) {
	tests := []struct {
		name        string
		origins     []string
		method      string
		origin      string
		wantStatus  int
		wantReached bool
		wantOrigin  string
		wantCreds   string
	}{
		{
			name:        "allows a configured origin",
			origins:     []string{allowedOrigin},
			method:      http.MethodGet,
			origin:      allowedOrigin,
			wantStatus:  http.StatusOK,
			wantReached: true,
			wantOrigin:  allowedOrigin,
			wantCreds:   "true",
		},
		{
			name:        "sends no headers for an origin that is not listed",
			origins:     []string{allowedOrigin},
			method:      http.MethodGet,
			origin:      "https://evil.example",
			wantStatus:  http.StatusOK,
			wantReached: true,
		},
		{
			name:        "does not match on host alone",
			origins:     []string{allowedOrigin},
			method:      http.MethodGet,
			origin:      "http://thelong0705.dev",
			wantStatus:  http.StatusOK,
			wantReached: true,
		},
		{
			name:        "answers a preflight without reaching the handler",
			origins:     []string{allowedOrigin},
			method:      http.MethodOptions,
			origin:      allowedOrigin,
			wantStatus:  http.StatusNoContent,
			wantReached: false,
			wantOrigin:  allowedOrigin,
			wantCreds:   "true",
		},
		{
			name:        "passes through when no origins are configured",
			origins:     nil,
			method:      http.MethodGet,
			origin:      allowedOrigin,
			wantStatus:  http.StatusOK,
			wantReached: true,
		},
		{
			name:        "a request with no Origin is untouched",
			origins:     []string{allowedOrigin},
			method:      http.MethodGet,
			wantStatus:  http.StatusOK,
			wantReached: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reached bool

			req := httptest.NewRequest(tt.method, "/cities", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			rec := httptest.NewRecorder()
			httpapi.CORS(tt.origins)(corsHandler(&reached)).ServeHTTP(rec, req)

			require.Equal(t, tt.wantStatus, rec.Code)
			require.Equal(t, tt.wantReached, reached)
			require.Equal(t, tt.wantOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
			require.Equal(t, tt.wantCreds, rec.Header().Get("Access-Control-Allow-Credentials"))

			require.NotEqual(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
		})
	}
}

func TestCORSVariesOnOrigin(t *testing.T) {
	var reached bool

	req := httptest.NewRequest(http.MethodGet, "/cities", nil)
	req.Header.Set("Origin", "https://evil.example")

	rec := httptest.NewRecorder()
	httpapi.CORS([]string{allowedOrigin})(corsHandler(&reached)).ServeHTTP(rec, req)

	require.Contains(t, rec.Header().Values("Vary"), "Origin")
}

func TestCORSPreflightAdvertisesWhatTheAPIUses(t *testing.T) {
	var reached bool

	req := httptest.NewRequest(http.MethodOptions, "/sessions", nil)
	req.Header.Set("Origin", allowedOrigin)

	rec := httptest.NewRecorder()
	httpapi.CORS([]string{allowedOrigin})(corsHandler(&reached)).ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), http.MethodDelete)
	require.Equal(t, "Content-Type", rec.Header().Get("Access-Control-Allow-Headers"))
	require.NotEmpty(t, rec.Header().Get("Access-Control-Max-Age"))
}
