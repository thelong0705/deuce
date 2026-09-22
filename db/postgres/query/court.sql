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
