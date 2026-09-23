package httpapi

import (
	"net/http"
	"slices"
	"strconv"
	"time"
)

const preflightMaxAge = 2 * time.Hour

func CORS(origins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if len(origins) == 0 {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			w.Header().Add("Vary", "Origin")

			if origin == "" || !slices.Contains(origins, origin) {
				next.ServeHTTP(w, r)
				return
			}

			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")

			if r.Method != http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			h.Add("Vary", "Access-Control-Request-Method")
			h.Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Content-Type")
			h.Set("Access-Control-Max-Age", strconv.Itoa(int(preflightMaxAge.Seconds())))

			w.WriteHeader(http.StatusNoContent)
		})
	}
}
