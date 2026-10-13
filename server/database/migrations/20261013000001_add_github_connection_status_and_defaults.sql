-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'github_connection_status') THEN
        CREATE TYPE github_connection_status AS ENUM ('connected', 'disconnected');
    END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE github_installations
    ADD COLUMN IF NOT EXISTS connection_status github_connection_status NOT NULL DEFAULT 'connected';

ALTER TABLE github_installations
    ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP,
    ALTER COLUMN updated_at SET DEFAULT CURRENT_TIMESTAMP;

-- Drop unique index on user_id so users can have multiple saved installations
DROP INDEX IF EXISTS github_installations_user_id_key;

-- Enforce at most one connected installation per ShipZero user at any time
CREATE UNIQUE INDEX IF NOT EXISTS github_installations_one_connected_per_user
    ON github_installations (user_id)
    WHERE connection_status = 'connected';

-- +goose Down
DROP INDEX IF EXISTS github_installations_one_connected_per_user;

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM github_installations GROUP BY user_id HAVING COUNT(*) > 1
    ) THEN
        CREATE UNIQUE INDEX IF NOT EXISTS github_installations_user_id_key ON github_installations (user_id);
    END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE github_installations
    ALTER COLUMN created_at DROP DEFAULT,
    ALTER COLUMN updated_at DROP DEFAULT,
    DROP COLUMN IF EXISTS connection_status;

DROP TYPE IF EXISTS github_connection_status;
