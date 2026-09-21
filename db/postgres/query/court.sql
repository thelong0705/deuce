-- name: CreateCourt :one
INSERT INTO courts (
    venue_id, name, open_hour, close_hour, price_per_hour
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;
