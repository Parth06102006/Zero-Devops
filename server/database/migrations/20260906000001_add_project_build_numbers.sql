-- +goose Up
-- Per-project build numbers (plan-server-12-08.md "Immediate Follow-Ups",
-- 2026-09-06). deployments.build_number is assigned inside the
-- StoreProjectBuildWithOutbox / StoreWebhookBuildWithOutbox transactions from
-- projects.next_build_number, so manual and webhook builds share one sequence
-- and a failed/rolled-back build never burns a number.

ALTER TABLE deployments
    ADD COLUMN build_number BIGINT;

ALTER TABLE projects
    ADD COLUMN next_build_number BIGINT NOT NULL DEFAULT 1;

-- Backfill: number existing project-scoped builds by creation order. Legacy
-- rows with NULL project_id keep NULL (no project, no sequence).
WITH numbered AS (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY project_id ORDER BY created_at, id) AS rn
    FROM deployments
    WHERE project_id IS NOT NULL
)
UPDATE deployments d
SET build_number = numbered.rn
FROM numbered
WHERE d.id = numbered.id;

-- Seed each project's counter one past its highest backfilled number.
-- Projects with no builds stay at the default 1.
WITH highest AS (
    SELECT project_id, MAX(build_number) AS max_number
    FROM deployments
    WHERE project_id IS NOT NULL
    GROUP BY project_id
)
UPDATE projects p
SET next_build_number = COALESCE(highest.max_number, 0) + 1
FROM highest
WHERE p.id = highest.project_id;

-- Safety net: the in-transaction counter UPDATE is already race-free (row
-- lock held until commit), but this partial unique index also catches any
-- future writer that assigns numbers outside the transaction.
CREATE UNIQUE INDEX deployments_project_build_number_key
    ON deployments (project_id, build_number)
    WHERE build_number IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS deployments_project_build_number_key;
ALTER TABLE projects DROP COLUMN IF EXISTS next_build_number;
ALTER TABLE deployments DROP COLUMN IF EXISTS build_number;
