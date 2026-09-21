package entity

import (
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/thelong0705/deuce/internal/pkg/apperr"
)

type Role string

const (
	RolePlayer Role = "player"
	RoleOwner  Role = "owner"
)

func (r Role) Valid() bool {
	return r == RolePlayer || r == RoleOwner
}

func (r Role) String() string { return string(r) }

type User struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
	PhoneNumber string
	Role        Role
	IsActive    bool
	CreatedAt   time.Time
}

const (
	MinPasswordBytes = 8
	MaxPasswordBytes = 72
)

var (
	ErrInvalidEmail     = apperr.New(apperr.KindInvalid, "invalid_email", "email must be a valid address")
	ErrPasswordTooShort = apperr.New(apperr.KindInvalid, "password_too_short", "password must be at least 8 characters")
	ErrPasswordTooLong  = apperr.New(apperr.KindInvalid, "password_too_long", "password must be at most 72 bytes")
	ErrPhoneRequired    = apperr.New(apperr.KindInvalid, "phone_required", "phone number is required")
	ErrInvalidRole      = apperr.New(apperr.KindInvalid, "invalid_role", `role must be "player" or "owner"`)
	ErrEmailTaken       = apperr.New(apperr.KindConflict, "email_taken", "email already registered")
)

// CreateUserInput is what someone supplies to sign up.
type CreateUserInput struct {
	Email       string
	Password    string
	PhoneNumber string
	Role        Role
}

// Validate reports the first rule the input breaks.
func (r CreateUserInput) Validate() error {
	if _, err := mail.ParseAddress(r.Email); err != nil {
		return ErrInvalidEmail
	}
	if len(r.Password) < MinPasswordBytes {
		return ErrPasswordTooShort
	}
	if len(r.Password) > MaxPasswordBytes {
		return ErrPasswordTooLong
	}
	if strings.TrimSpace(r.PhoneNumber) == "" {
		return ErrPhoneRequired
	}
	if !r.Role.Valid() {
		return ErrInvalidRole
	}
	return nil
}

// DisplayName is seeded from the email's local part so signup stays short.
// Users set a real one later.
func (r CreateUserInput) DisplayName() string {
	local, _, found := strings.Cut(r.Email, "@")
	if !found || strings.TrimSpace(local) == "" {
		return r.Email
	}
	return local
}
