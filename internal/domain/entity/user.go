package entity

import (
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
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
	ErrInvalidEmail     = errors.New("email must be a valid address")
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	ErrPasswordTooLong  = errors.New("password must be at most 72 bytes")
	ErrPhoneRequired    = errors.New("phone number is required")
	ErrInvalidRole      = errors.New(`role must be "player" or "owner"`)
	ErrEmailTaken       = errors.New("email already registered")
)

// Registration is what someone supplies to sign up.
type Registration struct {
	Email       string
	Password    string
	PhoneNumber string
	Role        Role
}

// Validate reports the first rule the registration breaks.
func (r Registration) Validate() error {
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
func (r Registration) DisplayName() string {
	local, _, found := strings.Cut(r.Email, "@")
	if !found || strings.TrimSpace(local) == "" {
		return r.Email
	}
	return local
}
