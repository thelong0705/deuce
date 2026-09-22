package entity_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

func validBookSlotInput() entity.BookSlotInput {
	return entity.BookSlotInput{
		PlayerID: uuid.New(),
		CourtID:  uuid.New(),
		StartsAt: time.Now().Add(time.Hour).Truncate(time.Hour),
	}
}

func TestBookSlotInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(in *entity.BookSlotInput)
		wantErr error
	}{
		{
			name:   "valid input",
			mutate: func(*entity.BookSlotInput) {},
		},
		{
			name:    "missing player",
			mutate:  func(in *entity.BookSlotInput) { in.PlayerID = uuid.Nil },
			wantErr: entity.ErrPlayerRequired,
		},
		{
			name:    "missing court",
			mutate:  func(in *entity.BookSlotInput) { in.CourtID = uuid.Nil },
			wantErr: entity.ErrCourtRequired,
		},
		{
			name:    "missing start time",
			mutate:  func(in *entity.BookSlotInput) { in.StartsAt = time.Time{} },
			wantErr: entity.ErrSlotRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validBookSlotInput()
			tt.mutate(&in)

			err := in.Validate()

			if tt.wantErr == nil {
				require.NoError(t, err)
				return
			}

			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestBookingIsActive(t *testing.T) {
	cancelled := time.Now()

	tests := []struct {
		name string
		in   entity.Booking
		want bool
	}{
		{
			name: "a booking with no cancellation holds its slot",
			in:   entity.Booking{},
			want: true,
		},
		{
			name: "a cancelled booking does not",
			in:   entity.Booking{CancelledAt: &cancelled},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.in.IsActive())
		})
	}
}

func TestBookingEndsAt(t *testing.T) {
	start := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)

	got := entity.Booking{StartsAt: start}.EndsAt()

	require.Equal(t, start.Add(time.Hour), got)
}

func TestCourtValidateSlot(t *testing.T) {
	hcm, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	require.NoError(t, err)

	// 09:00 in Ho Chi Minh City, comfortably inside a 6-22 court.
	now := time.Date(2026, 9, 22, 8, 0, 0, 0, hcm)
	slot := time.Date(2026, 9, 22, 9, 0, 0, 0, hcm)

	court := entity.Court{OpenHour: 6, CloseHour: 22, IsActive: true}

	tests := []struct {
		name     string
		court    func(c *entity.Court)
		startsAt time.Time
		wantErr  error
	}{
		{
			name:     "a slot inside opening hours",
			startsAt: slot,
		},
		{
			name:     "the first slot of the day",
			startsAt: time.Date(2026, 9, 23, 6, 0, 0, 0, hcm),
		},
		{
			// The slot runs an hour, so 21:00-22:00 is the last one.
			name:     "the last slot of the day",
			startsAt: time.Date(2026, 9, 23, 21, 0, 0, 0, hcm),
		},
		{
			name:     "a slot expressed in another timezone is still local 09:00",
			startsAt: slot.UTC(),
		},
		{
			name:     "exactly two weeks ahead",
			startsAt: slot.Add(entity.BookingHorizon - time.Hour),
		},
		{
			name:     "half past the hour",
			startsAt: slot.Add(30 * time.Minute),
			wantErr:  entity.ErrSlotNotOnTheHour,
		},
		{
			name:     "a slot that already started",
			startsAt: now.Add(-time.Hour),
			wantErr:  entity.ErrSlotInThePast,
		},
		{
			name:     "the slot starting exactly now",
			startsAt: time.Date(2026, 9, 22, 8, 0, 0, 0, hcm),
			wantErr:  entity.ErrSlotInThePast,
		},
		{
			name:     "more than two weeks ahead",
			startsAt: slot.Add(entity.BookingHorizon),
			wantErr:  entity.ErrSlotTooFarAhead,
		},
		{
			name:     "before opening",
			startsAt: time.Date(2026, 9, 22, 5, 0, 0, 0, hcm),
			wantErr:  entity.ErrSlotInThePast,
		},
		{
			name:     "before opening on a later day",
			startsAt: time.Date(2026, 9, 23, 5, 0, 0, 0, hcm),
			wantErr:  entity.ErrSlotOutsideOpeningHours,
		},
		{
			// 22:00-23:00 would run past closing.
			name:     "starting at the closing hour",
			startsAt: time.Date(2026, 9, 23, 22, 0, 0, 0, hcm),
			wantErr:  entity.ErrSlotOutsideOpeningHours,
		},
		{
			name:     "a deactivated court",
			court:    func(c *entity.Court) { c.IsActive = false },
			startsAt: slot,
			wantErr:  entity.ErrCourtInactive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := court
			if tt.court != nil {
				tt.court(&c)
			}

			err := c.ValidateSlot(tt.startsAt, hcm, now)

			if tt.wantErr == nil {
				require.NoError(t, err)
				return
			}

			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

// The venue's timezone decides which local hour a slot falls in, so the same
// instant is inside one venue's opening hours and outside another's.
func TestCourtValidateSlotReadsHoursInTheVenueTimezone(t *testing.T) {
	hcm, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	require.NoError(t, err)

	court := entity.Court{OpenHour: 6, CloseHour: 22, IsActive: true}

	// 23:00 UTC is 06:00 the next morning in Ho Chi Minh City: the first slot
	// of that court's day, and an hour after the same court would have closed
	// had the venue been in UTC.
	slot := time.Date(2026, 9, 22, 23, 0, 0, 0, time.UTC)
	now := slot.Add(-time.Hour)

	require.NoError(t, court.ValidateSlot(slot, hcm, now))
	require.ErrorIs(t, court.ValidateSlot(slot, time.UTC, now), entity.ErrSlotOutsideOpeningHours)
}

func TestCourtSlotsOn(t *testing.T) {
	hcm, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	require.NoError(t, err)

	// Well before the court opens, so nothing is refused for being past.
	day := time.Date(2026, 9, 22, 0, 0, 0, 0, hcm)
	now := day.Add(-24 * time.Hour)

	tests := []struct {
		name  string
		court entity.Court
		day   time.Time
		now   time.Time
		want  []int
	}{
		{
			name:  "every open hour, last one starting before closing",
			court: entity.Court{OpenHour: 6, CloseHour: 10, IsActive: true},
			want:  []int{6, 7, 8, 9},
		},
		{
			name:  "a court open one hour offers one slot",
			court: entity.Court{OpenHour: 6, CloseHour: 7, IsActive: true},
			want:  []int{6},
		},
		{
			name:  "an inactive court offers nothing",
			court: entity.Court{OpenHour: 6, CloseHour: 10},
			want:  nil,
		},
		{
			name:  "hours already gone are absent, not unavailable",
			court: entity.Court{OpenHour: 6, CloseHour: 10, IsActive: true},
			now:   time.Date(2026, 9, 22, 8, 30, 0, 0, hcm),
			want:  []int{9},
		},
		{
			name:  "a day past the horizon offers nothing",
			court: entity.Court{OpenHour: 6, CloseHour: 10, IsActive: true},
			day:   day.AddDate(0, 0, 30),
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			on, at := day, now
			if !tt.day.IsZero() {
				on = tt.day
			}
			if !tt.now.IsZero() {
				at = tt.now
			}

			slots := tt.court.SlotsOn(on, hcm, at)

			hours := make([]int, 0, len(slots))
			for _, slot := range slots {
				require.Equal(t, hcm, slot.Location())
				require.Zero(t, slot.Minute())
				hours = append(hours, slot.Hour())
			}

			if tt.want == nil {
				require.Empty(t, hours)
				return
			}
			require.Equal(t, tt.want, hours)
		})
	}
}

// Only the calendar date of the argument is read. A caller holding midnight
// UTC means that date at the venue, not the span of time UTC calls it.
func TestCourtSlotsOnResolvesTheDateInTheVenueTimezone(t *testing.T) {
	hcm, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	require.NoError(t, err)

	court := entity.Court{OpenHour: 6, CloseHour: 8, IsActive: true}
	now := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)

	slots := court.SlotsOn(time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC), hcm, now)

	require.Len(t, slots, 2)
	require.Equal(t, time.Date(2026, 9, 22, 6, 0, 0, 0, hcm), slots[0])
	// 06:00 in Ho Chi Minh City is 23:00 the evening before in UTC.
	require.Equal(t, time.Date(2026, 9, 21, 23, 0, 0, 0, time.UTC), slots[0].UTC())
}

// Whatever SlotsOn offers, Book must accept, or the two tell a player
// different stories about the same hour.
func TestCourtSlotsOnAgreesWithValidateSlot(t *testing.T) {
	hcm, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	require.NoError(t, err)

	court := entity.Court{OpenHour: 0, CloseHour: 24, IsActive: true}
	now := time.Date(2026, 9, 22, 11, 17, 0, 0, hcm)

	for _, day := range []time.Time{now, now.AddDate(0, 0, 1), now.AddDate(0, 0, 14)} {
		for _, slot := range court.SlotsOn(day, hcm, now) {
			require.NoError(t, court.ValidateSlot(slot, hcm, now), "offered %s", slot)
		}
	}
}
