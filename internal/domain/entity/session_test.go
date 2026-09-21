package entity_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/domain/entity"
	"github.com/thelong0705/deuce/internal/pkg/apperr"
)

func TestSessionIsActive(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		session entity.Session
		want    bool
	}{
		{
			name:    "not expired",
			session: entity.Session{ExpiresAt: now.Add(time.Hour)},
			want:    true,
		},
		{
			name:    "expired",
			session: entity.Session{ExpiresAt: now.Add(-time.Second)},
		},
		{
			name:    "expiring exactly now counts as expired",
			session: entity.Session{ExpiresAt: now},
		},
		{
			name:    "zero session is not active",
			session: entity.Session{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.session.IsActive(now))
		})
	}
}

func TestLoginInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		in      entity.LoginInput
		wantErr error
	}{
		{
			name: "valid",
			in:   entity.LoginInput{Email: "alice@example.com", Password: "supersecret"},
		},
		{
			name:    "empty email",
			in:      entity.LoginInput{Password: "supersecret"},
			wantErr: entity.ErrInvalidEmail,
		},
		{
			name:    "malformed email",
			in:      entity.LoginInput{Email: "nope", Password: "supersecret"},
			wantErr: entity.ErrInvalidEmail,
		},
		{
			name:    "empty password",
			in:      entity.LoginInput{Email: "alice@example.com"},
			wantErr: entity.ErrPasswordRequired,
		},
		{
			name: "a short password is accepted for checking",
			in:   entity.LoginInput{Email: "alice@example.com", Password: "x"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.Validate()

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestSessionErrorKinds(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want apperr.Kind
	}{
		{"invalid credentials", entity.ErrInvalidCredentials, apperr.KindUnauthorized},
		{"session invalid", entity.ErrSessionInvalid, apperr.KindUnauthorized},
		{"password required", entity.ErrPasswordRequired, apperr.KindInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, apperr.KindOf(tt.err))
		})
	}
}
