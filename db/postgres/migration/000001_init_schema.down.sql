-- Children first: bookings reference courts and users, courts reference venues,
-- venues reference users.
DROP TABLE IF EXISTS bookings;
DROP TABLE IF EXISTS courts;
DROP TABLE IF EXISTS venues;
DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS user_role;
