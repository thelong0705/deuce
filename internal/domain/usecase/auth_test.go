package usecase_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/domain/entity"
	"github.com/thelong0705/deuce/internal/domain/usecase"
	"github.com/thelong0705/deuce/internal/domain/usecase/mocks"
)

const sessionTTL = time.Hour

var (
	authUserID   = uuid.New()
	storedHash   = "hashed:supersecret"
	authPassword = "supersecret"
)

func validLogin() entity.LoginInput {
	return entity.LoginInput{
		Email:     "alice@example.com",
		Password:  authPassword,
		UserAgent: "curl/8",
		ClientIP:  "10.0.0.1",
	}
}

func hashOf(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func TestAuthLogin(t *testing.T) {
	boom := errors.New("boom")

	tests := []struct {
		name    string
		mutate  func(in *entity.LoginInput)
		setup   func(creds *mocks.MockCredentialFinder, pw *mocks.MockPasswordHasher, sessions *mocks.MockSessionStore)
		wantErr error
	}{
		{
			name: "issues a session for correct credentials",
		},
		{
			name:    "rejects a malformed email before any lookup",
			mutate:  func(in *entity.LoginInput) { in.Email = "nope" },
			setup:   func(*mocks.MockCredentialFinder, *mocks.MockPasswordHasher, *mocks.MockSessionStore) {},
			wantErr: entity.ErrInvalidEmail,
		},
		{
			name:    "rejects an empty password before any lookup",
			mutate:  func(in *entity.LoginInput) { in.Password = "" },
			setup:   func(*mocks.MockCredentialFinder, *mocks.MockPasswordHasher, *mocks.MockSessionStore) {},
			wantErr: entity.ErrPasswordRequired,
		},
		{
			name: "an unknown email is indistinguishable from a wrong password",
			setup: func(creds *mocks.MockCredentialFinder, _ *mocks.MockPasswordHasher, _ *mocks.MockSessionStore) {
				creds.EXPECT().GetCredentialsByEmail(mock.Anything, mock.Anything).
					Return(uuid.Nil, "", entity.ErrUserNotFound).Once()
			},
			wantErr: entity.ErrInvalidCredentials,
		},
		{
			name: "a wrong password does not create a session",
			setup: func(creds *mocks.MockCredentialFinder, pw *mocks.MockPasswordHasher, _ *mocks.MockSessionStore) {
				creds.EXPECT().GetCredentialsByEmail(mock.Anything, mock.Anything).
					Return(authUserID, storedHash, nil).Once()
				pw.EXPECT().Compare(storedHash, mock.Anything).Return(errors.New("mismatch")).Once()
			},
			wantErr: entity.ErrInvalidCredentials,
		},
		{
			name: "propagates a lookup failure",
			setup: func(creds *mocks.MockCredentialFinder, _ *mocks.MockPasswordHasher, _ *mocks.MockSessionStore) {
				creds.EXPECT().GetCredentialsByEmail(mock.Anything, mock.Anything).
					Return(uuid.Nil, "", boom).Once()
			},
			wantErr: boom,
		},
		{
			name: "propagates a session store failure",
			setup: func(creds *mocks.MockCredentialFinder, pw *mocks.MockPasswordHasher, sessions *mocks.MockSessionStore) {
				creds.EXPECT().GetCredentialsByEmail(mock.Anything, mock.Anything).
					Return(authUserID, storedHash, nil).Once()
				pw.EXPECT().Compare(storedHash, authPassword).Return(nil).Once()
				sessions.EXPECT().CreateSession(mock.Anything, mock.Anything, mock.Anything).
					Return(nil, boom).Once()
			},
			wantErr: boom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := mocks.NewMockCredentialFinder(t)
			pw := mocks.NewMockPasswordHasher(t)
			sessions := mocks.NewMockSessionStore(t)

			var storedToken string
			var stored entity.Session

			if tt.setup != nil {
				tt.setup(creds, pw, sessions)
			} else {
				creds.EXPECT().GetCredentialsByEmail(mock.Anything, "alice@example.com").
					Return(authUserID, storedHash, nil).Once()
				pw.EXPECT().Compare(storedHash, authPassword).Return(nil).Once()
				sessions.EXPECT().
					CreateSession(mock.Anything, mock.Anything, mock.Anything).
					Run(func(_ context.Context, s entity.Session, tokenHash string) {
						stored, storedToken = s, tokenHash
					}).
					Return(&entity.Session{UserID: authUserID}, nil).
					Once()
			}

			in := validLogin()
			if tt.mutate != nil {
				tt.mutate(&in)
			}

			svc := usecase.NewUser(mocks.NewMockUserCreator(t), pw, creds, sessions, missingCache(t), sessionTTL)
			token, session, err := svc.Login(context.Background(), in)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Empty(t, token)
				sessions.AssertNotCalled(t, "CreateSession")
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, token)
			require.Equal(t, authUserID, session.UserID)

			// Only the hash reaches storage; the raw token is returned once.
			require.Equal(t, hashOf(token), storedToken)
			require.NotEqual(t, token, storedToken)

			require.Equal(t, authUserID, stored.UserID)
			require.Equal(t, "curl/8", stored.UserAgent)
			require.Equal(t, "10.0.0.1", stored.ClientIP)
			require.WithinDuration(t, time.Now().Add(sessionTTL), stored.ExpiresAt, time.Minute)
		})
	}
}

func TestLoginTokensAreUnique(t *testing.T) {
	seen := make(map[string]bool, 50)

	for range 50 {
		creds := mocks.NewMockCredentialFinder(t)
		pw := mocks.NewMockPasswordHasher(t)
		sessions := mocks.NewMockSessionStore(t)

		creds.EXPECT().GetCredentialsByEmail(mock.Anything, mock.Anything).
			Return(authUserID, storedHash, nil).Once()
		pw.EXPECT().Compare(mock.Anything, mock.Anything).Return(nil).Once()
		sessions.EXPECT().CreateSession(mock.Anything, mock.Anything, mock.Anything).
			Return(&entity.Session{}, nil).Once()

		svc := usecase.NewUser(mocks.NewMockUserCreator(t), pw, creds, sessions, missingCache(t), sessionTTL)
		token, _, err := svc.Login(context.Background(), validLogin())
		require.NoError(t, err)

		require.False(t, seen[token], "session tokens must never repeat")
		seen[token] = true
	}
}

func TestAuthAuthenticate(t *testing.T) {
	const token = "a-token"
	now := time.Now()

	tests := []struct {
		name    string
		token   string
		setup   func(sessions *mocks.MockSessionStore)
		wantErr error
	}{
		{
			name:  "returns the user behind a live session",
			token: token,
			setup: func(sessions *mocks.MockSessionStore) {
				sessions.EXPECT().GetSessionUser(mock.Anything, hashOf(token)).
					Return(
						&entity.Session{UserID: authUserID, ExpiresAt: now.Add(time.Hour)},
						&entity.User{ID: authUserID, IsActive: true},
						nil,
					).Once()
			},
		},
		{
			name:    "an empty token is rejected without a lookup",
			token:   "",
			setup:   func(*mocks.MockSessionStore) {},
			wantErr: entity.ErrSessionInvalid,
		},
		{
			name:  "an expired session is rejected",
			token: token,
			setup: func(sessions *mocks.MockSessionStore) {
				sessions.EXPECT().GetSessionUser(mock.Anything, mock.Anything).
					Return(
						&entity.Session{UserID: authUserID, ExpiresAt: now.Add(-time.Second)},
						&entity.User{ID: authUserID, IsActive: true},
						nil,
					).Once()
			},
			wantErr: entity.ErrSessionInvalid,
		},
		{
			name:  "a deactivated user is rejected even with a live session",
			token: token,
			setup: func(sessions *mocks.MockSessionStore) {
				sessions.EXPECT().GetSessionUser(mock.Anything, mock.Anything).
					Return(
						&entity.Session{UserID: authUserID, ExpiresAt: now.Add(time.Hour)},
						&entity.User{ID: authUserID, IsActive: false},
						nil,
					).Once()
			},
			wantErr: entity.ErrSessionInvalid,
		},
		{
			name:  "an unknown token is rejected",
			token: token,
			setup: func(sessions *mocks.MockSessionStore) {
				sessions.EXPECT().GetSessionUser(mock.Anything, mock.Anything).
					Return(nil, nil, entity.ErrSessionInvalid).Once()
			},
			wantErr: entity.ErrSessionInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := mocks.NewMockSessionStore(t)
			tt.setup(sessions)

			svc := usecase.NewUser(
				mocks.NewMockUserCreator(t),
				mocks.NewMockPasswordHasher(t),
				mocks.NewMockCredentialFinder(t),
				sessions,
				missingCache(t),
				sessionTTL,
			)

			user, err := svc.Authenticate(context.Background(), tt.token)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, user)
				return
			}

			require.NoError(t, err)
			require.Equal(t, authUserID, user.ID)
		})
	}
}

func TestAuthLogout(t *testing.T) {
	const token = "a-token"

	t.Run("deletes the session by token hash", func(t *testing.T) {
		sessions := mocks.NewMockSessionStore(t)
		sessions.EXPECT().DeleteSession(mock.Anything, hashOf(token)).Return(nil).Once()

		svc := usecase.NewUser(
			mocks.NewMockUserCreator(t),
			mocks.NewMockPasswordHasher(t),
			mocks.NewMockCredentialFinder(t),
			sessions,
			missingCache(t),
			sessionTTL,
		)

		require.NoError(t, svc.Logout(context.Background(), token))
	})

	t.Run("an empty token is a no-op", func(t *testing.T) {
		sessions := mocks.NewMockSessionStore(t)

		svc := usecase.NewUser(
			mocks.NewMockUserCreator(t),
			mocks.NewMockPasswordHasher(t),
			mocks.NewMockCredentialFinder(t),
			sessions,
			missingCache(t),
			sessionTTL,
		)

		require.NoError(t, svc.Logout(context.Background(), ""))
		sessions.AssertNotCalled(t, "DeleteSession")
	})
}

// newCachedUser builds a use case whose only interesting dependencies are the
// session store and the cache in front of it.
// missingCache never has anything and remembers nothing, for the tests that are
// about the store rather than the cache.
func missingCache(t *testing.T) *mocks.MockSessionCache {
	t.Helper()

	cache := mocks.NewMockSessionCache(t)
	cache.EXPECT().GetSessionUser(mock.Anything, mock.Anything).Return(nil, nil, false).Maybe()
	cache.EXPECT().PutSessionUser(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	cache.EXPECT().DeleteSession(mock.Anything, mock.Anything).Maybe()

	return cache
}

func newCachedUser(t *testing.T, sessions *mocks.MockSessionStore, cache *mocks.MockSessionCache) *usecase.User {
	t.Helper()

	return usecase.NewUser(
		mocks.NewMockUserCreator(t),
		mocks.NewMockPasswordHasher(t),
		mocks.NewMockCredentialFinder(t),
		sessions,
		cache,
		sessionTTL,
	)
}

func liveSession() *entity.Session {
	return &entity.Session{UserID: authUserID, ExpiresAt: time.Now().Add(time.Hour)}
}

func activeUser() *entity.User {
	return &entity.User{ID: authUserID, IsActive: true}
}

func TestAuthAuthenticateUsesTheCache(t *testing.T) {
	const token = "a-token"

	tests := []struct {
		name  string
		token string
		setup func(sessions *mocks.MockSessionStore, cache *mocks.MockSessionCache)
		// wantErr is what Authenticate must return
		wantErr error
	}{
		{
			// The store expectation is absent, so reaching it fails the test.
			name:  "a hit is served without touching the store",
			token: token,
			setup: func(_ *mocks.MockSessionStore, cache *mocks.MockSessionCache) {
				cache.EXPECT().GetSessionUser(mock.Anything, hashOf(token)).
					Return(liveSession(), activeUser(), true).Once()
			},
		},
		{
			name:  "a miss reads through and fills the cache",
			token: token,
			setup: func(sessions *mocks.MockSessionStore, cache *mocks.MockSessionCache) {
				session, user := liveSession(), activeUser()

				cache.EXPECT().GetSessionUser(mock.Anything, hashOf(token)).
					Return(nil, nil, false).Once()
				sessions.EXPECT().GetSessionUser(mock.Anything, hashOf(token)).
					Return(session, user, nil).Once()
				cache.EXPECT().PutSessionUser(mock.Anything, hashOf(token), session, user).Once()
			},
		},
		{
			// Caching a rejection would have to be undone at login.
			name:  "a rejection is not cached",
			token: token,
			setup: func(sessions *mocks.MockSessionStore, cache *mocks.MockSessionCache) {
				cache.EXPECT().GetSessionUser(mock.Anything, mock.Anything).
					Return(nil, nil, false).Once()
				sessions.EXPECT().GetSessionUser(mock.Anything, mock.Anything).
					Return(nil, nil, entity.ErrSessionInvalid).Once()
			},
			wantErr: entity.ErrSessionInvalid,
		},
		{
			// Expiry is checked after the cache, not by it, so an entry that has
			// outlived its session cannot let the session through.
			name:  "a cached session that has since expired is still refused",
			token: token,
			setup: func(_ *mocks.MockSessionStore, cache *mocks.MockSessionCache) {
				expired := liveSession()
				expired.ExpiresAt = time.Now().Add(-time.Second)

				cache.EXPECT().GetSessionUser(mock.Anything, mock.Anything).
					Return(expired, activeUser(), true).Once()
			},
			wantErr: entity.ErrSessionInvalid,
		},
		{
			name:  "a cached user who has since been deactivated is still refused",
			token: token,
			setup: func(_ *mocks.MockSessionStore, cache *mocks.MockSessionCache) {
				inactive := activeUser()
				inactive.IsActive = false

				cache.EXPECT().GetSessionUser(mock.Anything, mock.Anything).
					Return(liveSession(), inactive, true).Once()
			},
			wantErr: entity.ErrSessionInvalid,
		},
		{
			name:    "an empty token is rejected without asking the cache",
			setup:   func(*mocks.MockSessionStore, *mocks.MockSessionCache) {},
			wantErr: entity.ErrSessionInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := mocks.NewMockSessionStore(t)
			cache := mocks.NewMockSessionCache(t)
			tt.setup(sessions, cache)

			user, err := newCachedUser(t, sessions, cache).Authenticate(context.Background(), tt.token)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, user)
				return
			}

			require.NoError(t, err)
			require.Equal(t, authUserID, user.ID)
		})
	}
}

// Logging out has to reach the cache, or the session would keep working from it
// until the entry expired.
func TestAuthLogoutEvictsTheCachedSession(t *testing.T) {
	const token = "a-token"

	sessions := mocks.NewMockSessionStore(t)
	cache := mocks.NewMockSessionCache(t)

	cache.EXPECT().DeleteSession(mock.Anything, hashOf(token)).Once()
	sessions.EXPECT().DeleteSession(mock.Anything, hashOf(token)).Return(nil).Once()

	require.NoError(t, newCachedUser(t, sessions, cache).Logout(context.Background(), token))
}

func TestAuthLogoutWithNoTokenTouchesNeither(t *testing.T) {
	sessions := mocks.NewMockSessionStore(t)
	cache := mocks.NewMockSessionCache(t)

	require.NoError(t, newCachedUser(t, sessions, cache).Logout(context.Background(), ""))

	cache.AssertNotCalled(t, "DeleteSession")
	sessions.AssertNotCalled(t, "DeleteSession")
}
