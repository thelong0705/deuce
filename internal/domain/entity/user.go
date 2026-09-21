package entity

import (
	"net/mail"
	"regexp"
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
	ErrPhoneInvalid     = apperr.New(apperr.KindInvalid, "phone_invalid", "phone number must be in E.164 format, like +84901234567")
	ErrInvalidRole      = apperr.New(apperr.KindInvalid, "invalid_role", `role must be "player" or "owner"`)
	ErrEmailTaken       = apperr.New(apperr.KindConflict, "email_taken", "email already registered")
	ErrUserNotFound     = apperr.New(apperr.KindNotFound, "user_not_found", "user not found")
)

// e164 is the storage format for a phone number: a plus, a country code that
// cannot start with zero, and at most fifteen digits in total. It says nothing
// about whether the number is assigned or reachable, only that it is written
// the one way every part of the system can compare.
//
// Nothing is normalised here. A number with spaces or a local trunk zero is
// rejected rather than repaired, so the caller that knows which country the
// digits came from is the one that has to say.
var e164 = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)

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
	if !e164.MatchString(r.PhoneNumber) {
		return ErrPhoneInvalid
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
