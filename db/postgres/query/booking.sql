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
