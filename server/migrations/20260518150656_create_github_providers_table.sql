-- +goose Up
CREATE TABLE IF NOT EXISTS github_installations(
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    installation_id BIGINT NOT NULL,
    account_name TEXT NOT NULL,

    CONSTRAINT github_installations_user_id_fkey

    FOREIGN KEY(user_id)
    REFERENCES users(id)
    ON DELETE CASCADE
);

-- +goose Down
DROP TABLE IF EXISTS github_installations;
