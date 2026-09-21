package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

// tokenBytes is the size of a session token before encoding.
const tokenBytes = 32

// CredentialFinder returns the stored password hash for an email address.
type CredentialFinder interface {
	GetCredentialsByEmail(ctx context.Context, email string) (userID uuid.UUID, passwordHash string, err error)
}

// PasswordComparer reports whether a plaintext password matches a hash.
type PasswordComparer interface {
	Compare(hash, plain string) error
}

// SessionStore persists sessions and looks them up by token hash.
type SessionStore interface {
	CreateSession(ctx context.Context, s entity.Session, tokenHash string) (*entity.Session, error)
	GetSessionUser(ctx context.Context, tokenHash string) (*entity.Session, *entity.User, error)
	DeleteSession(ctx context.Context, tokenHash string) error
}

type Auth struct {
	credentials CredentialFinder
	passwords   PasswordComparer
	sessions    SessionStore
	ttl         time.Duration
}

func NewAuth(credentials CredentialFinder, passwords PasswordComparer, sessions SessionStore, ttl time.Duration) *Auth {
	return &Auth{credentials: credentials, passwords: passwords, sessions: sessions, ttl: ttl}
}

// Login checks the credentials and starts a session, returning the raw token
// once. Only its hash is stored.
func (s *Auth) Login(ctx context.Context, in entity.LoginInput) (string, *entity.Session, error) {
	if err := in.Validate(); err != nil {
		return "", nil, err
	}

	userID, passwordHash, err := s.credentials.GetCredentialsByEmail(ctx, in.Email)
	if err != nil {
		if errors.Is(err, entity.ErrUserNotFound) {
			return "", nil, entity.ErrInvalidCredentials
		}
		return "", nil, err
	}

	if err := s.passwords.Compare(passwordHash, in.Password); err != nil {
		return "", nil, entity.ErrInvalidCredentials
	}

	raw, hash, err := newSessionToken()
	if err != nil {
		return "", nil, fmt.Errorf("generate session token: %w", err)
	}

	session, err := s.sessions.CreateSession(ctx, entity.Session{
		UserID:    userID,
		UserAgent: in.UserAgent,
		ClientIP:  in.ClientIP,
		ExpiresAt: time.Now().Add(s.ttl),
	}, hash)
	if err != nil {
		return "", nil, err
	}

	return raw, session, nil
}

// Authenticate returns the user behind a session token.
func (s *Auth) Authenticate(ctx context.Context, token string) (*entity.User, error) {
	if token == "" {
		return nil, entity.ErrSessionInvalid
	}

	session, user, err := s.sessions.GetSessionUser(ctx, hashSessionToken(token))
	if err != nil {
		return nil, err
	}

	if !session.IsActive(time.Now()) {
		return nil, entity.ErrSessionInvalid
	}

	if !user.IsActive {
		return nil, entity.ErrSessionInvalid
	}

	return user, nil
}

// Logout ends a session. An unknown token is not an error.
func (s *Auth) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}

	return s.sessions.DeleteSession(ctx, hashSessionToken(token))
}

func newSessionToken() (raw, hash string, err error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}

	raw = base64.RawURLEncoding.EncodeToString(b)

	return raw, hashSessionToken(raw), nil
}

func hashSessionToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
