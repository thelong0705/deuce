// Package rediscache holds Redis-backed adapters.
package rediscache

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/thelong0705/deuce/internal/domain/entity"
	"github.com/thelong0705/deuce/internal/domain/usecase"
)

var _ usecase.SessionCache = (*SessionCache)(nil)

// SessionCache remembers session lookups in Redis.
//
// When to read, write and evict is the use case's business; this only decides
// how an entry is stored and for how long. Every failure here is swallowed and
// logged, because the use case reads through on a miss and a cache must not be
// able to fail a request.
type SessionCache struct {
	rdb *redis.Client
	ttl time.Duration
}

// Options is how the cache expects its client to be configured: a cache that
// stalls is worse than no cache, so a slow or unreachable Redis gives up
// quickly and the request reads through instead of waiting.
func Options(addr string) *redis.Options {
	return &redis.Options{
		Addr:         addr,
		DialTimeout:  200 * time.Millisecond,
		ReadTimeout:  200 * time.Millisecond,
		WriteTimeout: 200 * time.Millisecond,
		MaxRetries:   -1,
	}
}

func NewSessionCache(rdb *redis.Client, ttl time.Duration) *SessionCache {
	return &SessionCache{rdb: rdb, ttl: ttl}
}

// cachedSession is what an entry holds. The raw token is not in it, and not in
// the key either: the key is already the token's hash.
type cachedSession struct {
	Session entity.Session `json:"session"`
	User    entity.User    `json:"user"`
}

func sessionKey(tokenHash string) string {
	return "session:" + tokenHash
}

// GetSessionUser reports false for a miss, an unreadable entry and an
// unreachable Redis alike. The caller reads through in every one of those
// cases, so a cache problem slows a request down rather than failing it.
func (c *SessionCache) GetSessionUser(ctx context.Context, tokenHash string) (*entity.Session, *entity.User, bool) {
	raw, err := c.rdb.Get(ctx, sessionKey(tokenHash)).Bytes()
	if err != nil {
		if err != redis.Nil {
			slog.Error("read cached session", "error", err)
		}
		return nil, nil, false
	}

	var got cachedSession
	if err := json.Unmarshal(raw, &got); err != nil {
		slog.Error("decode cached session", "error", err)
		return nil, nil, false
	}

	return &got.Session, &got.User, true
}

func (c *SessionCache) PutSessionUser(ctx context.Context, tokenHash string, session *entity.Session, user *entity.User) {
	// An entry must not outlive the session it describes, or expiry would stop
	// being enforced for as long as it lingered.
	ttl := c.ttl
	if until := time.Until(session.ExpiresAt); until < ttl {
		ttl = until
	}

	if ttl <= 0 {
		return
	}

	raw, err := json.Marshal(cachedSession{Session: *session, User: *user})
	if err != nil {
		slog.Error("encode cached session", "error", err)
		return
	}

	if err := c.rdb.Set(ctx, sessionKey(tokenHash), raw, ttl).Err(); err != nil {
		slog.Error("cache session", "error", err)
	}
}

func (c *SessionCache) DeleteSession(ctx context.Context, tokenHash string) {
	if err := c.rdb.Del(ctx, sessionKey(tokenHash)).Err(); err != nil {
		slog.Error("evict cached session", "error", err)
	}
}
