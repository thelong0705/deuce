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

// SessionStore persists sessions and looks them up by token hash.
type SessionStore interface {
	CreateSession(ctx context.Context, s entity.Session, tokenHash string) (*entity.Session, error)
	GetSessionUser(ctx context.Context, tokenHash string) (*entity.Session, *entity.User, error)
	DeleteSession(ctx context.Context, tokenHash string) error
}

// SessionCache remembers session lookups. A miss and a failure are the same
// thing to a caller, so it reports neither: every one falls through to the
// SessionStore, and the cache never decides whether a request succeeds.
type SessionCache interface {
	GetSessionUser(ctx context.Context, tokenHash string) (*entity.Session, *entity.User, bool)
	PutSessionUser(ctx context.Context, tokenHash string, session *entity.Session, user *entity.User)
	DeleteSession(ctx context.Context, tokenHash string)
}

// Login checks the credentials and starts a session, returning the raw token
// once. Only its hash is stored.
func (s *User) Login(ctx context.Context, in entity.LoginInput) (string, *entity.Session, error) {
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

	if err := s.hasher.Compare(passwordHash, in.Password); err != nil {
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
		ExpiresAt: time.Now().Add(s.sessionTTL),
	}, hash)
	if err != nil {
		return "", nil, err
	}

	return raw, session, nil
}

// Authenticate returns the user behind a session token.
//
// The cache is consulted first and filled on a miss, but it decides nothing:
// expiry and the account being active are checked on the way out, so a cached
// session that has since lapsed is still refused.
func (s *User) Authenticate(ctx context.Context, token string) (*entity.User, error) {
	if token == "" {
		return nil, entity.ErrSessionInvalid
	}

	tokenHash := hashSessionToken(token)

	session, user, cached := s.cache.GetSessionUser(ctx, tokenHash)
	if !cached {
		var err error

		session, user, err = s.sessions.GetSessionUser(ctx, tokenHash)
		if err != nil {
			// A rejection is not remembered: refusing an unknown token again is
			// cheap, and holding the absence would have to be undone at login.
			return nil, err
		}

		s.cache.PutSessionUser(ctx, tokenHash, session, user)
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
func (s *User) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}

	tokenHash := hashSessionToken(token)

	// Evicted before the row goes. The other order would leave the session
	// usable from the cache if this step failed.
	s.cache.DeleteSession(ctx, tokenHash)

	return s.sessions.DeleteSession(ctx, tokenHash)
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
