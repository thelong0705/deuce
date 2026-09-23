//go:build e2e

package e2e

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestBookASlot(t *testing.T) {
	owner := newClient(t)
	owner.signUpAndLogIn("owner")

	venue := owner.createVenue()
	court := owner.createCourt(venue)

	player := newClient(t)
	player.signUpAndLogIn("player")

	slot := freeSlot(t)

	before := player.availabilityOn(court, slot)
	require.True(t, availableAt(t, before, slot), "the slot starts free")

	var held booking
	player.expect(http.StatusCreated, http.MethodPost,
		"/courts/"+court.ID.String()+"/bookings",
		map[string]string{"starts_at": slot.Format(time.RFC3339)}, &held)

	require.Equal(t, "pending_payment", held.Status)
	require.Equal(t, slot.UTC(), held.StartsAt.UTC())
	require.NotEmpty(t, held.ClientSecret)

	after := player.availabilityOn(court, slot)
	require.False(t, availableAt(t, after, slot), "a held slot is not offered again")
}

func TestPaidSlotIsNotBookableByAnyoneElse(t *testing.T) {
	owner := newClient(t)
	owner.signUpAndLogIn("owner")

	court := owner.createCourt(owner.createVenue())

	player := newClient(t)
	player.signUpAndLogIn("player")

	slot := freeSlot(t)
	held := player.book(court, slot)
	require.Equal(t, "pending_payment", held.Status)

	status, body := deliverWebhook(t, "payment_intent.succeeded", paymentIntentOf(t, held.ID))
	require.Equal(t, http.StatusNoContent, status, "%s", body)

	require.Equal(t, "confirmed", statusOfBooking(t, player.bookings(), held.ID))

	rival := newClient(t)
	rival.signUpAndLogIn("player")

	taken, gotBody := rival.do(http.MethodPost, "/courts/"+court.ID.String()+"/bookings",
		map[string]string{"starts_at": slot.Format(time.RFC3339)})
	require.Equal(t, http.StatusConflict, taken, "%s", gotBody)

	offered := rival.availabilityOn(court, slot)
	require.False(t, availableAt(t, offered, slot))
}

func statusOfBooking(t *testing.T, bookings []booking, id uuid.UUID) string {
	t.Helper()

	for _, b := range bookings {
		if b.ID == id {
			return b.Status
		}
	}

	require.Failf(t, "booking missing", "no booking %s among the %d listed", id, len(bookings))

	return ""
}

func availableAt(t *testing.T, got availability, at time.Time) bool {
	t.Helper()

	for _, s := range got.Slots {
		if s.StartsAt.Equal(at) {
			return s.Available
		}
	}

	require.Failf(t, "slot missing", "no slot at %s among the %d offered", at, len(got.Slots))

	return false
}
