-- +goose Up
ALTER TABLE projects
    ADD COLUMN name TEXT;

-- Backfill existing projects with their repository_name
UPDATE projects
SET name = repository_name
WHERE name IS NULL;

-- In case duplicate (user_id, name) existed, suffix duplicates with first 4 chars of id
UPDATE projects p
SET name = p.name || '-' || SUBSTRING(p.id::TEXT, 1, 4)
WHERE p.id IN (
    SELECT id FROM (
        SELECT id, ROW_NUMBER() OVER (PARTITION BY user_id, name ORDER BY created_at) as rn
        FROM projects
    ) sub
    WHERE sub.rn > 1
);

ALTER TABLE projects
    ALTER COLUMN name SET NOT NULL;

-- Drop the 1-to-1 constraint that restricted one project per repository per user
ALTER TABLE projects
    DROP CONSTRAINT IF EXISTS projects_user_repository_key;

-- Enforce unique project name per user
ALTER TABLE projects
    ADD CONSTRAINT projects_user_name_key UNIQUE (user_id, name);

-- Index for fast webhook multi-project lookups by repo ID under an installation
CREATE INDEX IF NOT EXISTS idx_projects_installation_repo
    ON projects (github_installation_id, github_repository_id);

-- Update deployments_webhook_delivery_id_key so multiple projects can be built from the same webhook delivery
DROP INDEX IF EXISTS deployments_webhook_delivery_id_key;

CREATE UNIQUE INDEX deployments_webhook_delivery_id_key
    ON deployments (project_id, webhook_delivery_id)
    WHERE project_id IS NOT NULL AND webhook_delivery_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS deployments_webhook_delivery_id_key;

CREATE UNIQUE INDEX deployments_webhook_delivery_id_key
    ON deployments (webhook_delivery_id)
    WHERE webhook_delivery_id IS NOT NULL;

DROP INDEX IF EXISTS idx_projects_installation_repo;

ALTER TABLE projects
    DROP CONSTRAINT IF EXISTS projects_user_name_key;

ALTER TABLE projects
    ADD CONSTRAINT projects_user_repository_key UNIQUE (user_id, github_repository_id);

ALTER TABLE projects
    DROP COLUMN IF EXISTS name;
