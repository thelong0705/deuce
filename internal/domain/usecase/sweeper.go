package usecase

import (
	"context"
	"time"
)

// HoldReleaser cancels the holds that were not paid for in time.
type HoldReleaser interface {
	ReleaseLapsedHolds(ctx context.Context, asOf time.Time) (int, error)
}

// Sweeper puts back the slots nobody paid for. A hold occupies its slot until
// something cancels it, so without this an abandoned checkout blocks that hour
// for ever.
type Sweeper struct {
	holds HoldReleaser
}

func NewSweeper(holds HoldReleaser) *Sweeper {
	return &Sweeper{holds: holds}
}

// ReleaseLapsedHolds returns how many slots were put back.
func (s *Sweeper) ReleaseLapsedHolds(ctx context.Context, asOf time.Time) (int, error) {
	return s.holds.ReleaseLapsedHolds(ctx, asOf)
}
