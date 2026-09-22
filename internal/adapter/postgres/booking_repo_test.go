package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

func createRandomCourt(t *testing.T) Court {
	t.Helper()

	court, err := testQueries.CreateCourt(context.Background(), CreateCourtParams{
		VenueID:      createRandomVenue(t).ID,
		Name:         gofakeit.Noun() + " " + gofakeit.LetterN(6),
		OpenHour:     0,
		CloseHour:    24,
		PricePerHour: 120000,
		Currency:     CurrencyVND,
	})
	require.NoError(t, err)

	return court
}

func createRandomPlayer(t *testing.T) uuid.UUID {
	t.Helper()
	return createRandomUser(t, UserRolePlayer).ID
}

// nextSlot is the top of an hour far enough ahead that two tests running back
// to back do not pick the same one.
func nextSlot() time.Time {
	return time.Now().UTC().Truncate(time.Hour).Add(time.Duration(gofakeit.Number(1, 300)) * time.Hour)
}

func validBookSlotInput(t *testing.T) entity.BookSlotInput {
	t.Helper()

	return entity.BookSlotInput{
		PlayerID: createRandomPlayer(t),
		CourtID:  createRandomCourt(t).ID,
		StartsAt: nextSlot(),
	}
}

func TestBookingRepositoryCreateBooking(t *testing.T) {
	repo := NewBookingRepository(testQueries)

	tests := []struct {
		name    string
		in      func(t *testing.T) entity.BookSlotInput
		wantErr error
		// wantAnyErr covers failures Postgres reports but the domain has no
		// name for
		wantAnyErr bool
		check      func(t *testing.T, in entity.BookSlotInput, got *entity.Booking)
	}{
		{
			name: "books the slot",
			in:   validBookSlotInput,
			check: func(t *testing.T, in entity.BookSlotInput, got *entity.Booking) {
				require.Equal(t, in.CourtID, got.CourtID)
				require.Equal(t, in.PlayerID, got.PlayerID)
				require.True(t, in.StartsAt.Equal(got.StartsAt))
				require.False(t, got.IsBlock)
				require.True(t, got.IsActive())
				// Nothing is charged for yet, so the row owes nothing and has
				// no payment attached.
				require.Equal(t, entity.StatusConfirmed, got.Status)
				require.Nil(t, got.Amount)
				require.Nil(t, got.HoldExpiresAt)
				require.Empty(t, got.PaymentIntentID)
			},
		},
		{
			name: "storage assigns id and timestamp",
			in:   validBookSlotInput,
			check: func(t *testing.T, _ entity.BookSlotInput, got *entity.Booking) {
				require.NotEqual(t, uuid.Nil, got.ID)
				require.False(t, got.CreatedAt.IsZero())
			},
		},
		{
			name: "an unknown court is rejected by the foreign key",
			in: func(t *testing.T) entity.BookSlotInput {
				in := validBookSlotInput(t)
				in.CourtID = uuid.New()
				return in
			},
			wantAnyErr: true,
		},
		{
			name: "a slot already taken becomes ErrSlotTaken",
			in: func(t *testing.T) entity.BookSlotInput {
				in := validBookSlotInput(t)

				_, err := repo.CreateBooking(context.Background(), in)
				require.NoError(t, err)

				// A different player going for the same court and hour.
				in.PlayerID = createRandomPlayer(t)
				return in
			},
			wantErr: entity.ErrSlotTaken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := tt.in(t)

			got, err := repo.CreateBooking(context.Background(), in)

			switch {
			case tt.wantErr != nil:
				require.ErrorIs(t, err, tt.wantErr)
				return
			case tt.wantAnyErr:
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			tt.check(t, in, got)
		})
	}
}

// The slot is held by a unique index rather than by a read-then-write check, so
// concurrent bookings for one slot settle in the database: exactly one wins and
// the rest are told the slot is taken.
func TestBookingRepositoryCreateBookingSettlesRacesInTheDatabase(t *testing.T) {
	const players = 8

	repo := NewBookingRepository(testQueries)

	courtID := createRandomCourt(t).ID
	slot := nextSlot()

	playerIDs := make([]uuid.UUID, players)
	for i := range playerIDs {
		playerIDs[i] = createRandomPlayer(t)
	}

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		won      []*entity.Booking
		lost     int
		unwanted []error
	)

	// The goroutines are all built before any of them runs, so they go for the
	// slot together rather than in the order they were started.
	start := make(chan struct{})

	for _, playerID := range playerIDs {
		wg.Add(1)

		go func(playerID uuid.UUID) {
			defer wg.Done()
			<-start

			got, err := repo.CreateBooking(context.Background(), entity.BookSlotInput{
				PlayerID: playerID,
				CourtID:  courtID,
				StartsAt: slot,
			})

			mu.Lock()
			defer mu.Unlock()

			switch {
			case err == nil:
				won = append(won, got)
			case errors.Is(err, entity.ErrSlotTaken):
				lost++
			default:
				unwanted = append(unwanted, err)
			}
		}(playerID)
	}

	close(start)
	wg.Wait()

	require.Empty(t, unwanted, "the losers must be told the slot is taken, not fail some other way")
	require.Len(t, won, 1, "exactly one booking may hold the slot")
	require.Equal(t, players-1, lost)
	require.Contains(t, playerIDs, won[0].PlayerID)
}

func TestBookingRepositoryGetCourtWithVenue(t *testing.T) {
	repo := NewBookingRepository(testQueries)

	tests := []struct {
		name    string
		courtID func(t *testing.T) uuid.UUID
		wantErr error
	}{
		{
			name:    "returns the court and the venue it belongs to",
			courtID: func(t *testing.T) uuid.UUID { return createRandomCourt(t).ID },
		},
		{
			name:    "an unknown id becomes ErrCourtNotFound",
			courtID: func(*testing.T) uuid.UUID { return uuid.New() },
			wantErr: entity.ErrCourtNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := tt.courtID(t)

			court, venue, err := repo.GetCourtWithVenue(context.Background(), id)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, id, court.ID)
			require.Equal(t, court.VenueID, venue.ID)
			require.Equal(t, "Asia/Ho_Chi_Minh", venue.Timezone)
		})
	}
}

func TestBookingRepositoryListBookedSlots(t *testing.T) {
	repo := NewBookingRepository(testQueries)
	ctx := context.Background()

	court := createRandomCourt(t)
	player := createRandomPlayer(t)

	base := nextSlot()
	first, second := base, base.Add(2*time.Hour)

	for _, at := range []time.Time{first, second} {
		_, err := repo.CreateBooking(ctx, entity.BookSlotInput{
			PlayerID: player, CourtID: court.ID, StartsAt: at,
		})
		require.NoError(t, err)
	}

	// A cancelled booking no longer holds its slot.
	cancelled := base.Add(4 * time.Hour)
	booking, err := repo.CreateBooking(ctx, entity.BookSlotInput{
		PlayerID: player, CourtID: court.ID, StartsAt: cancelled,
	})
	require.NoError(t, err)
	cancelBooking(t, booking.ID)

	tests := []struct {
		name      string
		courtID   uuid.UUID
		from, to  time.Time
		wantSlots []time.Time
	}{
		{
			name:      "the slots held in the window",
			courtID:   court.ID,
			from:      base,
			to:        base.Add(5 * time.Hour),
			wantSlots: []time.Time{first, second},
		},
		{
			name:      "from is inclusive and to is exclusive",
			courtID:   court.ID,
			from:      first,
			to:        second,
			wantSlots: []time.Time{first},
		},
		{
			name:      "another court's bookings are not counted",
			courtID:   createRandomCourt(t).ID,
			from:      base,
			to:        base.Add(5 * time.Hour),
			wantSlots: []time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.ListBookedSlots(ctx, tt.courtID, tt.from, tt.to)
			require.NoError(t, err)

			require.Len(t, got, len(tt.wantSlots))
			for i, want := range tt.wantSlots {
				require.True(t, want.Equal(got[i]), "want %s, got %s", want, got[i])
			}
		})
	}
}

func TestBookingRepositoryListPlayerBookings(t *testing.T) {
	repo := NewBookingRepository(testQueries)
	ctx := context.Background()

	court := createRandomCourt(t)
	player := createRandomPlayer(t)

	at := nextSlot()
	_, err := repo.CreateBooking(ctx, entity.BookSlotInput{
		PlayerID: player, CourtID: court.ID, StartsAt: at,
	})
	require.NoError(t, err)

	// Another player's booking on the same court.
	_, err = repo.CreateBooking(ctx, entity.BookSlotInput{
		PlayerID: createRandomPlayer(t), CourtID: court.ID, StartsAt: at.Add(time.Hour),
	})
	require.NoError(t, err)

	t.Run("carries the court and the venue, not just ids", func(t *testing.T) {
		got, err := repo.ListPlayerBookings(ctx, player, at.Add(-time.Hour))
		require.NoError(t, err)
		require.Len(t, got, 1)

		require.Equal(t, player, got[0].PlayerID)
		require.Equal(t, court.ID, got[0].Court.ID)
		require.Equal(t, court.Name, got[0].Court.Name)
		require.Equal(t, court.VenueID, got[0].Venue.ID)
		require.NotEmpty(t, got[0].Venue.Name)
		require.NotEmpty(t, got[0].Venue.City)
	})

	t.Run("slots before from are left out", func(t *testing.T) {
		got, err := repo.ListPlayerBookings(ctx, player, at.Add(time.Hour))
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("a cancelled booking is not listed", func(t *testing.T) {
		other := createRandomPlayer(t)
		booking, err := repo.CreateBooking(ctx, entity.BookSlotInput{
			PlayerID: other, CourtID: court.ID, StartsAt: at.Add(2 * time.Hour),
		})
		require.NoError(t, err)

		cancelBooking(t, booking.ID)

		got, err := repo.ListPlayerBookings(ctx, other, at.Add(-time.Hour))
		require.NoError(t, err)
		require.Empty(t, got)
	})
}

// cancelBooking writes cancelled_at directly: nothing in the application
// cancels yet, and these tests only need a cancelled row to read past.
func cancelBooking(t *testing.T, id uuid.UUID) {
	t.Helper()

	_, err := testPool.Exec(
		context.Background(), "UPDATE bookings SET cancelled_at = now() WHERE id = $1", id,
	)
	require.NoError(t, err)
}

// One query answers for every court at once, keyed so each court's slots stay
// its own.
func TestBookingRepositoryListBookedSlotsForCourts(t *testing.T) {
	repo := NewBookingRepository(testQueries)
	ctx := context.Background()

	first := createRandomCourt(t).ID
	second := createRandomCourt(t).ID
	untouched := createRandomCourt(t).ID

	slot := nextSlot()

	_, err := repo.CreateBooking(ctx, entity.BookSlotInput{
		PlayerID: createRandomPlayer(t), CourtID: first, StartsAt: slot,
	})
	require.NoError(t, err)

	_, err = repo.CreateBooking(ctx, entity.BookSlotInput{
		PlayerID: createRandomPlayer(t), CourtID: second, StartsAt: slot,
	})
	require.NoError(t, err)

	got, err := repo.ListBookedSlotsForCourts(ctx,
		[]uuid.UUID{first, second, untouched},
		slot, slot.Add(entity.SlotDuration),
	)
	require.NoError(t, err)

	require.Len(t, got[first], 1)
	require.True(t, slot.Equal(got[first][0]))
	require.Len(t, got[second], 1)
	require.Empty(t, got[untouched], "a court with nothing booked has no entry")
}

func TestBookingRepositoryListBookedSlotsForCourtsIsBoundedAndSkipsCancelled(t *testing.T) {
	repo := NewBookingRepository(testQueries)
	ctx := context.Background()

	courtID := createRandomCourt(t).ID
	slot := nextSlot()

	booking, err := repo.CreateBooking(ctx, entity.BookSlotInput{
		PlayerID: createRandomPlayer(t), CourtID: courtID, StartsAt: slot,
	})
	require.NoError(t, err)

	// Outside the window asked for.
	got, err := repo.ListBookedSlotsForCourts(ctx, []uuid.UUID{courtID},
		slot.Add(entity.SlotDuration), slot.Add(2*entity.SlotDuration))
	require.NoError(t, err)
	require.Empty(t, got[courtID])

	_, err = testPool.Exec(ctx, "UPDATE bookings SET cancelled_at = now() WHERE id = $1", booking.ID)
	require.NoError(t, err)

	// A cancelled booking holds nothing.
	got, err = repo.ListBookedSlotsForCourts(ctx, []uuid.UUID{courtID}, slot, slot.Add(entity.SlotDuration))
	require.NoError(t, err)
	require.Empty(t, got[courtID])
}
