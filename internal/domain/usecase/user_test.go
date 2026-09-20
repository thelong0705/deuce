package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/domain/entity"
	"github.com/thelong0705/deuce/internal/domain/usecase"
	"github.com/thelong0705/deuce/internal/domain/usecase/mocks"
)

func validInput() entity.CreateUserInput {
	return entity.CreateUserInput{
		Email:       "alice@example.com",
		Password:    "supersecret",
		PhoneNumber: "+84901234567",
		Role:        entity.RolePlayer,
	}
}

// noMocks sets no expectations, so the mock panics if either dependency is
// touched. That is the assertion for cases that must fail before any I/O.
func noMocks(*mocks.MockUserCreator, *mocks.MockPasswordHasher) {}

func TestRegister(t *testing.T) {
	boom := errors.New("boom")

	tests := []struct {
		name string
		// mutate adjusts the valid input for this case
		mutate func(in *entity.CreateUserInput)
		// setup configures the mocks; nil means the happy path
		setup func(creator *mocks.MockUserCreator, hasher *mocks.MockPasswordHasher)
		// wantErr is what Register must return
		wantErr error
		// wantStored asserts what was handed to storage
		wantStored func(t *testing.T, rec usecase.CreateUserRecord)
		// wantNoStorage asserts the creator was never reached
		wantNoStorage bool
	}{
		{
			name: "hashes the password before storing it",
			wantStored: func(t *testing.T, rec usecase.CreateUserRecord) {
				require.Equal(t, "hashed:supersecret", rec.PasswordHash)
				// The rule that matters: storage never sees the plaintext.
				require.NotEqual(t, "supersecret", rec.PasswordHash)
			},
		},
		{
			name:   "defaults an empty role to player",
			mutate: func(in *entity.CreateUserInput) { in.Role = "" },
			wantStored: func(t *testing.T, rec usecase.CreateUserRecord) {
				require.Equal(t, entity.RolePlayer, rec.Role)
			},
		},
		{
			name:   "keeps an explicit owner role",
			mutate: func(in *entity.CreateUserInput) { in.Role = entity.RoleOwner },
			wantStored: func(t *testing.T, rec usecase.CreateUserRecord) {
				require.Equal(t, entity.RoleOwner, rec.Role)
			},
		},
		{
			name:   "derives the display name from the email",
			mutate: func(in *entity.CreateUserInput) { in.Email = "long@abema.tv" },
			wantStored: func(t *testing.T, rec usecase.CreateUserRecord) {
				require.Equal(t, "long", rec.DisplayName)
			},
		},
		{
			name:          "rejects a bad email",
			mutate:        func(in *entity.CreateUserInput) { in.Email = "nope" },
			setup:         noMocks,
			wantErr:       entity.ErrInvalidEmail,
			wantNoStorage: true,
		},
		{
			name:          "rejects a short password",
			mutate:        func(in *entity.CreateUserInput) { in.Password = "short" },
			setup:         noMocks,
			wantErr:       entity.ErrPasswordTooShort,
			wantNoStorage: true,
		},
		{
			name:          "rejects a missing phone",
			mutate:        func(in *entity.CreateUserInput) { in.PhoneNumber = "" },
			setup:         noMocks,
			wantErr:       entity.ErrPhoneRequired,
			wantNoStorage: true,
		},
		{
			name:          "rejects an unknown role",
			mutate:        func(in *entity.CreateUserInput) { in.Role = "admin" },
			setup:         noMocks,
			wantErr:       entity.ErrInvalidRole,
			wantNoStorage: true,
		},
		{
			name: "propagates a hasher failure without storing",
			setup: func(_ *mocks.MockUserCreator, hasher *mocks.MockPasswordHasher) {
				hasher.EXPECT().Hash(mock.Anything).Return("", boom).Once()
			},
			wantErr:       boom,
			wantNoStorage: true,
		},
		{
			name: "propagates a creator failure",
			setup: func(creator *mocks.MockUserCreator, hasher *mocks.MockPasswordHasher) {
				hasher.EXPECT().Hash(mock.Anything).Return("hashed", nil).Once()
				creator.EXPECT().
					CreateUser(mock.Anything, mock.Anything).
					Return(entity.User{}, entity.ErrEmailTaken).
					Once()
			},
			wantErr: entity.ErrEmailTaken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The generated constructors register AssertExpectations on cleanup.
			creator := mocks.NewMockUserCreator(t)
			hasher := mocks.NewMockPasswordHasher(t)

			var storedRec usecase.CreateUserRecord

			if tt.setup != nil {
				tt.setup(creator, hasher)
			} else {
				hasher.EXPECT().Hash("supersecret").Return("hashed:supersecret", nil).Once()
				creator.EXPECT().
					CreateUser(mock.Anything, mock.Anything).
					Run(func(_ context.Context, rec usecase.CreateUserRecord) {
						storedRec = rec
					}).
					Return(entity.User{Email: "alice@example.com"}, nil).
					Once()
			}

			in := validInput()
			if tt.mutate != nil {
				tt.mutate(&in)
			}

			svc := usecase.NewUser(creator, hasher)
			_, err := svc.Register(context.Background(), in)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			if tt.wantNoStorage {
				creator.AssertNotCalled(t, "CreateUser")
			}

			if tt.wantStored != nil {
				tt.wantStored(t, storedRec)
			}
		})
	}
}
