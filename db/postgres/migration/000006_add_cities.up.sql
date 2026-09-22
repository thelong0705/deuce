CREATE TABLE cities (
    name     citext PRIMARY KEY,
    timezone text   NOT NULL
);

INSERT INTO cities (name, timezone) VALUES
    ('Ha Noi', 'Asia/Ho_Chi_Minh'),
    ('Ho Chi Minh City', 'Asia/Ho_Chi_Minh');

ALTER TABLE venues
    ADD CONSTRAINT venues_city_fkey FOREIGN KEY (city) REFERENCES cities (name);
