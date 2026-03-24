-- Seed data for the blog API example

INSERT INTO users (email, name, role) VALUES
    ('alice@example.com', 'Alice Johnson', 'admin'),
    ('bob@example.com', 'Bob Smith', 'moderator'),
    ('carol@example.com', 'Carol Williams', 'member');

INSERT INTO posts (user_id, title, body, status) VALUES
    ((SELECT id FROM users WHERE email = 'alice@example.com'), 'Getting Started with Go', 'Go is a statically typed language...', 'published'),
    ((SELECT id FROM users WHERE email = 'alice@example.com'), 'Advanced Concurrency Patterns', 'Goroutines and channels are powerful...', 'published'),
    ((SELECT id FROM users WHERE email = 'bob@example.com'), 'Database Design Tips', 'Normalize your schema, but know when to denormalize...', 'draft'),
    ((SELECT id FROM users WHERE email = 'carol@example.com'), 'My First Post', 'Hello world!', 'draft');

INSERT INTO comments (post_id, user_id, body) VALUES
    ((SELECT id FROM posts WHERE title = 'Getting Started with Go'), (SELECT id FROM users WHERE email = 'bob@example.com'), 'Great introduction!'),
    ((SELECT id FROM posts WHERE title = 'Getting Started with Go'), (SELECT id FROM users WHERE email = 'carol@example.com'), 'Very helpful, thanks.'),
    ((SELECT id FROM posts WHERE title = 'Advanced Concurrency Patterns'), (SELECT id FROM users WHERE email = 'carol@example.com'), 'I learned a lot from this.');

INSERT INTO tags (name) VALUES
    ('go'),
    ('databases'),
    ('tutorial'),
    ('concurrency');
