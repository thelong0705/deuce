package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

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
		Currency:     entity.CurrencyVND,
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

			svc := usecase.NewCourt(repo, venues, mocks.NewMockBookedSlotFinder(t))
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

			svc := usecase.NewCourt(courts, venues, mocks.NewMockBookedSlotFinder(t))
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

// searchCourt builds a court at a venue in Hanoi, open all day, with a
// two-hour grid from midnight.
func searchCourt(t *testing.T, name string) entity.CourtAtVenue {
	t.Helper()

	return entity.CourtAtVenue{
		Court: entity.Court{
			ID:        uuid.New(),
			Name:      name,
			OpenHour:  0,
			CloseHour: 24,
			IsActive:  true,
		},
		Venue: entity.Venue{
			ID:       uuid.New(),
			Name:     "Venue " + name,
			City:     "Hanoi",
			Timezone: "UTC",
			IsActive: true,
		},
	}
}

func searchInput() entity.CourtSearch {
	return entity.CourtSearch{
		City: "Hanoi",
		// Tomorrow, so no hour of the day has already gone.
		Date:     time.Now().UTC().AddDate(0, 0, 1),
		FromHour: 0,
		ToHour:   24,
	}
}

func TestCourtSearch(t *testing.T) {
	boom := errors.New("boom")

	tests := []struct {
		name string
		// mutate adjusts the valid search for this case
		mutate func(in *entity.CourtSearch)
		setup  func(t *testing.T, repo *mocks.MockCourtRepo, booked *mocks.MockBookedSlotFinder)
		// wantErr is what Search must return
		wantErr error
		// wantCourts is how many courts come back
		wantCourts int
		check      func(t *testing.T, got []entity.CourtAvailability)
	}{
		{
			name: "returns the courts with something free",
			setup: func(t *testing.T, repo *mocks.MockCourtRepo, booked *mocks.MockBookedSlotFinder) {
				repo.EXPECT().SearchCourts(mock.Anything, "Hanoi").
					Return([]entity.CourtAtVenue{searchCourt(t, "1"), searchCourt(t, "2")}, nil).Once()
				booked.EXPECT().ListBookedSlotsForCourts(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(map[uuid.UUID][]time.Time{}, nil).Once()
			},
			wantCourts: 2,
			check: func(t *testing.T, got []entity.CourtAvailability) {
				require.NotEmpty(t, got[0].Slots)
				require.Equal(t, "Hanoi", got[0].Venue.City)
			},
		},
		{
			// The answer to "where can I play" is places that can take a
			// booking, so a court with nothing free is left out entirely.
			name: "a fully booked court is left out",
			setup: func(t *testing.T, repo *mocks.MockCourtRepo, booked *mocks.MockBookedSlotFinder) {
				court := searchCourt(t, "1")
				full := court.Court.SlotsOn(searchInput().Date, time.UTC, time.Now())

				repo.EXPECT().SearchCourts(mock.Anything, mock.Anything).
					Return([]entity.CourtAtVenue{court}, nil).Once()
				booked.EXPECT().ListBookedSlotsForCourts(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(map[uuid.UUID][]time.Time{court.Court.ID: full}, nil).Once()
			},
			wantCourts: 0,
		},
		{
			name: "only the free slots are reported",
			setup: func(t *testing.T, repo *mocks.MockCourtRepo, booked *mocks.MockBookedSlotFinder) {
				court := searchCourt(t, "1")
				all := court.Court.SlotsOn(searchInput().Date, time.UTC, time.Now())

				repo.EXPECT().SearchCourts(mock.Anything, mock.Anything).
					Return([]entity.CourtAtVenue{court}, nil).Once()
				booked.EXPECT().ListBookedSlotsForCourts(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(map[uuid.UUID][]time.Time{court.Court.ID: all[:1]}, nil).Once()
			},
			wantCourts: 1,
			check: func(t *testing.T, got []entity.CourtAvailability) {
				for _, slot := range got[0].Slots {
					require.True(t, slot.Available)
				}
			},
		},
		{
			name:   "the window narrows what is offered",
			mutate: func(in *entity.CourtSearch) { in.FromHour, in.ToHour = 18, 22 },
			setup: func(t *testing.T, repo *mocks.MockCourtRepo, booked *mocks.MockBookedSlotFinder) {
				repo.EXPECT().SearchCourts(mock.Anything, mock.Anything).
					Return([]entity.CourtAtVenue{searchCourt(t, "1")}, nil).Once()
				booked.EXPECT().ListBookedSlotsForCourts(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(map[uuid.UUID][]time.Time{}, nil).Once()
			},
			wantCourts: 1,
			check: func(t *testing.T, got []entity.CourtAvailability) {
				require.Len(t, got[0].Slots, 2)
				for _, slot := range got[0].Slots {
					hour := slot.StartsAt.UTC().Hour()
					require.GreaterOrEqual(t, hour, 18)
					require.Less(t, hour, 22)
				}
			},
		},
		{
			// Nothing to ask about, so storage is not asked.
			name: "a city with no courts asks nothing further",
			setup: func(t *testing.T, repo *mocks.MockCourtRepo, _ *mocks.MockBookedSlotFinder) {
				repo.EXPECT().SearchCourts(mock.Anything, mock.Anything).
					Return([]entity.CourtAtVenue{}, nil).Once()
			},
			wantCourts: 0,
		},
		{
			name:    "rejects a search with no city",
			mutate:  func(in *entity.CourtSearch) { in.City = "" },
			setup:   func(*testing.T, *mocks.MockCourtRepo, *mocks.MockBookedSlotFinder) {},
			wantErr: entity.ErrSearchCityRequired,
		},
		{
			name:   "propagates a search failure",
			mutate: func(*entity.CourtSearch) {},
			setup: func(t *testing.T, repo *mocks.MockCourtRepo, _ *mocks.MockBookedSlotFinder) {
				repo.EXPECT().SearchCourts(mock.Anything, mock.Anything).Return(nil, boom).Once()
			},
			wantErr: boom,
		},
		{
			name: "propagates a booked-slot failure",
			setup: func(t *testing.T, repo *mocks.MockCourtRepo, booked *mocks.MockBookedSlotFinder) {
				repo.EXPECT().SearchCourts(mock.Anything, mock.Anything).
					Return([]entity.CourtAtVenue{searchCourt(t, "1")}, nil).Once()
				booked.EXPECT().ListBookedSlotsForCourts(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, boom).Once()
			},
			wantErr: boom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewMockCourtRepo(t)
			booked := mocks.NewMockBookedSlotFinder(t)
			tt.setup(t, repo, booked)

			in := searchInput()
			if tt.mutate != nil {
				tt.mutate(&in)
			}

			svc := usecase.NewCourt(repo, mocks.NewMockVenueFinder(t), booked)
			got, err := svc.Search(context.Background(), in)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Len(t, got, tt.wantCourts)

			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

// One query covers every court, rather than one per court.
func TestCourtSearchAsksForAllCourtsAtOnce(t *testing.T) {
	repo := mocks.NewMockCourtRepo(t)
	booked := mocks.NewMockBookedSlotFinder(t)

	first, second := searchCourt(t, "1"), searchCourt(t, "2")

	repo.EXPECT().SearchCourts(mock.Anything, mock.Anything).
		Return([]entity.CourtAtVenue{first, second}, nil).Once()

	booked.EXPECT().
		ListBookedSlotsForCourts(mock.Anything, mock.MatchedBy(func(ids []uuid.UUID) bool {
			return len(ids) == 2
		}), mock.Anything, mock.Anything).
		Return(map[uuid.UUID][]time.Time{}, nil).
		Once()

	_, err := usecase.NewCourt(repo, mocks.NewMockVenueFinder(t), booked).
		Search(context.Background(), searchInput())

	require.NoError(t, err)
}
