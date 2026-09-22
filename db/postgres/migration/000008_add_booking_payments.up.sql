CREATE TYPE booking_status AS ENUM ('pending_payment', 'confirmed');

ALTER TABLE bookings
    ADD COLUMN status            booking_status NOT NULL,
    ADD COLUMN amount            integer,
    ADD COLUMN hold_expires_at   timestamptz,
    ADD COLUMN payment_intent_id text;

CREATE UNIQUE INDEX bookings_payment_intent_key
    ON bookings (payment_intent_id)
    WHERE payment_intent_id IS NOT NULL;

CREATE INDEX bookings_expiring_holds_idx
    ON bookings (hold_expires_at)
    WHERE status = 'pending_payment' AND cancelled_at IS NULL;

CREATE TABLE stripe_events (
    id          text        PRIMARY KEY,
    type        text        NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now()
);
