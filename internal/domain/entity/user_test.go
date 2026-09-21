package entity_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

func validInput() entity.CreateUserInput {
	return entity.CreateUserInput{
		Email:       "alice@example.com",
		Password:    "supersecret",
		PhoneNumber: "+84901234567",
		Role:        entity.RolePlayer,
	}
}

func TestCreateUserInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(r *entity.CreateUserInput)
		wantErr error
	}{
		{
			name:   "valid player",
			mutate: func(*entity.CreateUserInput) {},
		},
		{
			name:   "valid owner",
			mutate: func(r *entity.CreateUserInput) { r.Role = entity.RoleOwner },
		},
		{
			name:   "one-digit country code",
			mutate: func(r *entity.CreateUserInput) { r.PhoneNumber = "+15551234567" },
		},
		{
			name:   "the full fifteen digits E.164 allows",
			mutate: func(r *entity.CreateUserInput) { r.PhoneNumber = "+849012345678901" },
		},
		{
			name:   "the shortest E.164 allows",
			mutate: func(r *entity.CreateUserInput) { r.PhoneNumber = "+84" },
		},
		{
			name:    "empty email",
			mutate:  func(r *entity.CreateUserInput) { r.Email = "" },
			wantErr: entity.ErrInvalidEmail,
		},
		{
			name:    "email without an @",
			mutate:  func(r *entity.CreateUserInput) { r.Email = "alice.example.com" },
			wantErr: entity.ErrInvalidEmail,
		},
		{
			name:    "password one byte too short",
			mutate:  func(r *entity.CreateUserInput) { r.Password = strings.Repeat("a", 7) },
			wantErr: entity.ErrPasswordTooShort,
		},
		{
			name:   "password at the minimum",
			mutate: func(r *entity.CreateUserInput) { r.Password = strings.Repeat("a", 8) },
		},
		{
			name:   "password at the bcrypt limit",
			mutate: func(r *entity.CreateUserInput) { r.Password = strings.Repeat("a", 72) },
		},
		{
			name:    "password one byte past the bcrypt limit",
			mutate:  func(r *entity.CreateUserInput) { r.Password = strings.Repeat("a", 73) },
			wantErr: entity.ErrPasswordTooLong,
		},
		{
			name:    "missing phone",
			mutate:  func(r *entity.CreateUserInput) { r.PhoneNumber = "" },
			wantErr: entity.ErrPhoneRequired,
		},
		{
			name:    "phone that is only whitespace",
			mutate:  func(r *entity.CreateUserInput) { r.PhoneNumber = "   " },
			wantErr: entity.ErrPhoneRequired,
		},
		{
			name:    "phone without a plus",
			mutate:  func(r *entity.CreateUserInput) { r.PhoneNumber = "84901234567" },
			wantErr: entity.ErrPhoneInvalid,
		},
		{
			name:    "phone written the local way, with a trunk zero",
			mutate:  func(r *entity.CreateUserInput) { r.PhoneNumber = "0901234567" },
			wantErr: entity.ErrPhoneInvalid,
		},
		{
			// Nothing normalises, so a number that is only readable is refused.
			name:    "phone with spaces",
			mutate:  func(r *entity.CreateUserInput) { r.PhoneNumber = "+84 90 123 4567" },
			wantErr: entity.ErrPhoneInvalid,
		},
		{
			name:    "phone with a country code starting at zero",
			mutate:  func(r *entity.CreateUserInput) { r.PhoneNumber = "+0901234567" },
			wantErr: entity.ErrPhoneInvalid,
		},
		{
			name:    "phone of letters",
			mutate:  func(r *entity.CreateUserInput) { r.PhoneNumber = "+84call-me" },
			wantErr: entity.ErrPhoneInvalid,
		},
		{
			name:    "phone longer than E.164 allows",
			mutate:  func(r *entity.CreateUserInput) { r.PhoneNumber = "+8490123456789012" },
			wantErr: entity.ErrPhoneInvalid,
		},
		{
			name:    "phone of a plus and one digit",
			mutate:  func(r *entity.CreateUserInput) { r.PhoneNumber = "+8" },
			wantErr: entity.ErrPhoneInvalid,
		},
		{
			name:    "unknown role",
			mutate:  func(r *entity.CreateUserInput) { r.Role = "admin" },
			wantErr: entity.ErrInvalidRole,
		},
		{
			name:    "empty role",
			mutate:  func(r *entity.CreateUserInput) { r.Role = "" },
			wantErr: entity.ErrInvalidRole,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := validInput()
			tt.mutate(&reg)

			err := reg.Validate()

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestCreateUserInputDisplayName(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  string
	}{
		{"local part", "alice@example.com", "alice"},
		{"dots and tags are kept", "a.b+tennis@example.com", "a.b+tennis"},
		{"no at sign falls back to the whole string", "noatsign", "noatsign"},
		{"empty local part falls back", "@example.com", "@example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := validInput()
			reg.Email = tt.email

			require.Equal(t, tt.want, reg.DisplayName())
		})
	}
}

func TestRoleValid(t *testing.T) {
	tests := []struct {
		role entity.Role
		want bool
	}{
		{entity.RolePlayer, true},
		{entity.RoleOwner, true},
		{"admin", false},
		{"", false},
		{"Player", false}, // case matters; it maps to a Postgres enum
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			require.Equal(t, tt.want, tt.role.Valid())
		})
	}
}
