// Package redis caches in Redis.
package redis

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/thelong0705/deuce/internal/domain/entity"
	"github.com/thelong0705/deuce/internal/domain/usecase"
)

var _ usecase.SessionCache = (*SessionCache)(nil)

// SessionCache remembers session lookups in Redis. Every failure is logged and
// reported as a miss, so the use case reads through rather than failing.
type SessionCache struct {
	rdb *goredis.Client
	ttl time.Duration
}

// Options gives up quickly, because a cache that stalls is worse than no cache.
func Options(addr string) *goredis.Options {
	return &goredis.Options{
		Addr:         addr,
		DialTimeout:  200 * time.Millisecond,
		ReadTimeout:  200 * time.Millisecond,
		WriteTimeout: 200 * time.Millisecond,
		MaxRetries:   -1,
	}
}

func NewSessionCache(rdb *goredis.Client, ttl time.Duration) *SessionCache {
	return &SessionCache{rdb: rdb, ttl: ttl}
}

type cachedSession struct {
	Session entity.Session `json:"session"`
	User    entity.User    `json:"user"`
}

func sessionKey(tokenHash string) string {
	return "session:" + tokenHash
}

func (c *SessionCache) GetSessionUser(ctx context.Context, tokenHash string) (*entity.Session, *entity.User, bool) {
	raw, err := c.rdb.Get(ctx, sessionKey(tokenHash)).Bytes()
	if err != nil {
		if err != goredis.Nil {
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
	// An entry must not outlive the session, or expiry would go unenforced for
	// as long as it lingered.
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
