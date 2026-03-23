-- Blog schema - MySQL 8
-- Uses: CHAR(36) for UUIDs, DATETIME, VARCHAR, UUID() function
-- MySQL has no native UUID type — stored as CHAR(36)

SET FOREIGN_KEY_CHECKS = 0;
DROP TABLE IF EXISTS post_tags;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS posts;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS users;
SET FOREIGN_KEY_CHECKS = 1;

CREATE TABLE users (
                       id         CHAR(36)     NOT NULL PRIMARY KEY DEFAULT (UUID()),
                       email      VARCHAR(255) NOT NULL UNIQUE,
                       name       VARCHAR(255) NOT NULL,
                       bio        TEXT,
                       role       VARCHAR(50)  NOT NULL DEFAULT 'member',
                       created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
                       updated_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE posts (
                       id           CHAR(36)     NOT NULL PRIMARY KEY DEFAULT (UUID()),
                       user_id      CHAR(36)     NOT NULL,
                       title        VARCHAR(500) NOT NULL,
                       body         TEXT         NOT NULL,
                       status       VARCHAR(50)  NOT NULL DEFAULT 'draft',
                       published_at DATETIME,
                       created_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
                       updated_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
                       CONSTRAINT fk_posts_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE comments (
                          id         CHAR(36)  NOT NULL PRIMARY KEY DEFAULT (UUID()),
                          post_id    CHAR(36)  NOT NULL,
                          user_id    CHAR(36)  NOT NULL,
                          body       TEXT      NOT NULL,
                          created_at DATETIME  NOT NULL DEFAULT CURRENT_TIMESTAMP,
                          CONSTRAINT fk_comments_post FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
                          CONSTRAINT fk_comments_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE tags (
                      id   CHAR(36)     NOT NULL PRIMARY KEY DEFAULT (UUID()),
                      name VARCHAR(100) NOT NULL UNIQUE
);

-- Composite PK — kiln skips this table in v1
CREATE TABLE post_tags (
                           post_id CHAR(36) NOT NULL,
                           tag_id  CHAR(36) NOT NULL,
                           PRIMARY KEY (post_id, tag_id),
                           CONSTRAINT fk_post_tags_post FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
                           CONSTRAINT fk_post_tags_tag  FOREIGN KEY (tag_id)  REFERENCES tags(id)  ON DELETE CASCADE
);