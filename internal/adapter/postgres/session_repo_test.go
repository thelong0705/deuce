package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

func insertSession(t *testing.T, repo *SessionRepository, userID uuid.UUID, tokenHash string, expiresAt time.Time) *entity.Session {
	t.Helper()

	session, err := repo.CreateSession(context.Background(), entity.Session{
		UserID:    userID,
		UserAgent: "curl/8",
		ClientIP:  "10.0.0.1",
		ExpiresAt: expiresAt,
	}, tokenHash)
	require.NoError(t, err)

	return session
}

func TestSessionRepositoryCreateSession(t *testing.T) {
	repo := NewSessionRepository(testQueries)

	tests := []struct {
		name string
		// tokenHash supplies the hash to insert with, and may seed the table first
		tokenHash func(t *testing.T, userID uuid.UUID) string
		wantErr   bool
		check     func(t *testing.T, userID uuid.UUID, expires time.Time, got *entity.Session)
	}{
		{
			name:      "creates a session",
			tokenHash: func(*testing.T, uuid.UUID) string { return uuid.NewString() },
			check: func(t *testing.T, userID uuid.UUID, expires time.Time, got *entity.Session) {
				require.NotEqual(t, uuid.Nil, got.ID)
				require.Equal(t, userID, got.UserID)
				require.Equal(t, "curl/8", got.UserAgent)
				require.Equal(t, "10.0.0.1", got.ClientIP)
				require.WithinDuration(t, expires, got.ExpiresAt, time.Second)
				require.False(t, got.CreatedAt.IsZero())
			},
		},
		{
			name: "a duplicate token hash is rejected",
			tokenHash: func(t *testing.T, userID uuid.UUID) string {
				hash := uuid.NewString()
				insertSession(t, repo, userID, hash, time.Now().Add(time.Hour))
				return hash
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := createRandomUser(t, UserRolePlayer).ID
			expires := time.Now().Add(time.Hour)

			got, err := repo.CreateSession(context.Background(), entity.Session{
				UserID:    userID,
				UserAgent: "curl/8",
				ClientIP:  "10.0.0.1",
				ExpiresAt: expires,
			}, tt.tokenHash(t, userID))

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			tt.check(t, userID, expires, got)
		})
	}
}

func TestSessionRepositoryGetSessionUser(t *testing.T) {
	repo := NewSessionRepository(testQueries)

	tests := []struct {
		name string
		// token returns the hash to look up and the user it should belong to
		token   func(t *testing.T) (string, *User)
		wantErr error
	}{
		{
			name: "returns the session and its user",
			token: func(t *testing.T) (string, *User) {
				user := createRandomUser(t, UserRoleOwner)
				hash := uuid.NewString()
				insertSession(t, repo, user.ID, hash, time.Now().Add(time.Hour))
				return hash, &user
			},
		},
		{
			name: "an unknown token is invalid",
			token: func(*testing.T) (string, *User) {
				return uuid.NewString(), nil
			},
			wantErr: entity.ErrSessionInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, want := tt.token(t)

			gotSession, gotUser, err := repo.GetSessionUser(context.Background(), hash)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, gotSession)
				require.Nil(t, gotUser)
				return
			}

			require.NoError(t, err)
			require.NotEqual(t, uuid.Nil, gotSession.ID)
			require.Equal(t, want.ID, gotUser.ID)
			require.Equal(t, want.Email, gotUser.Email)
			require.Equal(t, entity.RoleOwner, gotUser.Role)
			require.True(t, gotUser.IsActive)
		})
	}
}

func TestSessionRepositoryDeleteSession(t *testing.T) {
	repo := NewSessionRepository(testQueries)

	tests := []struct {
		name      string
		tokenHash func(t *testing.T) string
	}{
		{
			name: "deletes an existing session",
			tokenHash: func(t *testing.T) string {
				userID := createRandomUser(t, UserRolePlayer).ID
				hash := uuid.NewString()
				insertSession(t, repo, userID, hash, time.Now().Add(time.Hour))
				return hash
			},
		},
		{
			// Logout must be idempotent, so an unknown token is not an error.
			name:      "deleting an unknown session is a no-op",
			tokenHash: func(*testing.T) string { return uuid.NewString() },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			hash := tt.tokenHash(t)

			require.NoError(t, repo.DeleteSession(ctx, hash))

			_, _, err := repo.GetSessionUser(ctx, hash)
			require.ErrorIs(t, err, entity.ErrSessionInvalid)
		})
	}
}

func TestUserRepositoryGetCredentialsByEmail(t *testing.T) {
	repo := NewUserRepository(testQueries)

	tests := []struct {
		name    string
		email   func(t *testing.T) (string, *User)
		wantErr error
	}{
		{
			name: "returns the id and password hash",
			email: func(t *testing.T) (string, *User) {
				user := createRandomUser(t, UserRolePlayer)
				return user.Email, &user
			},
		},
		{
			name: "an unknown email is not found",
			email: func(*testing.T) (string, *User) {
				return "nobody-" + uuid.NewString() + "@example.com", nil
			},
			wantErr: entity.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email, want := tt.email(t)

			id, hash, err := repo.GetCredentialsByEmail(context.Background(), email)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Equal(t, uuid.Nil, id)
				require.Empty(t, hash)
				return
			}

			require.NoError(t, err)
			require.Equal(t, want.ID, id)
			require.Equal(t, want.PasswordHash, hash)
		})
	}
}
