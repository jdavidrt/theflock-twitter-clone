-- Schema for the SQLite store (D-64 table shapes, carried over from the archived PostgreSQL
-- design in my-docs/archive/postgres/). Applied at boot with CREATE TABLE IF NOT EXISTS; there
-- is one schema and no migration history to manage (D-66). snake_case tables, TEXT UUID ids,
-- ISO 8601 TEXT timestamps (application enforces lengths and other business rules — never a
-- database CHECK, except the self-follow guard below which also has a Go-level check in
-- Store.Follow so the error is the same store.ErrSelfFollow sentinel the memory store returns).

CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    email         TEXT NOT NULL UNIQUE,
    display_name  TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    bio           TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS follows (
    follower_id TEXT NOT NULL REFERENCES users (id),
    followee_id TEXT NOT NULL REFERENCES users (id),
    created_at  TEXT NOT NULL,
    PRIMARY KEY (follower_id, followee_id),
    CHECK (follower_id <> followee_id)
);

CREATE INDEX IF NOT EXISTS idx_follows_followee_created ON follows (followee_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_follows_follower_created ON follows (follower_id, created_at DESC);

-- ON DELETE NO ACTION (not RESTRICT): SQLite checks RESTRICT per row instead of at statement
-- end, which breaks a bulk DELETE over a reply chain; NO ACTION gives the same "a parent with
-- replies cannot be hard-deleted" guarantee with statement-end checking (D-64). Nothing in this
-- codebase hard-deletes a tweet — deletion is always the soft delete below — so this is a
-- safety rail, not a path anything exercises today.
CREATE TABLE IF NOT EXISTS tweets (
    id              TEXT PRIMARY KEY,
    author_id       TEXT NOT NULL REFERENCES users (id),
    content         TEXT NOT NULL,
    parent_tweet_id TEXT REFERENCES tweets (id) ON DELETE NO ACTION,
    created_at      TEXT NOT NULL,
    deleted_at      TEXT
);

CREATE INDEX IF NOT EXISTS idx_tweets_author_created ON tweets (author_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_tweets_parent_created ON tweets (parent_tweet_id, created_at);

CREATE TABLE IF NOT EXISTS likes (
    user_id    TEXT NOT NULL REFERENCES users (id),
    tweet_id   TEXT NOT NULL REFERENCES tweets (id),
    created_at TEXT NOT NULL,
    PRIMARY KEY (user_id, tweet_id)
);

CREATE INDEX IF NOT EXISTS idx_likes_tweet ON likes (tweet_id);
