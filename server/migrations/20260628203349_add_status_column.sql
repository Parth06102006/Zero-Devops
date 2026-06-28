-- +goose Up
CREATE TYPE github_installation_status AS ENUM ('active', 'suspended', 'uninstalled');

ALTER TABLE github_installations
    ADD COLUMN status github_installation_status NOT NULL DEFAULT 'active';

UPDATE github_installations SET status='active' WHERE status IS NULL;

-- +goose Down
ALTER TABLE github_installations
    DROP COLUMN status;

DROP TYPE github_installation_status;
