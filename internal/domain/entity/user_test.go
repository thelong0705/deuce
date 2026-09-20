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
