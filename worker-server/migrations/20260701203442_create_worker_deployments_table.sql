-- +goose Up
CREATE TYPE deployment_status AS ENUM (
    'queued',
    'building',
    'done',
    'failed'
);

CREATE TABLE deployments (
    id              BIGINT PRIMARY KEY,
    clone_url       TEXT NOT NULL,
    status          deployment_status NOT NULL DEFAULT 'queued',

    retry_count     INTEGER NOT NULL DEFAULT 0,
    error_message   TEXT,

    image_tag       TEXT,               -- e.g. "deploy-<id>:latest"
    output_url      TEXT,               -- CloudStorage reference once build completes

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at      TIMESTAMPTZ,        -- set when worker picks it up (status -> building)
    finished_at     TIMESTAMPTZ         -- set on done/failed
);

CREATE INDEX idx_deployments_status ON deployments(status);
CREATE INDEX idx_deployments_created_at ON deployments(created_at);

-- +goose Down

DROP TABLE IF EXISTS deployments;
DROP TYPE IF EXISTS deployment_status;
