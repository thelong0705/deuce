package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestCreateUser(t *testing.T) {
	tests := []struct {
		name    string
		arg     func(t *testing.T) CreateUserParams
		wantErr bool
	}{
		{
			name: "player",
			arg: func(t *testing.T) CreateUserParams {
				return CreateUserParams{
					Email:        randomEmail(),
					PasswordHash: gofakeit.Password(true, true, true, false, false, 32),
					PhoneNumber:  randomPhone(),
					DisplayName:  gofakeit.Name(),
					Role:         UserRolePlayer,
				}
			},
		},
		{
			name: "owner",
			arg: func(t *testing.T) CreateUserParams {
				return CreateUserParams{
					Email:        randomEmail(),
					PasswordHash: gofakeit.Password(true, true, true, false, false, 32),
					PhoneNumber:  randomPhone(),
					DisplayName:  gofakeit.Name(),
					Role:         UserRoleOwner,
				}
			},
		},
		{
			name: "duplicate email",
			arg: func(t *testing.T) CreateUserParams {
				existing := createRandomUser(t, UserRolePlayer)
				return CreateUserParams{
					Email:        existing.Email,
					PasswordHash: gofakeit.Password(true, true, true, false, false, 32),
					PhoneNumber:  randomPhone(),
					DisplayName:  gofakeit.Name(),
					Role:         UserRolePlayer,
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arg := tt.arg(t)

			user, err := testQueries.CreateUser(context.Background(), arg)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, arg.Email, user.Email)
			require.Equal(t, arg.PasswordHash, user.PasswordHash)
			require.Equal(t, arg.DisplayName, user.DisplayName)
			require.Equal(t, arg.Role, user.Role)

			require.NotEqual(t, uuid.Nil, user.ID)
			require.True(t, user.IsActive)
			require.True(t, user.CreatedAt.Valid)
			require.Equal(t, arg.PhoneNumber, user.PhoneNumber)
		})
	}
}

func TestGetUser(t *testing.T) {
	tests := []struct {
		name    string
		user    func(t *testing.T) User
		wantErr error
	}{
		{
			name: "existing user",
			user: func(t *testing.T) User {
				return createRandomUser(t, UserRolePlayer)
			},
		},
		{
			name: "unknown id",
			user: func(t *testing.T) User {
				return User{ID: uuid.New()}
			},
			wantErr: pgx.ErrNoRows,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := tt.user(t)

			got, err := testQueries.GetUser(context.Background(), want.ID)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, want.ID, got.ID)
			require.Equal(t, want.Email, got.Email)
			require.Equal(t, want.DisplayName, got.DisplayName)
			require.Equal(t, want.Role, got.Role)
		})
	}
}

func TestGetUserByEmail(t *testing.T) {
	tests := []struct {
		name    string
		user    func(t *testing.T) User
		wantErr error
	}{
		{
			name: "existing user",
			user: func(t *testing.T) User {
				return createRandomUser(t, UserRoleOwner)
			},
		},
		{
			name: "unknown email",
			user: func(t *testing.T) User {
				return User{Email: randomEmail()}
			},
			wantErr: pgx.ErrNoRows,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := tt.user(t)

			got, err := testQueries.GetUserByEmail(context.Background(), want.Email)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, want.ID, got.ID)
			require.Equal(t, want.Email, got.Email)
		})
	}
}

func TestUpdateUserProfile(t *testing.T) {
	tests := []struct {
		name           string
		newDisplayName string
	}{
		{"simple name", "Alice Nguyen"},
		{"single word", "Deuce"},
		{"non-ascii", "Nguyễn Văn Long"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created := createRandomUser(t, UserRolePlayer)

			updated, err := testQueries.UpdateUserProfile(context.Background(), UpdateUserProfileParams{
				ID:          created.ID,
				DisplayName: tt.newDisplayName,
			})
			require.NoError(t, err)

			require.Equal(t, created.ID, updated.ID)
			require.Equal(t, tt.newDisplayName, updated.DisplayName)

			require.Equal(t, created.Email, updated.Email)
			require.Equal(t, created.Role, updated.Role)
		})
	}
}

func randomPhone() string {
	return fmt.Sprintf("+849%08d", gofakeit.Number(0, 99999999))
}

func createRandomUser(t *testing.T, role UserRole) User {
	t.Helper()

	user, err := testQueries.CreateUser(context.Background(), CreateUserParams{
		Email:        randomEmail(),
		PasswordHash: gofakeit.Password(true, true, true, false, false, 32),
		PhoneNumber:  randomPhone(),
		DisplayName:  gofakeit.Name(),
		Role:         role,
	})
	require.NoError(t, err)

	return user
}
