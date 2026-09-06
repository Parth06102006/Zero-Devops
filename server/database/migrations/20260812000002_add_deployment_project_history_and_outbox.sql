-- +goose Up
CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    delivery_id UUID NOT NULL,
    event_name TEXT NOT NULL,
    event_action TEXT,
    github_installation_external_id BIGINT,
    github_installation_db_id UUID REFERENCES github_installations(id) ON DELETE SET NULL,
    github_repository_id BIGINT,
    processing_status TEXT NOT NULL DEFAULT 'received'
        CHECK (processing_status IN ('received', 'accepted', 'processed', 'failed')),
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    processing_error TEXT,
    payload_reference TEXT,

    CONSTRAINT webhook_deliveries_delivery_id_key UNIQUE (delivery_id)
);

-- All new deployment fields are initially nullable or have defaults so existing
-- deployment history remains readable until application backfill/use-case work lands.
ALTER TABLE deployments
    ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    ADD COLUMN github_installation_id UUID REFERENCES github_installations(id) ON DELETE SET NULL,
    ADD COLUMN commit_sha TEXT,
    ADD COLUMN requested_ref TEXT,
    ADD COLUMN trigger TEXT NOT NULL DEFAULT 'manual'
        CHECK (trigger IN ('manual', 'webhook_push')),
    ADD COLUMN webhook_delivery_id UUID REFERENCES webhook_deliveries(id) ON DELETE SET NULL,
    ADD COLUMN desired_revision_generation BIGINT,
    ADD COLUMN configuration_snapshot JSONB,
    ADD COLUMN configuration_version INTEGER,
    ADD COLUMN command_policy_version TEXT,
    ADD COLUMN command_scan_result JSONB,
    ADD COLUMN worker_id TEXT,
    ADD COLUMN local_image_reference TEXT,
    ADD COLUMN local_image_digest TEXT,
    ADD COLUMN log_url TEXT,
    ADD COLUMN manual_idempotency_key UUID;

CREATE INDEX idx_deployments_project_created_at
    ON deployments (project_id, created_at DESC)
    WHERE project_id IS NOT NULL;

CREATE INDEX idx_deployments_project_status_created_at
    ON deployments (project_id, status, created_at DESC)
    WHERE project_id IS NOT NULL;

CREATE UNIQUE INDEX deployments_webhook_delivery_id_key
    ON deployments (webhook_delivery_id)
    WHERE webhook_delivery_id IS NOT NULL;

CREATE UNIQUE INDEX deployments_project_manual_idempotency_key
    ON deployments (project_id, manual_idempotency_key)
    WHERE project_id IS NOT NULL AND manual_idempotency_key IS NOT NULL;

CREATE TABLE deployment_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id UUID NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    message_version INTEGER NOT NULL DEFAULT 1 CHECK (message_version > 0),
    payload JSONB NOT NULL,
    state TEXT NOT NULL DEFAULT 'pending'
        CHECK (state IN ('pending', 'publishing', 'sent', 'failed')),
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT deployment_outbox_deployment_event_type_key UNIQUE (deployment_id, event_type)
);

CREATE INDEX idx_deployment_outbox_pending
    ON deployment_outbox (available_at, created_at)
    WHERE state IN ('pending', 'failed');

-- +goose Down
DROP TABLE IF EXISTS deployment_outbox;

DROP INDEX IF EXISTS deployments_project_manual_idempotency_key;
DROP INDEX IF EXISTS deployments_webhook_delivery_id_key;
DROP INDEX IF EXISTS idx_deployments_project_status_created_at;
DROP INDEX IF EXISTS idx_deployments_project_created_at;

ALTER TABLE deployments
    DROP COLUMN IF EXISTS manual_idempotency_key,
    DROP COLUMN IF EXISTS log_url,
    DROP COLUMN IF EXISTS local_image_digest,
    DROP COLUMN IF EXISTS local_image_reference,
    DROP COLUMN IF EXISTS worker_id,
    DROP COLUMN IF EXISTS command_scan_result,
    DROP COLUMN IF EXISTS command_policy_version,
    DROP COLUMN IF EXISTS configuration_version,
    DROP COLUMN IF EXISTS configuration_snapshot,
    DROP COLUMN IF EXISTS desired_revision_generation,
    DROP COLUMN IF EXISTS webhook_delivery_id,
    DROP COLUMN IF EXISTS trigger,
    DROP COLUMN IF EXISTS requested_ref,
    DROP COLUMN IF EXISTS commit_sha,
    DROP COLUMN IF EXISTS github_installation_id,
    DROP COLUMN IF EXISTS project_id;

DROP TABLE IF EXISTS webhook_deliveries;
