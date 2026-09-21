package entity

import (
	"net/mail"
	"time"

	"github.com/google/uuid"

	"github.com/thelong0705/deuce/internal/pkg/apperr"
)

// Session is a logged-in user's server-side session.
type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	UserAgent string
	ClientIP  string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// IsActive reports whether the session may still be used at the given time.
func (s Session) IsActive(now time.Time) bool {
	return now.Before(s.ExpiresAt)
}

var (
	// ErrInvalidCredentials covers both an unknown email and a wrong password,
	// so a caller cannot use login to discover which addresses are registered.
	ErrInvalidCredentials = apperr.New(apperr.KindUnauthorized, "invalid_credentials", "email or password is incorrect")
	ErrSessionInvalid     = apperr.New(apperr.KindUnauthorized, "session_invalid", "session is invalid or has expired")
	ErrPasswordRequired   = apperr.New(apperr.KindInvalid, "password_required", "password is required")
)

// LoginInput is what someone supplies to start a session.
type LoginInput struct {
	Email     string
	Password  string
	UserAgent string
	ClientIP  string
}

// Validate returns the first rule the input breaks.
func (in LoginInput) Validate() error {
	if _, err := mail.ParseAddress(in.Email); err != nil {
		return ErrInvalidEmail
	}
	if in.Password == "" {
		return ErrPasswordRequired
	}
	return nil
}
