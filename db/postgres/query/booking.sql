-- name: CreateBooking :one
INSERT INTO bookings (
    court_id, player_id, starts_at
) VALUES (
    $1, $2, $3
)
RETURNING *;

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
