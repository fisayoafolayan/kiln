-- Blog schema — Postgres
-- Uses: native UUID, TIMESTAMPTZ, TEXT, gen_random_uuid()

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

DROP TABLE IF EXISTS post_tags CASCADE;
DROP TABLE IF EXISTS comments CASCADE;
DROP TABLE IF EXISTS posts CASCADE;
DROP TABLE IF EXISTS tags CASCADE;
DROP TABLE IF EXISTS users CASCADE;

CREATE TABLE users (
                       id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
                       email       TEXT        NOT NULL UNIQUE,
                       name        TEXT        NOT NULL,
                       bio         TEXT,
                       role        TEXT        NOT NULL DEFAULT 'member',
                       created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
                       updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE posts (
                       id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
                       user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                       title        TEXT        NOT NULL,
                       body         TEXT        NOT NULL,
                       status       TEXT        NOT NULL DEFAULT 'draft',
                       published_at TIMESTAMPTZ,
                       created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
                       updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE comments (
                          id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
                          post_id    UUID        NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
                          user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                          body       TEXT        NOT NULL,
                          created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE tags (
                      id   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                      name TEXT NOT NULL UNIQUE
);

-- Composite PK — kiln skips this table in v1
CREATE TABLE post_tags (
                           post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
                           tag_id  UUID NOT NULL REFERENCES tags(id)  ON DELETE CASCADE,
                           PRIMARY KEY (post_id, tag_id)
);