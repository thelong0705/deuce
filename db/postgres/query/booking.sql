-- name: HoldSlot :one
INSERT INTO bookings (
    court_id, player_id, starts_at, status, amount, hold_expires_at
) VALUES (
    $1, $2, $3, 'pending_payment', $4, $5
)
RETURNING *;

-- name: AttachPayment :exec
UPDATE bookings
SET payment_intent_id = $2
WHERE id = $1;

-- name: ConfirmBooking :exec
UPDATE bookings
SET status = 'confirmed', hold_expires_at = NULL
WHERE id = $1;

-- name: GetBookingByPayment :one
SELECT * FROM bookings
WHERE payment_intent_id = $1;

-- name: CancelBooking :exec
UPDATE bookings
SET cancelled_at = now()
WHERE id = $1 AND cancelled_at IS NULL;

-- name: GetCourtVenue :one
SELECT sqlc.embed(courts), sqlc.embed(venues)
FROM courts
JOIN venues ON venues.id = courts.venue_id
WHERE courts.id = $1;

-- name: ListBookedSlots :many
SELECT starts_at FROM bookings
WHERE court_id = $1
  AND cancelled_at IS NULL
  AND starts_at >= $2
  AND starts_at < $3
ORDER BY starts_at;

-- name: ListPlayerBookings :many
SELECT sqlc.embed(bookings), sqlc.embed(courts), sqlc.embed(venues)
FROM bookings
JOIN courts ON courts.id = bookings.court_id
JOIN venues ON venues.id = courts.venue_id
WHERE bookings.player_id = $1
  AND bookings.cancelled_at IS NULL
  AND bookings.starts_at >= $2
ORDER BY bookings.starts_at;

-- name: ListBookedSlotsForCourts :many
SELECT court_id, starts_at FROM bookings
WHERE court_id = ANY(@court_ids::uuid[])
  AND cancelled_at IS NULL
  AND starts_at >= @from_time
  AND starts_at < @to_time
ORDER BY court_id, starts_at;

-- name: ReleaseLapsedHolds :execrows
UPDATE bookings
SET cancelled_at = now()
WHERE status = 'pending_payment'
  AND cancelled_at IS NULL
  AND hold_expires_at <= $1;
