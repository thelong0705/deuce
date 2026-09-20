CREATE EXTENSION IF NOT EXISTS citext;

CREATE TYPE "user_role" AS ENUM (
  'player',
  'owner'
);

CREATE TABLE "users" (
                         "id" uuid PRIMARY KEY DEFAULT (gen_random_uuid()),
                         "email" citext UNIQUE NOT NULL,
                         "password_hash" varchar NOT NULL,
                         "display_name" varchar NOT NULL,
                         "phone_number" varchar(16),
                         "role" user_role NOT NULL,
                         "is_active" boolean NOT NULL DEFAULT (true),
                         "created_at" timestamptz NOT NULL DEFAULT (now()),
                         "updated_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "venues" (
                          "id" uuid PRIMARY KEY DEFAULT (gen_random_uuid()),
                          "owner_id" uuid NOT NULL,
                          "name" varchar NOT NULL,
                          "city" citext NOT NULL,
                          "address" varchar NOT NULL,
                          "is_active" boolean NOT NULL DEFAULT (true),
                          "created_at" timestamptz NOT NULL DEFAULT (now()),
                          "updated_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "courts" (
                          "id" uuid PRIMARY KEY DEFAULT (gen_random_uuid()),
                          "venue_id" uuid NOT NULL,
                          "name" varchar NOT NULL,
                          "open_hour" smallint NOT NULL DEFAULT 6,
                          "close_hour" smallint NOT NULL DEFAULT 22,
                          "price_per_hour" integer NOT NULL,
                          "is_active" boolean NOT NULL DEFAULT (true),
                          "created_at" timestamptz NOT NULL DEFAULT (now()),
                          "updated_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "bookings" (
                            "id" uuid PRIMARY KEY DEFAULT (gen_random_uuid()),
                            "court_id" uuid NOT NULL,
                            "player_id" uuid,
                            "is_block" boolean NOT NULL DEFAULT (false),
                            "starts_at" timestamptz NOT NULL,
                            "cancelled_at" timestamptz,
                            "created_at" timestamptz NOT NULL DEFAULT (now()),
                            "updated_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE UNIQUE INDEX ON "users" ("id", "role");

CREATE INDEX ON "venues" ("owner_id");

CREATE UNIQUE INDEX ON "courts" ("venue_id", "name");

CREATE UNIQUE INDEX bookings_active_slot_unique
    ON bookings (court_id, starts_at)
    WHERE cancelled_at IS NULL;

CREATE INDEX ON "bookings" ("player_id", "starts_at");

ALTER TABLE "courts" ADD FOREIGN KEY ("venue_id") REFERENCES "venues" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "bookings" ADD FOREIGN KEY ("court_id") REFERENCES "courts" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "bookings" ADD FOREIGN KEY ("player_id") REFERENCES "users" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE venues ADD FOREIGN KEY (owner_id) REFERENCES users (id);