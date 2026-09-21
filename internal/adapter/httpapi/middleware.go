package httpapi

import (
	"context"
	"net/http"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

type contextKey struct{}

var userContextKey contextKey

// requireAuth rejects a request whose session cookie is missing, unknown or
// expired, and puts the authenticated user in the request context.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := s.users.Authenticate(r.Context(), sessionToken(r))
		if err != nil {
			writeAppError(w, err)
			return
		}

		next.ServeHTTP(w, r.WithContext(withUser(r.Context(), user)))
	})
}

func withUser(ctx context.Context, user *entity.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// UserFromContext returns the user put there by requireAuth.
func UserFromContext(ctx context.Context) (*entity.User, bool) {
	user, ok := ctx.Value(userContextKey).(*entity.User)
	return user, ok
}
