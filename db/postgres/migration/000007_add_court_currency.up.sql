-- An enum rather than free text, so a price cannot be stored in a currency
-- nothing knows how to render. Adding one later is ALTER TYPE ... ADD VALUE.
CREATE TYPE "currency" AS ENUM ('VND');

ALTER TABLE courts
    ADD COLUMN currency currency NOT NULL DEFAULT 'VND';
