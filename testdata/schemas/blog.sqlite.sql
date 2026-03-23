-- Blog schema — SQLite
-- Uses: TEXT for UUIDs (SQLite has no UUID type), DATETIME

DROP TABLE IF EXISTS post_tags;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS posts;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS users;

CREATE TABLE users (
                       id         TEXT     NOT NULL PRIMARY KEY DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6)))),
                       email      TEXT     NOT NULL UNIQUE,
                       name       TEXT     NOT NULL,
                       bio        TEXT,
                       role       TEXT     NOT NULL DEFAULT 'member',
                       created_at DATETIME NOT NULL DEFAULT (datetime('now')),
                       updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE posts (
                       id           TEXT     NOT NULL PRIMARY KEY DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6)))),
                       user_id      TEXT     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                       title        TEXT     NOT NULL,
                       body         TEXT     NOT NULL,
                       status       TEXT     NOT NULL DEFAULT 'draft',
                       published_at DATETIME,
                       created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
                       updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE comments (
                          id         TEXT     NOT NULL PRIMARY KEY DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6)))),
                          post_id    TEXT     NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
                          user_id    TEXT     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                          body       TEXT     NOT NULL,
                          created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE tags (
                      id   TEXT NOT NULL PRIMARY KEY DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6)))),
                      name TEXT NOT NULL UNIQUE
);

-- Composite PK — kiln skips this table in v1
CREATE TABLE post_tags (
                           post_id TEXT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
                           tag_id  TEXT NOT NULL REFERENCES tags(id)  ON DELETE CASCADE,
                           PRIMARY KEY (post_id, tag_id)
);

-- SQLite doesn't fire updated_at automatically — use a trigger instead
CREATE TRIGGER users_updated_at
    AFTER UPDATE ON users
    FOR EACH ROW
BEGIN
    UPDATE users SET updated_at = datetime('now') WHERE id = OLD.id;
END;

CREATE TRIGGER posts_updated_at
    AFTER UPDATE ON posts
    FOR EACH ROW
BEGIN
    UPDATE posts SET updated_at = datetime('now') WHERE id = OLD.id;
END;