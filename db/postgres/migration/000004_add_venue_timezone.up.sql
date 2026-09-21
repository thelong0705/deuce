ALTER TABLE venues
    ADD COLUMN timezone text NOT NULL DEFAULT 'UTC';
