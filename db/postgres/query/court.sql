-- name: CreateCourt :one
INSERT INTO courts (
    venue_id, name, open_hour, close_hour, price_per_hour
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: ListCourtsByVenue :many
SELECT * FROM courts
WHERE venue_id = $1 AND is_active
ORDER BY name;

-- name: SearchCourts :many
SELECT sqlc.embed(courts), sqlc.embed(venues)
FROM courts
JOIN venues ON venues.id = courts.venue_id
WHERE venues.city = $1
  AND venues.is_active
  AND courts.is_active
ORDER BY venues.name, courts.name;
