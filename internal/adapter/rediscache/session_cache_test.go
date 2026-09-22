package rediscache_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/adapter/rediscache"
	"github.com/thelong0705/deuce/internal/domain/entity"
)

const (
	tokenHash = "e3b0c44298fc1c149afbf4c8996fb924"
	cacheKey  = "session:" + tokenHash
	cacheTTL  = 10 * time.Minute
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

// newCache returns a cache and the fake Redis behind it, so a test can expire
// keys or take it away.
func newCache(t *testing.T, ttl time.Duration) (*rediscache.SessionCache, *miniredis.Miniredis) {
	t.Helper()

	server := miniredis.RunT(t)

	// Same impatience as the server wires up: a test for the unreachable case
	// should not spend seconds backing off.
	client := redis.NewClient(rediscache.Options(server.Addr()))
	t.Cleanup(func() { _ = client.Close() })

	return rediscache.NewSessionCache(client, ttl), server
}

func TestPutThenGetReturnsWhatWasStored(t *testing.T) {
	cache, _ := newCache(t, cacheTTL)
	ctx := context.Background()

	session, user := storedSession(), storedUser()
	cache.PutSessionUser(ctx, tokenHash, session, user)

	gotSession, gotUser, ok := cache.GetSessionUser(ctx, tokenHash)

	require.True(t, ok)
	require.Equal(t, session.ID, gotSession.ID)
	require.Equal(t, session.UserAgent, gotSession.UserAgent)
	require.True(t, session.ExpiresAt.Equal(gotSession.ExpiresAt))
	require.Equal(t, user.ID, gotUser.ID)
	require.Equal(t, user.Email, gotUser.Email)
	require.Equal(t, user.Role, gotUser.Role)
	require.True(t, gotUser.IsActive)
}

func TestGetReportsAMiss(t *testing.T) {
	cache, _ := newCache(t, cacheTTL)

	_, _, ok := cache.GetSessionUser(context.Background(), tokenHash)

	require.False(t, ok)
}

func TestTheEntryLapsesAfterTheTTL(t *testing.T) {
	cache, server := newCache(t, cacheTTL)
	ctx := context.Background()

	cache.PutSessionUser(ctx, tokenHash, storedSession(), storedUser())

	server.FastForward(cacheTTL + time.Second)

	_, _, ok := cache.GetSessionUser(ctx, tokenHash)
	require.False(t, ok)
}

// An entry must not outlive the session, or expiry would stop being enforced
// for as long as it lingered.
func TestTheEntryNeverOutlivesTheSession(t *testing.T) {
	cache, server := newCache(t, time.Hour)
	ctx := context.Background()

	session := storedSession()
	session.ExpiresAt = time.Now().Add(5 * time.Second)

	cache.PutSessionUser(ctx, tokenHash, session, storedUser())

	ttl := server.TTL(cacheKey)
	require.Greater(t, ttl, time.Duration(0))
	require.LessOrEqual(t, ttl, 5*time.Second)
}

func TestAnAlreadyExpiredSessionIsNotStored(t *testing.T) {
	cache, server := newCache(t, cacheTTL)
	ctx := context.Background()

	session := storedSession()
	session.ExpiresAt = time.Now().Add(-time.Minute)

	cache.PutSessionUser(ctx, tokenHash, session, storedUser())

	require.False(t, server.Exists(cacheKey))
}

func TestDeleteRemovesTheEntry(t *testing.T) {
	cache, server := newCache(t, cacheTTL)
	ctx := context.Background()

	cache.PutSessionUser(ctx, tokenHash, storedSession(), storedUser())
	require.True(t, server.Exists(cacheKey))

	cache.DeleteSession(ctx, tokenHash)

	require.False(t, server.Exists(cacheKey))
}

// An entry written by an older build, or corrupted, is a miss rather than
// something that reaches the caller.
func TestAnUnreadableEntryIsAMiss(t *testing.T) {
	cache, server := newCache(t, cacheTTL)

	require.NoError(t, server.Set(cacheKey, "not json"))

	_, _, ok := cache.GetSessionUser(context.Background(), tokenHash)

	require.False(t, ok)
}

// Redis being unreachable must be a miss, not a failure: the use case reads
// through, so requests keep working.
func TestUnreachableRedisIsAMiss(t *testing.T) {
	cache, server := newCache(t, cacheTTL)
	ctx := context.Background()

	server.Close()

	_, _, ok := cache.GetSessionUser(ctx, tokenHash)
	require.False(t, ok)

	// Neither of these may panic or block.
	cache.PutSessionUser(ctx, tokenHash, storedSession(), storedUser())
	cache.DeleteSession(ctx, tokenHash)
}

// The key is the token's hash, which is what the port is given. The raw token
// never reaches Redis.
func TestTheEntryHoldsNoRawToken(t *testing.T) {
	cache, server := newCache(t, cacheTTL)

	cache.PutSessionUser(context.Background(), tokenHash, storedSession(), storedUser())

	require.Equal(t, []string{cacheKey}, server.Keys())

	raw, err := server.Get(cacheKey)
	require.NoError(t, err)
	require.NotContains(t, raw, "password")
}
