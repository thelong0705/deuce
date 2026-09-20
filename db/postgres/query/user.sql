-- name: CreateUser :one
INSERT INTO users (
    email, password_hash, display_name, role
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: GetUser :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 LIMIT 1;

-- name: UpdateUserProfile :one
UPDATE users
SET display_name = $2
WHERE id = $1
RETURNING *;
