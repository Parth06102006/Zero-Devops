-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id BIGINT NOT NULL,
    provider TEXT NOT NULL,
    username TEXT NOT NULL,
    email TEXT,
    avatar_url TEXT,
    created_at TIMESTAMP NOT NULL,
    refresh_token TEXT
);

-- +goose Down
DROP TABLE IF EXISTS users;
