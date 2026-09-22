DROP TABLE IF EXISTS stripe_events;

DROP INDEX IF EXISTS bookings_expiring_holds_idx;
DROP INDEX IF EXISTS bookings_payment_intent_key;

ALTER TABLE bookings
    DROP COLUMN IF EXISTS payment_intent_id,
    DROP COLUMN IF EXISTS hold_expires_at,
    DROP COLUMN IF EXISTS amount,
    DROP COLUMN IF EXISTS status;

DROP TYPE IF EXISTS booking_status;
