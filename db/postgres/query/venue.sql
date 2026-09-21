-- name: CreateVenue :one
INSERT INTO venues (
    owner_id, name, city, address, timezone
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: GetVenue :one
SELECT * FROM venues
WHERE id = $1 LIMIT 1;

-- name: ListVenues :many
SELECT * FROM venues
WHERE is_active
ORDER BY name;

-- name: UpdateVenue :one
UPDATE venues
SET name    = $2,
    city    = $3,
    address = $4
WHERE id = $1
RETURNING *;

-- name: DeactivateVenue :one
UPDATE venues
SET is_active = false
WHERE id = $1
RETURNING *;

-- name: ListVenuesByOwner :many
SELECT * FROM venues
WHERE owner_id = $1
ORDER BY name;
