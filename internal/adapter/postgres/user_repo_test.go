package postgres

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/domain/entity"
	"github.com/thelong0705/deuce/internal/domain/usecase"
)

func validRecord() usecase.CreateUserRecord {
	return usecase.CreateUserRecord{
		Email:        randomEmail(),
		PasswordHash: gofakeit.Password(true, true, true, false, false, 32),
		DisplayName:  gofakeit.Name(),
		PhoneNumber:  randomPhone(),
		Role:         entity.RolePlayer,
	}
}

func TestUserRepositoryCreateUser(t *testing.T) {
	repo := NewUserRepository(testQueries)

	tests := []struct {
		name        string
		rec         func(t *testing.T) usecase.CreateUserRecord
		wantErr     error
		wantErrText string
		check       func(t *testing.T, rec usecase.CreateUserRecord, got *entity.User)
	}{
		{
			name: "creates a player",
			rec:  func(*testing.T) usecase.CreateUserRecord { return validRecord() },
			check: func(t *testing.T, rec usecase.CreateUserRecord, got *entity.User) {
				require.Equal(t, rec.Email, got.Email)
				require.Equal(t, rec.DisplayName, got.DisplayName)
				require.Equal(t, rec.PhoneNumber, got.PhoneNumber)
				require.Equal(t, entity.RolePlayer, got.Role)
			},
		},
		{
			name: "creates an owner",
			rec: func(*testing.T) usecase.CreateUserRecord {
				r := validRecord()
				r.Role = entity.RoleOwner
				return r
			},
			check: func(t *testing.T, _ usecase.CreateUserRecord, got *entity.User) {
				require.Equal(t, entity.RoleOwner, got.Role)
			},
		},
		{
			name: "storage assigns id, timestamp and active state",
			rec:  func(*testing.T) usecase.CreateUserRecord { return validRecord() },
			check: func(t *testing.T, _ usecase.CreateUserRecord, got *entity.User) {
				require.NotEqual(t, uuid.Nil, got.ID)
				require.False(t, got.CreatedAt.IsZero())
				// is_active defaults to true in the schema, which is why the
				// use case does not set it.
				require.True(t, got.IsActive)
			},
		},
		{
			name: "an unknown role is rejected before the insert",
			rec: func(*testing.T) usecase.CreateUserRecord {
				r := validRecord()
				r.Role = "admin"
				return r
			},
			wantErrText: "unknown role",
		},
		{
			name: "a duplicate email becomes ErrEmailTaken",
			rec: func(t *testing.T) usecase.CreateUserRecord {
				existing := validRecord()
				_, err := repo.CreateUser(context.Background(), existing)
				require.NoError(t, err)

				dup := validRecord()
				dup.Email = existing.Email
				return dup
			},
			wantErr: entity.ErrEmailTaken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := tt.rec(t)

			got, err := repo.CreateUser(context.Background(), rec)

			if tt.wantErrText != "" {
				require.ErrorContains(t, err, tt.wantErrText)
				return
			}

			if tt.wantErr != nil {
				// The point of this case: nothing above this layer should ever
				// have to know what SQLSTATE 23505 means.
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			tt.check(t, rec, got)
		})
	}
}

// The default branches are unreachable through the public API -- the column is
// a Postgres enum -- so they are exercised directly.
func TestRoleConversion(t *testing.T) {
	t.Run("to postgres", func(t *testing.T) {
		tests := []struct {
			in      entity.Role
			want    UserRole
			wantErr bool
		}{
			{in: entity.RolePlayer, want: UserRolePlayer},
			{in: entity.RoleOwner, want: UserRoleOwner},
			{in: "admin", wantErr: true},
			{in: "", wantErr: true},
		}

		for _, tt := range tests {
			t.Run(string(tt.in), func(t *testing.T) {
				got, err := toUserRole(tt.in)
				if tt.wantErr {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
			})
		}
	})

	t.Run("to entity", func(t *testing.T) {
		tests := []struct {
			in      UserRole
			want    entity.Role
			wantErr bool
		}{
			{in: UserRolePlayer, want: entity.RolePlayer},
			{in: UserRoleOwner, want: entity.RoleOwner},
			{in: "admin", wantErr: true},
			{in: "", wantErr: true},
		}

		for _, tt := range tests {
			t.Run(string(tt.in), func(t *testing.T) {
				got, err := toEntityRole(tt.in)
				if tt.wantErr {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
			})
		}
	})
}
