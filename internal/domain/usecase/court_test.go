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

var (
	courtOwnerID = uuid.New()
	courtVenueID = uuid.New()
)

func validCourtInput() entity.CreateCourtInput {
	return entity.CreateCourtInput{
		OwnerID:      courtOwnerID,
		VenueID:      courtVenueID,
		Name:         "Court 1",
		OpenHour:     6,
		CloseHour:    22,
		PricePerHour: 120000,
	}
}

func ownedVenue() *entity.Venue {
	return &entity.Venue{ID: courtVenueID, OwnerID: courtOwnerID, IsActive: true}
}

// noCourtMocks sets no expectations, so the mock panics if either dependency
// is touched.
func noCourtMocks(*mocks.MockCourtRepo, *mocks.MockVenueFinder) {}

func TestCourtCreate(t *testing.T) {
	boom := errors.New("boom")

	tests := []struct {
		name string
		// mutate adjusts the valid input for this case
		mutate func(in *entity.CreateCourtInput)
		// setup configures the mocks; nil means the happy path
		setup func(repo *mocks.MockCourtRepo, venues *mocks.MockVenueFinder)
		// wantErr is what Create must return
		wantErr error
		// wantStored asserts what was handed to storage
		wantStored func(t *testing.T, in entity.CreateCourtInput)
		// wantNoStorage asserts the repo was never reached
		wantNoStorage bool
	}{
		{
			name: "stores the court for the venue owner",
			wantStored: func(t *testing.T, in entity.CreateCourtInput) {
				require.Equal(t, courtVenueID, in.VenueID)
				require.Equal(t, "Court 1", in.Name)
				require.Equal(t, 6, in.OpenHour)
				require.Equal(t, 22, in.CloseHour)
				require.Equal(t, 120000, in.PricePerHour)
			},
		},
		{
			name:          "rejects invalid input before looking anything up",
			mutate:        func(in *entity.CreateCourtInput) { in.Name = "  " },
			setup:         noCourtMocks,
			wantErr:       entity.ErrCourtNameRequired,
			wantNoStorage: true,
		},
		{
			name: "rejects an unknown venue",
			setup: func(_ *mocks.MockCourtRepo, venues *mocks.MockVenueFinder) {
				venues.EXPECT().GetVenue(mock.Anything, courtVenueID).
					Return(nil, entity.ErrVenueNotFound).Once()
			},
			wantErr:       entity.ErrVenueNotFound,
			wantNoStorage: true,
		},
		{
			name: "rejects an owner who does not own the venue",
			setup: func(_ *mocks.MockCourtRepo, venues *mocks.MockVenueFinder) {
				venue := ownedVenue()
				venue.OwnerID = uuid.New()
				venues.EXPECT().GetVenue(mock.Anything, courtVenueID).Return(venue, nil).Once()
			},
			wantErr:       entity.ErrNotVenueOwner,
			wantNoStorage: true,
		},
		{
			name: "rejects a deactivated venue",
			setup: func(_ *mocks.MockCourtRepo, venues *mocks.MockVenueFinder) {
				venue := ownedVenue()
				venue.IsActive = false
				venues.EXPECT().GetVenue(mock.Anything, courtVenueID).Return(venue, nil).Once()
			},
			wantErr:       entity.ErrVenueInactive,
			wantNoStorage: true,
		},
		{
			name: "propagates a repo failure",
			setup: func(repo *mocks.MockCourtRepo, venues *mocks.MockVenueFinder) {
				venues.EXPECT().GetVenue(mock.Anything, courtVenueID).Return(ownedVenue(), nil).Once()
				repo.EXPECT().CreateCourt(mock.Anything, mock.Anything).Return(nil, boom).Once()
			},
			wantErr: boom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewMockCourtRepo(t)
			venues := mocks.NewMockVenueFinder(t)

			var storedInput entity.CreateCourtInput

			if tt.setup != nil {
				tt.setup(repo, venues)
			} else {
				venues.EXPECT().GetVenue(mock.Anything, courtVenueID).Return(ownedVenue(), nil).Once()
				repo.EXPECT().
					CreateCourt(mock.Anything, mock.Anything).
					Run(func(_ context.Context, in entity.CreateCourtInput) {
						storedInput = in
					}).
					Return(&entity.Court{Name: "Court 1"}, nil).
					Once()
			}

			in := validCourtInput()
			if tt.mutate != nil {
				tt.mutate(&in)
			}

			svc := usecase.NewCourt(repo, venues)
			_, err := svc.Create(context.Background(), in)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			if tt.wantNoStorage {
				repo.AssertNotCalled(t, "CreateCourt")
			}

			if tt.wantStored != nil {
				tt.wantStored(t, storedInput)
			}
		})
	}
}

func TestCourtListByVenue(t *testing.T) {
	boom := errors.New("boom")
	venueID := uuid.New()

	tests := []struct {
		name    string
		venueID uuid.UUID
		setup   func(courts *mocks.MockCourtRepo, venues *mocks.MockVenueFinder)
		wantLen int
		wantErr error
	}{
		{
			name:    "returns the venue's courts",
			venueID: venueID,
			setup: func(courts *mocks.MockCourtRepo, venues *mocks.MockVenueFinder) {
				venues.EXPECT().GetVenue(mock.Anything, venueID).
					Return(&entity.Venue{ID: venueID, IsActive: true}, nil).Once()
				courts.EXPECT().ListCourtsByVenue(mock.Anything, venueID).
					Return([]entity.Court{{}, {}}, nil).Once()
			},
			wantLen: 2,
		},
		{
			// An empty list would read as a venue with nothing at it.
			name:    "an unknown venue is not found, not empty",
			venueID: venueID,
			setup: func(_ *mocks.MockCourtRepo, venues *mocks.MockVenueFinder) {
				venues.EXPECT().GetVenue(mock.Anything, venueID).
					Return(nil, entity.ErrVenueNotFound).Once()
			},
			wantErr: entity.ErrVenueNotFound,
		},
		{
			name:    "an empty venue id is refused without a lookup",
			venueID: uuid.Nil,
			setup:   func(*mocks.MockCourtRepo, *mocks.MockVenueFinder) {},
			wantErr: entity.ErrVenueRequired,
		},
		{
			name:    "propagates a storage failure",
			venueID: venueID,
			setup: func(courts *mocks.MockCourtRepo, venues *mocks.MockVenueFinder) {
				venues.EXPECT().GetVenue(mock.Anything, venueID).
					Return(&entity.Venue{ID: venueID, IsActive: true}, nil).Once()
				courts.EXPECT().ListCourtsByVenue(mock.Anything, venueID).Return(nil, boom).Once()
			},
			wantErr: boom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			courts := mocks.NewMockCourtRepo(t)
			venues := mocks.NewMockVenueFinder(t)
			tt.setup(courts, venues)

			svc := usecase.NewCourt(courts, venues)
			got, err := svc.ListByVenue(context.Background(), tt.venueID)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Len(t, got, tt.wantLen)
		})
	}
}
