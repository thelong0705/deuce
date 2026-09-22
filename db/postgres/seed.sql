-- Sample data for poking at a local deuce: two owners with venues and courts,
-- two players, and a few bookings.
--
-- Every account's password is "supersecret". The hash below is a real bcrypt
-- of it, so signing in through the app works.
--
--     make seed
--
-- Re-running replaces what this file inserted and leaves anything else alone.

BEGIN;

-- Only the seeded accounts, so a database with hand-made data keeps it.
-- Bookings, courts and venues go first because they point at the users.
DELETE FROM bookings WHERE player_id IN (
    SELECT id FROM users WHERE email LIKE '%@deuce.test'
);
DELETE FROM bookings WHERE court_id IN (
    SELECT c.id FROM courts c
    JOIN venues v ON v.id = c.venue_id
    JOIN users u ON u.id = v.owner_id
    WHERE u.email LIKE '%@deuce.test'
);
DELETE FROM courts WHERE venue_id IN (
    SELECT v.id FROM venues v
    JOIN users u ON u.id = v.owner_id
    WHERE u.email LIKE '%@deuce.test'
);
DELETE FROM venues WHERE owner_id IN (
    SELECT id FROM users WHERE email LIKE '%@deuce.test'
);
DELETE FROM users WHERE email LIKE '%@deuce.test';

WITH seeded_users AS (
    INSERT INTO users (email, password_hash, display_name, phone_number, role)
    VALUES
        ('owner1@deuce.test',  '$2a$10$Y6RMqWU7hoxncfRSPBQx7OFkw6cyN1LtIYzlR8ZO3UHWvR7DEVg5W', 'Mai',  '+84900000101', 'owner'),
        ('owner2@deuce.test',  '$2a$10$Y6RMqWU7hoxncfRSPBQx7OFkw6cyN1LtIYzlR8ZO3UHWvR7DEVg5W', 'Hung', '+84900000102', 'owner'),
        ('player1@deuce.test', '$2a$10$Y6RMqWU7hoxncfRSPBQx7OFkw6cyN1LtIYzlR8ZO3UHWvR7DEVg5W', 'Linh', '+84900000201', 'player'),
        ('player2@deuce.test', '$2a$10$Y6RMqWU7hoxncfRSPBQx7OFkw6cyN1LtIYzlR8ZO3UHWvR7DEVg5W', 'Nam',  '+84900000202', 'player')
    RETURNING id, email
),
-- The timezone comes from the city, as CreateVenue does it, so a city that is
-- not in the table seeds no venue rather than a wrong one.
seeded_venues AS (
    INSERT INTO venues (owner_id, name, city, address, timezone)
    SELECT u.id, v.name, c.name, v.address, c.timezone
    FROM (VALUES
        ('owner1@deuce.test', 'Riverside Tennis Club', 'Ha Noi',           '12 Bach Dang'),
        ('owner1@deuce.test', 'Lakeside Courts',       'Ha Noi',           '88 Tay Ho'),
        ('owner2@deuce.test', 'Saigon Smash Club',     'Ho Chi Minh City', '5 Nguyen Hue')
    ) AS v(owner_email, name, city, address)
    JOIN seeded_users u ON u.email = v.owner_email
    JOIN cities c ON c.name = v.city
    RETURNING id, name
)
-- Currency is spelled out rather than left to the column default, so this file
-- says what the prices are in.
INSERT INTO courts (venue_id, name, open_hour, close_hour, price_per_hour, currency)
-- The cast is needed because a VALUES literal is text, and currency is an enum.
SELECT v.id, c.name, c.open_hour, c.close_hour, c.price_per_hour, c.currency::currency
FROM (VALUES
    -- Slots are two hours wide and tile from open_hour, so 6-22 gives
    -- 06:00-08:00 through 20:00-22:00.
    ('Riverside Tennis Club', 'Court 1',      6, 22, 150000, 'VND'),
    ('Riverside Tennis Club', 'Court 2',      6, 22, 150000, 'VND'),
    ('Riverside Tennis Club', 'Centre Court', 8, 20, 250000, 'VND'),
    -- An odd span: 6-11 gives 06:00-08:00 and 08:00-10:00, and the last hour
    -- goes unused. Handy for seeing that rule in the UI.
    ('Lakeside Courts',       'Court A',      6, 11, 120000, 'VND'),
    ('Lakeside Courts',       'Court B',     14, 22, 120000, 'VND'),
    ('Saigon Smash Club',     'Clay 1',       6, 24, 200000, 'VND'),
    ('Saigon Smash Club',     'Clay 2',       6, 24, 200000, 'VND')
) AS c(venue_name, name, open_hour, close_hour, price_per_hour, currency)
JOIN seeded_venues v ON v.name = c.venue_name;

-- A couple of bookings tomorrow, so "My bookings" is not empty and some slots
-- show as taken. Dates are relative so the seed does not go stale.
INSERT INTO bookings (court_id, player_id, starts_at)
SELECT c.id, u.id,
       ((CURRENT_DATE + 1) + b.at) AT TIME ZONE v.timezone
FROM (VALUES
    ('Riverside Tennis Club', 'Court 1', 'player1@deuce.test', TIME '08:00'),
    ('Riverside Tennis Club', 'Court 1', 'player2@deuce.test', TIME '10:00'),
    ('Saigon Smash Club',     'Clay 1',  'player1@deuce.test', TIME '18:00')
) AS b(venue_name, court_name, player_email, at)
JOIN venues v ON v.name = b.venue_name
JOIN courts c ON c.venue_id = v.id AND c.name = b.court_name
JOIN users u ON u.email = b.player_email;

COMMIT;

\echo ''
\echo 'Seeded. Every account signs in with the password: supersecret'
\echo '  owner1@deuce.test   Riverside Tennis Club, Lakeside Courts'
\echo '  owner2@deuce.test   Saigon Smash Club'
\echo '  player1@deuce.test  two bookings tomorrow'
\echo '  player2@deuce.test  one booking tomorrow'
