-- +goose Up
CREATE TYPE github_installation_status AS ENUM ('active', 'suspended', 'uninstalled');

CREATE TABLE IF NOT EXISTS github_installations(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    installation_id BIGINT NOT NULL,
    account_type TEXT NOT NULL,
    account_login TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    status github_installation_status NOT NULL DEFAULT 'active',

    CONSTRAINT github_installations_user_id_fkey
    FOREIGN KEY(user_id)
    REFERENCES users(id)
    ON DELETE CASCADE
);

-- +goose Down
DROP TABLE IF EXISTS github_installations;
DROP TYPE IF EXISTS github_installation_status;
