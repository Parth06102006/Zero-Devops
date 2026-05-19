-- +goose Up
ALTER TABLE github_installations DROP COLUMN account_name;
ALTER TABLE github_installations
    ADD COLUMN account_type TEXT NOT NULL,
    ADD COLUMN account_login TEXT NOT NULL,
    ADD COLUMN created_at TIMESTAMP NOT NULL,
    ADD COLUMN updated_at TIMESTAMP NOT NULL
;
-- ALTER TABLE github_installations ADD COLUMN ;
-- ALTER TABLE github_installations ADD COLUMN account_login TEXT NOT NULL;

-- +goose Down
ALTER TABLE github_installations
    ADD COLUMN account_name TEXT;

UPDATE github_installations
SET account_name = account_login;

ALTER TABLE github_installations
    ALTER COLUMN account_name SET NOT NULL;

ALTER TABLE github_installations 
    DROP COLUMN account_type,
    DROP COLUMN account_login,
    DROP COLUMN created_at,
    DROP COLUMN updated_at;