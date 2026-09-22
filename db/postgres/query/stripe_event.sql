-- name: RecordStripeEvent :one
INSERT INTO stripe_events (id, type)
VALUES ($1, $2)
ON CONFLICT (id) DO NOTHING
RETURNING id;
