-- +goose Up
-- Keep existing installation data intact: uniqueness cannot be introduced safely if
-- duplicates already exist, so fail with an actionable error instead of deleting rows.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM github_installations
        GROUP BY installation_id
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'cannot enforce unique github installation_id: duplicate github_installations rows exist';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM github_installations
        GROUP BY user_id
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'cannot enforce one GitHub installation per user: duplicate github_installations rows exist';
    END IF;
END $$;
-- +goose StatementEnd

CREATE UNIQUE INDEX github_installations_installation_id_key
    ON github_installations (installation_id);

CREATE UNIQUE INDEX github_installations_user_id_key
    ON github_installations (user_id);

CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    github_installation_id UUID NOT NULL REFERENCES github_installations(id) ON DELETE CASCADE,
    github_repository_id BIGINT NOT NULL,
    repository_owner TEXT NOT NULL,
    repository_name TEXT NOT NULL,
    repository_full_name TEXT NOT NULL,
    configured_branch TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    desired_revision_generation BIGINT NOT NULL DEFAULT 0 CHECK (desired_revision_generation >= 0),
    build_configuration JSONB NOT NULL DEFAULT '{}'::JSONB,
    configuration_version INTEGER NOT NULL DEFAULT 1 CHECK (configuration_version > 0),
    command_policy_version TEXT NOT NULL DEFAULT '',
    command_scan_result JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT projects_user_repository_key UNIQUE (user_id, github_repository_id)
);

CREATE INDEX idx_projects_github_installation_id
    ON projects (github_installation_id);

-- +goose Down
DROP TABLE IF EXISTS projects;
DROP INDEX IF EXISTS github_installations_user_id_key;
DROP INDEX IF EXISTS github_installations_installation_id_key;
