-- +goose Up
CREATE TYPE deployment_status AS ENUM (
    'pending',
    'building',
    'success',
    'failed',
    'canceled'
);

CREATE TABLE deployments (
    id              UUID PRIMARY KEY,
    clone_url       TEXT NOT NULL,
    status          deployment_status NOT NULL DEFAULT 'pending',

    retry_count     INTEGER NOT NULL DEFAULT 0,
    error_message   TEXT,

    image_tag       TEXT,
    output_url      TEXT,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ
);

CREATE INDEX idx_deployments_status ON deployments(status);
CREATE INDEX idx_deployments_created_at ON deployments(created_at);

-- +goose Down

DROP TABLE IF EXISTS deployments;
DROP TYPE IF EXISTS deployment_status;
