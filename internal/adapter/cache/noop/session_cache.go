// Package noop caches nothing.
package noop

import (
	"context"

	"github.com/thelong0705/deuce/internal/domain/entity"
	"github.com/thelong0705/deuce/internal/domain/usecase"
)

var _ usecase.SessionCache = (*SessionCache)(nil)

// SessionCache stands in when there is no cache, so the use case reads the
// same whether one is configured or not: every lookup is a miss, and storing
// and evicting do nothing.
type SessionCache struct{}

func NewSessionCache() *SessionCache {
	return &SessionCache{}
}

func (*SessionCache) GetSessionUser(context.Context, string) (*entity.Session, *entity.User, bool) {
	return nil, nil, false
}

func (*SessionCache) PutSessionUser(context.Context, string, *entity.Session, *entity.User) {}

func (*SessionCache) DeleteSession(context.Context, string) {}
