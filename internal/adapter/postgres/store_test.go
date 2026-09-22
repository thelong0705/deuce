package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestExecTx(t *testing.T) {
	errBoom := errors.New("boom")

	tests := []struct {
		name      string
		returnErr error
		wantRow   bool
	}{
		{
			name:    "commits when the callback succeeds",
			wantRow: true,
		},
		{
			name:      "rolls back when the callback fails",
			returnErr: errBoom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ownerID := createRandomOwner(t)

			var venueID uuid.UUID

			err := testStore.execTx(ctx, func(q *Queries) error {
				venue, err := q.CreateVenue(ctx, CreateVenueParams{
					OwnerID: ownerID,
					Name:    gofakeit.Company() + " Tennis Club",
					City:    supportedCity(),
					Address: gofakeit.Street(),
				})
				require.NoError(t, err)

				venueID = venue.ID
				return tt.returnErr
			})

			if tt.returnErr != nil {
				// The callback's error must reach the caller unwrapped enough
				// to still be matchable.
				require.ErrorIs(t, err, tt.returnErr)
			} else {
				require.NoError(t, err)
			}

			// Read through the pool, not the transaction, so this only sees
			// what was actually committed.
			_, getErr := testQueries.GetVenue(ctx, venueID)
			if tt.wantRow {
				require.NoError(t, getErr, "row should survive the commit")
			} else {
				require.ErrorIs(t, getErr, pgx.ErrNoRows, "row should be gone after rollback")
			}
		})
	}
}

// The callback gets a *Queries bound to the transaction, so its writes are not
// visible to anyone else until the commit.
func TestExecTxIsolatesUncommittedWrites(t *testing.T) {
	ctx := context.Background()
	ownerID := createRandomOwner(t)

	err := testStore.execTx(ctx, func(q *Queries) error {
		venue, err := q.CreateVenue(ctx, CreateVenueParams{
			OwnerID: ownerID,
			Name:    gofakeit.Company() + " Tennis Club",
			City:    supportedCity(),
			Address: gofakeit.Street(),
		})
		require.NoError(t, err)

		// Visible inside the transaction...
		_, err = q.GetVenue(ctx, venue.ID)
		require.NoError(t, err)

		// ...but not to a separate connection from the pool.
		_, err = testQueries.GetVenue(ctx, venue.ID)
		require.ErrorIs(t, err, pgx.ErrNoRows)

		return nil
	})
	require.NoError(t, err)
}
