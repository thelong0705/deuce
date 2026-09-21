package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/thelong0705/deuce/internal/domain/entity"
	"github.com/thelong0705/deuce/internal/domain/usecase"
)

var (
	_ usecase.UserCreator      = (*UserRepository)(nil)
	_ usecase.UserFinder       = (*UserRepository)(nil)
	_ usecase.CredentialFinder = (*UserRepository)(nil)
)

// UserRepository persists users in Postgres.
type UserRepository struct {
	q *Queries
}

func NewUserRepository(q *Queries) *UserRepository {
	return &UserRepository{q: q}
}

func (r *UserRepository) CreateUser(ctx context.Context, rec usecase.CreateUserRecord) (*entity.User, error) {
	role, err := toUserRole(rec.Role)
	if err != nil {
		return nil, err
	}

	row, err := r.q.CreateUser(ctx, CreateUserParams{
		Email:        rec.Email,
		PasswordHash: rec.PasswordHash,
		DisplayName:  rec.DisplayName,
		PhoneNumber:  rec.PhoneNumber,
		Role:         role,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, entity.ErrEmailTaken
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	return toEntityUser(row)
}

func toEntityUser(u User) (*entity.User, error) {
	role, err := toEntityRole(u.Role)
	if err != nil {
		return nil, err
	}

	return &entity.User{
		ID:          u.ID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		PhoneNumber: u.PhoneNumber,
		Role:        role,
		IsActive:    u.IsActive,
		CreatedAt:   u.CreatedAt.Time,
	}, nil
}

// toUserRole maps an entity role onto the Postgres enum. A switch rather than a
// cast, because entity.Role is a string type: UserRole("admin") would compile
// and only fail at insert time with an opaque enum error.
func toUserRole(r entity.Role) (UserRole, error) {
	switch r {
	case entity.RolePlayer:
		return UserRolePlayer, nil
	case entity.RoleOwner:
		return UserRoleOwner, nil
	default:
		return "", fmt.Errorf("unknown role %q", r)
	}
}

// toEntityRole maps the Postgres enum back onto an entity role. The column can
// only hold values the enum allows, so the default is unreachable today -- but
// a migration that adds a value would otherwise let it through unnoticed.
func toEntityRole(r UserRole) (entity.Role, error) {
	switch r {
	case UserRolePlayer:
		return entity.RolePlayer, nil
	case UserRoleOwner:
		return entity.RoleOwner, nil
	default:
		return "", fmt.Errorf("unknown role %q from database", r)
	}
}

func (r *UserRepository) GetUser(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	row, err := r.q.GetUser(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, entity.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	return toEntityUser(row)
}

func (r *UserRepository) GetCredentialsByEmail(ctx context.Context, email string) (uuid.UUID, string, error) {
	row, err := r.q.GetCredentialsByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, "", entity.ErrUserNotFound
		}
		return uuid.Nil, "", fmt.Errorf("get credentials by email: %w", err)
	}

	return row.ID, row.PasswordHash, nil
}
