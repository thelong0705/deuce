package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/thelong0705/deuce/internal/domain/entity"
	"github.com/thelong0705/deuce/internal/domain/usecase"
)

var _ usecase.SessionStore = (*SessionRepository)(nil)

// SessionRepository persists sessions in Postgres.
type SessionRepository struct {
	q *Queries
}

func NewSessionRepository(q *Queries) *SessionRepository {
	return &SessionRepository{q: q}
}

func (r *SessionRepository) CreateSession(ctx context.Context, s entity.Session, tokenHash string) (*entity.Session, error) {
	row, err := r.q.CreateSession(ctx, CreateSessionParams{
		UserID:    s.UserID,
		TokenHash: tokenHash,
		UserAgent: s.UserAgent,
		ClientIp:  s.ClientIP,
		ExpiresAt: pgtype.Timestamptz{Time: s.ExpiresAt, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return toEntitySession(row), nil
}

func (r *SessionRepository) GetSessionUser(ctx context.Context, tokenHash string) (*entity.Session, *entity.User, error) {
	row, err := r.q.GetSessionUser(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, entity.ErrSessionInvalid
		}
		return nil, nil, fmt.Errorf("get session user: %w", err)
	}

	user, err := toEntityUser(row.User)
	if err != nil {
		return nil, nil, err
	}

	return toEntitySession(row.Session), user, nil
}

func (r *SessionRepository) DeleteSession(ctx context.Context, tokenHash string) error {
	if err := r.q.DeleteSession(ctx, tokenHash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

func toEntitySession(s Session) *entity.Session {
	return &entity.Session{
		ID:        s.ID,
		UserID:    s.UserID,
		UserAgent: s.UserAgent,
		ClientIP:  s.ClientIp,
		ExpiresAt: s.ExpiresAt.Time,
		CreatedAt: s.CreatedAt.Time,
	}
}
