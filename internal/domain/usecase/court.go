package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

// CourtRepo stores courts.
type CourtRepo interface {
	CreateCourt(ctx context.Context, in entity.CreateCourtInput) (*entity.Court, error)
	ListCourtsByVenue(ctx context.Context, venueID uuid.UUID) ([]entity.Court, error)
	// SearchCourts returns the active courts at the active venues in a city.
	SearchCourts(ctx context.Context, city string) ([]entity.CourtAtVenue, error)
}

// BookedSlotFinder reports the slots held across several courts, between from
// inclusive and to exclusive.
type BookedSlotFinder interface {
	ListBookedSlotsForCourts(ctx context.Context, courtIDs []uuid.UUID, from, to time.Time) (map[uuid.UUID][]time.Time, error)
}

// VenueFinder looks up a venue by id.
type VenueFinder interface {
	GetVenue(ctx context.Context, id uuid.UUID) (*entity.Venue, error)
}

type Court struct {
	courtRepo   CourtRepo
	venueFinder VenueFinder
	booked      BookedSlotFinder
}

func NewCourt(courtRepo CourtRepo, venueFinder VenueFinder, booked BookedSlotFinder) *Court {
	return &Court{courtRepo: courtRepo, venueFinder: venueFinder, booked: booked}
}

// Create validates the input, checks the caller owns an active venue, and
// persists the court.
func (s *Court) Create(ctx context.Context, in entity.CreateCourtInput) (*entity.Court, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}

	venue, err := s.venueFinder.GetVenue(ctx, in.VenueID)
	if err != nil {
		return nil, err
	}

	if venue.OwnerID != in.OwnerID {
		return nil, entity.ErrNotVenueOwner
	}

	if !venue.IsActive {
		return nil, entity.ErrVenueInactive
	}

	return s.courtRepo.CreateCourt(ctx, in)
}

// ListByVenue returns a venue's active courts. The venue is looked up first
// so an unknown id answers "not found" rather than an empty list.
func (s *Court) ListByVenue(ctx context.Context, venueID uuid.UUID) ([]entity.Court, error) {
	if venueID == uuid.Nil {
		return nil, entity.ErrVenueRequired
	}

	if _, err := s.venueFinder.GetVenue(ctx, venueID); err != nil {
		return nil, err
	}

	return s.courtRepo.ListCourtsByVenue(ctx, venueID)
}

// Search returns the courts in a city with a free slot on the date, between
// the hours, and which slots those are. A court with nothing free is left out:
// the answer is places that can take a booking.
//
// Like Availability it is a snapshot, and Book is where the race is settled.
func (s *Court) Search(ctx context.Context, in entity.CourtSearch) ([]entity.CourtAvailability, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}

	courts, err := s.courtRepo.SearchCourts(ctx, in.City)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	// Candidates first: they bound the one query for what is already taken.
	candidates := make(map[uuid.UUID][]time.Time, len(courts))
	courtIDs := make([]uuid.UUID, 0, len(courts))

	var from, to time.Time

	for _, cv := range courts {
		starts := cv.Court.SlotsWithin(in.Date, cv.Venue.Location(), now, in.FromHour, in.ToHour)
		if len(starts) == 0 {
			continue
		}

		candidates[cv.Court.ID] = starts
		courtIDs = append(courtIDs, cv.Court.ID)

		if from.IsZero() || starts[0].Before(from) {
			from = starts[0]
		}

		if end := starts[len(starts)-1].Add(entity.SlotDuration); to.IsZero() || end.After(to) {
			to = end
		}
	}

	if len(courtIDs) == 0 {
		return []entity.CourtAvailability{}, nil
	}

	taken, err := s.booked.ListBookedSlotsForCourts(ctx, courtIDs, from, to)
	if err != nil {
		return nil, err
	}

	results := make([]entity.CourtAvailability, 0, len(courtIDs))

	for _, cv := range courts {
		starts, ok := candidates[cv.Court.ID]
		if !ok {
			continue
		}

		// Keyed by instant, so a booking stored in one offset still matches a
		// slot built in another.
		held := make(map[int64]bool, len(taken[cv.Court.ID]))
		for _, at := range taken[cv.Court.ID] {
			held[at.UnixNano()] = true
		}

		free := make([]entity.Slot, 0, len(starts))
		for _, at := range starts {
			if !held[at.UnixNano()] {
				free = append(free, entity.Slot{StartsAt: at, Available: true})
			}
		}

		if len(free) == 0 {
			continue
		}

		results = append(results, entity.CourtAvailability{
			Court: cv.Court,
			Venue: cv.Venue,
			Slots: free,
		})
	}

	return results, nil
}
