package noop_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/adapter/cache/noop"
	"github.com/thelong0705/deuce/internal/domain/entity"
)

// Whatever it is told, it still has nothing: the use case reads through every
// time.
func TestEverythingIsAMiss(t *testing.T) {
	cache := noop.NewSessionCache()
	ctx := context.Background()

	session := &entity.Session{ID: uuid.New(), ExpiresAt: time.Now().Add(time.Hour)}
	user := &entity.User{ID: uuid.New(), IsActive: true}

	cache.PutSessionUser(ctx, "a-hash", session, user)

	gotSession, gotUser, ok := cache.GetSessionUser(ctx, "a-hash")

	require.False(t, ok)
	require.Nil(t, gotSession)
	require.Nil(t, gotUser)
}

func TestDeleteIsHarmless(t *testing.T) {
	require.NotPanics(t, func() {
		noop.NewSessionCache().DeleteSession(context.Background(), "a-hash")
	})
}
