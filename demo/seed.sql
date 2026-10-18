-- Fake demo data. Nothing here is real.
CREATE TABLE users (
    id    serial PRIMARY KEY,
    name  text NOT NULL,
    email text NOT NULL
);
INSERT INTO users (name, email) VALUES
    ('Ada Example',   'ada@example.test'),
    ('Linus Example', 'linus@example.test'),
    ('Grace Example', 'grace@example.test'),
    ('Alan Example',  'alan@example.test'),
    ('Test User 1',   'junk1@example.test'),
    ('Test User 2',   'junk2@example.test');

CREATE TABLE orders (
    id      serial PRIMARY KEY,
    user_id integer REFERENCES users(id),
    item    text NOT NULL
);
INSERT INTO orders (user_id, item) VALUES (1, 'keyboard'), (2, 'monitor'), (3, 'coffee');
