package rediscache_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/adapter/rediscache"
	"github.com/thelong0705/deuce/internal/domain/entity"
	"github.com/thelong0705/deuce/internal/domain/usecase/mocks"
)

const (
	tokenHash = "e3b0c44298fc1c149afbf4c8996fb924"
	cacheTTL  = 30 * time.Second
)

var userID = uuid.New()

func storedSession() *entity.Session {
	return &entity.Session{
		ID:        uuid.New(),
		UserID:    userID,
		UserAgent: "curl/8",
		ClientIP:  "203.0.113.9",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		CreatedAt: time.Now(),
	}
}

func storedUser() *entity.User {
	return &entity.User{
		ID:          userID,
		Email:       "alice@example.com",
		DisplayName: "alice",
		PhoneNumber: "+84901234567",
		Role:        entity.RolePlayer,
		IsActive:    true,
	}
}

// newStore returns a cache over a mock store, and the fake Redis behind it so a
// test can expire keys or take it away.
func newStore(t *testing.T, ttl time.Duration) (*rediscache.SessionStore, *mocks.MockSessionStore, *miniredis.Miniredis) {
	t.Helper()

	server := miniredis.RunT(t)
	// Same impatience as the server wires up: a test for the unreachable case
	// should not spend seconds backing off.
	client := redis.NewClient(rediscache.Options(server.Addr()))
	t.Cleanup(func() { _ = client.Close() })

	inner := mocks.NewMockSessionStore(t)

	return rediscache.NewSessionStore(inner, client, ttl), inner, server
}

// A second lookup of the same token must not reach the store behind the cache,
// which Once() on the expectation is what proves.
func TestGetSessionUserServesTheSecondLookupFromRedis(t *testing.T) {
	store, inner, _ := newStore(t, cacheTTL)
	ctx := context.Background()

	inner.EXPECT().GetSessionUser(ctx, tokenHash).Return(storedSession(), storedUser(), nil).Once()

	first, firstUser, err := store.GetSessionUser(ctx, tokenHash)
	require.NoError(t, err)

	second, secondUser, err := store.GetSessionUser(ctx, tokenHash)
	require.NoError(t, err)

	require.Equal(t, first.ID, second.ID)
	require.Equal(t, first.UserAgent, second.UserAgent)
	require.True(t, first.ExpiresAt.Equal(second.ExpiresAt))
	require.Equal(t, firstUser.ID, secondUser.ID)
	require.Equal(t, firstUser.Email, secondUser.Email)
	require.Equal(t, firstUser.Role, secondUser.Role)
	require.True(t, secondUser.IsActive)
}

func TestGetSessionUserReadsThroughAgainOnceTheEntryExpires(t *testing.T) {
	store, inner, server := newStore(t, cacheTTL)
	ctx := context.Background()

	inner.EXPECT().GetSessionUser(ctx, tokenHash).Return(storedSession(), storedUser(), nil).Twice()

	_, _, err := store.GetSessionUser(ctx, tokenHash)
	require.NoError(t, err)

	server.FastForward(cacheTTL + time.Second)

	_, _, err = store.GetSessionUser(ctx, tokenHash)
	require.NoError(t, err)
}

// An entry must not outlive the session, or expiry would stop being enforced
// for as long as it lingered.
func TestGetSessionUserNeverCachesPastTheSessionsOwnExpiry(t *testing.T) {
	store, inner, server := newStore(t, time.Hour)
	ctx := context.Background()

	session := storedSession()
	session.ExpiresAt = time.Now().Add(5 * time.Second)

	inner.EXPECT().GetSessionUser(ctx, tokenHash).Return(session, storedUser(), nil).Once()

	_, _, err := store.GetSessionUser(ctx, tokenHash)
	require.NoError(t, err)

	ttl := server.TTL("session:" + tokenHash)
	require.Greater(t, ttl, time.Duration(0))
	require.LessOrEqual(t, ttl, 5*time.Second)
}

func TestGetSessionUserDoesNotCacheAnAlreadyExpiredSession(t *testing.T) {
	store, inner, server := newStore(t, cacheTTL)
	ctx := context.Background()

	session := storedSession()
	session.ExpiresAt = time.Now().Add(-time.Minute)

	inner.EXPECT().GetSessionUser(ctx, tokenHash).Return(session, storedUser(), nil).Once()

	_, _, err := store.GetSessionUser(ctx, tokenHash)
	require.NoError(t, err)

	require.False(t, server.Exists("session:"+tokenHash))
}

// Rejections are not cached: rejecting a bad token again is cheap, and holding
// the absence would need evicting when a session is created.
func TestGetSessionUserDoesNotCacheARejection(t *testing.T) {
	store, inner, server := newStore(t, cacheTTL)
	ctx := context.Background()

	inner.EXPECT().GetSessionUser(ctx, tokenHash).
		Return(nil, nil, entity.ErrSessionInvalid).Twice()

	_, _, err := store.GetSessionUser(ctx, tokenHash)
	require.ErrorIs(t, err, entity.ErrSessionInvalid)

	require.False(t, server.Exists("session:"+tokenHash))

	_, _, err = store.GetSessionUser(ctx, tokenHash)
	require.ErrorIs(t, err, entity.ErrSessionInvalid)
}

// Logging out has to reach the cache, or the session would keep working from
// it until the entry expired.
func TestDeleteSessionEvictsTheEntry(t *testing.T) {
	store, inner, server := newStore(t, cacheTTL)
	ctx := context.Background()

	inner.EXPECT().GetSessionUser(ctx, tokenHash).Return(storedSession(), storedUser(), nil).Once()
	inner.EXPECT().DeleteSession(ctx, tokenHash).Return(nil).Once()

	_, _, err := store.GetSessionUser(ctx, tokenHash)
	require.NoError(t, err)
	require.True(t, server.Exists("session:"+tokenHash))

	require.NoError(t, store.DeleteSession(ctx, tokenHash))
	require.False(t, server.Exists("session:"+tokenHash))
}

func TestDeleteSessionPropagatesTheStoreFailure(t *testing.T) {
	store, inner, _ := newStore(t, cacheTTL)
	ctx := context.Background()

	boom := errors.New("boom")
	inner.EXPECT().DeleteSession(ctx, tokenHash).Return(boom).Once()

	require.ErrorIs(t, store.DeleteSession(ctx, tokenHash), boom)
}

// Redis being unreachable must slow requests down, not fail them.
func TestUnreachableRedisFallsThroughToTheStore(t *testing.T) {
	store, inner, server := newStore(t, cacheTTL)
	ctx := context.Background()

	server.Close()

	inner.EXPECT().GetSessionUser(ctx, tokenHash).Return(storedSession(), storedUser(), nil).Twice()
	inner.EXPECT().DeleteSession(ctx, tokenHash).Return(nil).Once()

	for range 2 {
		_, user, err := store.GetSessionUser(ctx, tokenHash)
		require.NoError(t, err)
		require.Equal(t, userID, user.ID)
	}

	require.NoError(t, store.DeleteSession(ctx, tokenHash))
}

// An entry written by an older build, or corrupted, is a miss rather than an
// error.
func TestUnreadableEntryFallsThroughToTheStore(t *testing.T) {
	store, inner, server := newStore(t, cacheTTL)
	ctx := context.Background()

	require.NoError(t, server.Set("session:"+tokenHash, "not json"))

	inner.EXPECT().GetSessionUser(ctx, tokenHash).Return(storedSession(), storedUser(), nil).Once()

	_, user, err := store.GetSessionUser(ctx, tokenHash)

	require.NoError(t, err)
	require.Equal(t, userID, user.ID)
}

// Creating a session writes no entry: it is about to be used, not read back.
func TestCreateSessionIsNotCached(t *testing.T) {
	store, inner, server := newStore(t, cacheTTL)
	ctx := context.Background()

	session := storedSession()
	inner.EXPECT().CreateSession(ctx, *session, tokenHash).Return(session, nil).Once()

	_, err := store.CreateSession(ctx, *session, tokenHash)

	require.NoError(t, err)
	require.False(t, server.Exists("session:"+tokenHash))
}

// The key is the token's hash, which is what the port is given. The raw token
// never reaches Redis.
func TestTheEntryHoldsNoRawToken(t *testing.T) {
	store, inner, server := newStore(t, cacheTTL)
	ctx := context.Background()

	inner.EXPECT().GetSessionUser(ctx, tokenHash).Return(storedSession(), storedUser(), nil).Once()

	_, _, err := store.GetSessionUser(ctx, tokenHash)
	require.NoError(t, err)

	require.Equal(t, []string{"session:" + tokenHash}, server.Keys())

	raw, err := server.Get("session:" + tokenHash)
	require.NoError(t, err)
	require.NotContains(t, raw, "password")
}
