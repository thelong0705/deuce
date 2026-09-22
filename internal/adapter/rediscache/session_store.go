// Package rediscache holds adapters that put Redis in front of another
// implementation of a port.
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

var _ usecase.SessionStore = (*SessionStore)(nil)

// SessionStore caches session lookups in front of another store, which stays
// the source of truth: every miss, and every Redis failure, falls through to
// it.
//
// Logging out evicts the entry, because DeleteSession is on the same port. A
// user being deactivated is not, so a session cached before that keeps working
// until its entry expires — which is what bounds the ttl rather than any
// concern about memory.
type SessionStore struct {
	inner usecase.SessionStore
	rdb   *redis.Client
	ttl   time.Duration
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

func NewSessionStore(inner usecase.SessionStore, rdb *redis.Client, ttl time.Duration) *SessionStore {
	return &SessionStore{inner: inner, rdb: rdb, ttl: ttl}
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

// CreateSession is not cached. A new session is about to be used, not read
// back, and writing it here would mean holding it before anything asks.
func (s *SessionStore) CreateSession(ctx context.Context, session entity.Session, tokenHash string) (*entity.Session, error) {
	return s.inner.CreateSession(ctx, session, tokenHash)
}

func (s *SessionStore) GetSessionUser(ctx context.Context, tokenHash string) (*entity.Session, *entity.User, error) {
	if session, user, ok := s.lookup(ctx, tokenHash); ok {
		return session, user, nil
	}

	session, user, err := s.inner.GetSessionUser(ctx, tokenHash)
	if err != nil {
		// Not cached, including "no such session": a rejected token is cheap to
		// reject again, and caching the absence would need evicting on login.
		return nil, nil, err
	}

	s.store(ctx, tokenHash, session, user)

	return session, user, nil
}

func (s *SessionStore) DeleteSession(ctx context.Context, tokenHash string) error {
	// Evicted before the row goes. The other order would leave an entry with
	// nothing behind it if this failed, and it would be served until it
	// expired.
	if err := s.rdb.Del(ctx, sessionKey(tokenHash)).Err(); err != nil {
		slog.Error("evict cached session", "error", err)
	}

	return s.inner.DeleteSession(ctx, tokenHash)
}

// lookup reports a cached session, or false for a miss, unreadable entry or
// unreachable Redis. The caller reads through in every one of those cases, so
// a cache problem slows requests down rather than failing them.
func (s *SessionStore) lookup(ctx context.Context, tokenHash string) (*entity.Session, *entity.User, bool) {
	raw, err := s.rdb.Get(ctx, sessionKey(tokenHash)).Bytes()
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

func (s *SessionStore) store(ctx context.Context, tokenHash string, session *entity.Session, user *entity.User) {
	// An entry must not outlive the session it describes, or expiry would stop
	// being enforced for as long as it lingered.
	ttl := s.ttl
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

	if err := s.rdb.Set(ctx, sessionKey(tokenHash), raw, ttl).Err(); err != nil {
		slog.Error("cache session", "error", err)
	}
}
