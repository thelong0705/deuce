package usecase

import (
	"context"
	"fmt"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

// CreateUserRecord is what storage needs in order to persist a user. It holds
// only what the caller decides. ID, CreatedAt and IsActive are assigned by
// storage -- accounts are only deactivated after they exist, so activation is
// not an input to creation.
type CreateUserRecord struct {
	Email        string
	PasswordHash string
	DisplayName  string
	PhoneNumber  string
	Role         entity.Role
}

// UserCreator creates a user.
type UserCreator interface {
	CreateUser(ctx context.Context, rec CreateUserRecord) (entity.User, error)
}

// PasswordHasher keeps bcrypt out of both the domain and the use case.
type PasswordHasher interface {
	Hash(plain string) (string, error)
}

type User struct {
	userCreator UserCreator
	hasher      PasswordHasher
}

func NewUser(userCreator UserCreator, hasher PasswordHasher) *User {
	return &User{userCreator: userCreator, hasher: hasher}
}

// Register validates a signup, hashes the password and persists the account.
func (s *User) Register(ctx context.Context, in entity.CreateUserInput) (entity.User, error) {
	if in.Role == "" {
		in.Role = entity.RolePlayer
	}

	if err := in.Validate(); err != nil {
		return entity.User{}, err
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return entity.User{}, fmt.Errorf("hash password: %w", err)
	}

	return s.userCreator.CreateUser(ctx, CreateUserRecord{
		Email:        in.Email,
		PasswordHash: hash,
		DisplayName:  in.DisplayName(),
		PhoneNumber:  in.PhoneNumber,
		Role:         in.Role,
	})
}
