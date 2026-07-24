-- +goose Up
ALTER TABLE deployments ADD COLUMN build_type TEXT NOT NULL DEFAULT 'buildpacks'
    CHECK (build_type IN ('docker', 'buildpacks'));

-- +goose Down
ALTER TABLE deployments DROP COLUMN IF EXISTS build_type;
