package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

// CourtRepo stores courts.
type CourtRepo interface {
	CreateCourt(ctx context.Context, in entity.CreateCourtInput) (*entity.Court, error)
}

// VenueFinder looks up a venue by id.
type VenueFinder interface {
	GetVenue(ctx context.Context, id uuid.UUID) (*entity.Venue, error)
}

type Court struct {
	courtRepo   CourtRepo
	venueFinder VenueFinder
}

func NewCourt(courtRepo CourtRepo, venueFinder VenueFinder) *Court {
	return &Court{courtRepo: courtRepo, venueFinder: venueFinder}
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
