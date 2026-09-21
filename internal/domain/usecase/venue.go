package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

// VenueRepo stores and reads back venues.
type VenueRepo interface {
	CreateVenue(ctx context.Context, in entity.CreateVenueInput) (*entity.Venue, error)
	ListVenuesByOwner(ctx context.Context, ownerID uuid.UUID) ([]entity.Venue, error)
}

// UserFinder looks up a user by id.
type UserFinder interface {
	GetUser(ctx context.Context, id uuid.UUID) (*entity.User, error)
}

type Venue struct {
	venueRepo  VenueRepo
	userFinder UserFinder
}

func NewVenue(venueRepo VenueRepo, userFinder UserFinder) *Venue {
	return &Venue{venueRepo: venueRepo, userFinder: userFinder}
}

// Create validates the input, checks the owner may register venues, and
// persists the venue.
func (s *Venue) Create(ctx context.Context, in entity.CreateVenueInput) (*entity.Venue, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}

	owner, err := s.userFinder.GetUser(ctx, in.OwnerID)
	if err != nil {
		return nil, err
	}

	if owner.Role != entity.RoleOwner {
		return nil, entity.ErrNotAnOwner
	}

	if !owner.IsActive {
		return nil, entity.ErrOwnerInactive
	}

	return s.venueRepo.CreateVenue(ctx, in)
}

// ListByOwner returns every venue owned by a user, active or not.
func (s *Venue) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]entity.Venue, error) {
	if ownerID == uuid.Nil {
		return nil, entity.ErrOwnerRequired
	}

	return s.venueRepo.ListVenuesByOwner(ctx, ownerID)
}
