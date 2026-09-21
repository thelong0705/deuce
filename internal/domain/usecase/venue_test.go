package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/domain/entity"
	"github.com/thelong0705/deuce/internal/domain/usecase"
	"github.com/thelong0705/deuce/internal/domain/usecase/mocks"
)

var ownerID = uuid.New()

func validVenueInput() entity.CreateVenueInput {
	return entity.CreateVenueInput{
		OwnerID:  ownerID,
		Name:     "Ace Tennis Club",
		City:     "Hanoi",
		Address:  "12 Le Loi",
		Timezone: "Asia/Ho_Chi_Minh",
	}
}

func activeOwner() *entity.User {
	return &entity.User{ID: ownerID, Role: entity.RoleOwner, IsActive: true}
}

// noVenueMocks sets no expectations, so the mock panics if either dependency
// is touched.
func noVenueMocks(*mocks.MockVenueRepo, *mocks.MockUserFinder) {}

func TestVenueCreate(t *testing.T) {
	boom := errors.New("boom")

	tests := []struct {
		name string
		// mutate adjusts the valid input for this case
		mutate func(in *entity.CreateVenueInput)
		// setup configures the mocks; nil means the happy path
		setup func(repo *mocks.MockVenueRepo, finder *mocks.MockUserFinder)
		// wantErr is what Create must return
		wantErr error
		// wantStored asserts what was handed to storage
		wantStored func(t *testing.T, in entity.CreateVenueInput)
		// wantNoStorage asserts the repo was never reached
		wantNoStorage bool
	}{
		{
			name: "stores the venue for an active owner",
			wantStored: func(t *testing.T, in entity.CreateVenueInput) {
				require.Equal(t, ownerID, in.OwnerID)
				require.Equal(t, "Ace Tennis Club", in.Name)
				require.Equal(t, "Hanoi", in.City)
				require.Equal(t, "12 Le Loi", in.Address)
			},
		},
		{
			name:          "rejects a missing owner",
			mutate:        func(in *entity.CreateVenueInput) { in.OwnerID = uuid.Nil },
			setup:         noVenueMocks,
			wantErr:       entity.ErrOwnerRequired,
			wantNoStorage: true,
		},
		{
			name:          "rejects a blank name",
			mutate:        func(in *entity.CreateVenueInput) { in.Name = "  " },
			setup:         noVenueMocks,
			wantErr:       entity.ErrVenueNameRequired,
			wantNoStorage: true,
		},
		{
			name:          "rejects a blank city",
			mutate:        func(in *entity.CreateVenueInput) { in.City = "" },
			setup:         noVenueMocks,
			wantErr:       entity.ErrVenueCityRequired,
			wantNoStorage: true,
		},
		{
			name:          "rejects a blank address",
			mutate:        func(in *entity.CreateVenueInput) { in.Address = "" },
			setup:         noVenueMocks,
			wantErr:       entity.ErrVenueAddressRequired,
			wantNoStorage: true,
		},
		{
			name: "rejects a player",
			setup: func(_ *mocks.MockVenueRepo, finder *mocks.MockUserFinder) {
				finder.EXPECT().GetUser(mock.Anything, ownerID).
					Return(&entity.User{ID: ownerID, Role: entity.RolePlayer, IsActive: true}, nil).Once()
			},
			wantErr:       entity.ErrNotAnOwner,
			wantNoStorage: true,
		},
		{
			name: "rejects a deactivated owner",
			setup: func(_ *mocks.MockVenueRepo, finder *mocks.MockUserFinder) {
				finder.EXPECT().GetUser(mock.Anything, ownerID).
					Return(&entity.User{ID: ownerID, Role: entity.RoleOwner, IsActive: false}, nil).Once()
			},
			wantErr:       entity.ErrOwnerInactive,
			wantNoStorage: true,
		},
		{
			name: "propagates a lookup failure",
			setup: func(_ *mocks.MockVenueRepo, finder *mocks.MockUserFinder) {
				finder.EXPECT().GetUser(mock.Anything, ownerID).Return(nil, boom).Once()
			},
			wantErr:       boom,
			wantNoStorage: true,
		},
		{
			name: "propagates a repo failure",
			setup: func(repo *mocks.MockVenueRepo, finder *mocks.MockUserFinder) {
				finder.EXPECT().GetUser(mock.Anything, ownerID).Return(activeOwner(), nil).Once()
				repo.EXPECT().CreateVenue(mock.Anything, mock.Anything).Return(nil, boom).Once()
			},
			wantErr: boom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewMockVenueRepo(t)
			finder := mocks.NewMockUserFinder(t)

			var storedInput entity.CreateVenueInput

			if tt.setup != nil {
				tt.setup(repo, finder)
			} else {
				finder.EXPECT().GetUser(mock.Anything, ownerID).Return(activeOwner(), nil).Once()
				repo.EXPECT().
					CreateVenue(mock.Anything, mock.Anything).
					Run(func(_ context.Context, in entity.CreateVenueInput) {
						storedInput = in
					}).
					Return(&entity.Venue{Name: "Ace Tennis Club"}, nil).
					Once()
			}

			in := validVenueInput()
			if tt.mutate != nil {
				tt.mutate(&in)
			}

			svc := usecase.NewVenue(repo, finder)
			_, err := svc.Create(context.Background(), in)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			if tt.wantNoStorage {
				repo.AssertNotCalled(t, "CreateVenue")
			}

			if tt.wantStored != nil {
				tt.wantStored(t, storedInput)
			}
		})
	}
}

func TestVenueListByOwner(t *testing.T) {
	boom := errors.New("boom")

	tests := []struct {
		name    string
		ownerID uuid.UUID
		setup   func(repo *mocks.MockVenueRepo)
		wantErr error
		wantLen int
	}{
		{
			name:    "returns the owner's venues",
			ownerID: ownerID,
			setup: func(repo *mocks.MockVenueRepo) {
				repo.EXPECT().ListVenuesByOwner(mock.Anything, ownerID).
					Return([]entity.Venue{{Name: "Ace"}, {Name: "Deuce"}}, nil).Once()
			},
			wantLen: 2,
		},
		{
			name:    "an owner with no venues returns an empty slice",
			ownerID: ownerID,
			setup: func(repo *mocks.MockVenueRepo) {
				repo.EXPECT().ListVenuesByOwner(mock.Anything, ownerID).
					Return([]entity.Venue{}, nil).Once()
			},
		},
		{
			name:    "rejects a nil owner without querying",
			ownerID: uuid.Nil,
			setup:   func(*mocks.MockVenueRepo) {},
			wantErr: entity.ErrOwnerRequired,
		},
		{
			name:    "propagates a storage failure",
			ownerID: ownerID,
			setup: func(repo *mocks.MockVenueRepo) {
				repo.EXPECT().ListVenuesByOwner(mock.Anything, ownerID).Return(nil, boom).Once()
			},
			wantErr: boom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewMockVenueRepo(t)
			tt.setup(repo)

			svc := usecase.NewVenue(repo, mocks.NewMockUserFinder(t))
			got, err := svc.ListByOwner(context.Background(), tt.ownerID)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				repo.AssertNotCalled(t, "ListVenuesByOwner")
				return
			}

			require.NoError(t, err)
			require.Len(t, got, tt.wantLen)
		})
	}
}
