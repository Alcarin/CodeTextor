-- TODO: add indexes for performance
-- Schema for user management

CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  email TEXT UNIQUE NOT NULL
);

CREATE TABLE posts (
  id SERIAL PRIMARY KEY,
  user_id INTEGER REFERENCES users(id),
  title TEXT NOT NULL,
  body TEXT,
  created_at TIMESTAMP DEFAULT NOW()
);

INSERT INTO users (name, email) VALUES ('alice', 'alice@example.com');
INSERT INTO users (name, email) VALUES ('bob', 'bob@example.com');

-- FIXME: optimize this query
SELECT id, name, email FROM users;

SELECT p.title, u.name
FROM posts p
JOIN users u ON p.user_id = u.id;

DROP TABLE posts;
DROP TABLE users;
