-- name: CreateVenue :one
-- Selecting from cities takes the timezone from it and writes no row at all
-- for a city that is not in the table.
INSERT INTO venues (
    owner_id, name, city, address, timezone
)
SELECT sqlc.arg(owner_id), sqlc.arg(name), cities.name, sqlc.arg(address), cities.timezone
FROM cities
WHERE cities.name = sqlc.arg(city)
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

-- name: SearchVenuesByCity :many
-- city is citext, so this matches regardless of how the caller capitalised it.
SELECT * FROM venues
WHERE is_active AND city = $1
ORDER BY name;
