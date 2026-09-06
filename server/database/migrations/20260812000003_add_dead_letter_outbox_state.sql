-- +goose Up

-- Events that cannot be published after the retry limit are retained for
-- inspection and manual replay instead of being retried forever.
ALTER TABLE deployment_outbox
    DROP CONSTRAINT IF EXISTS deployment_outbox_state_check;

ALTER TABLE deployment_outbox
    ADD CONSTRAINT deployment_outbox_state_check
    CHECK (state IN ('pending', 'publishing', 'sent', 'failed', 'dead_letter'));

-- +goose Down

-- Do not silently discard dead-letter records during rollback.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM deployment_outbox
        WHERE state = 'dead_letter'
    ) THEN
        RAISE EXCEPTION 'cannot remove dead_letter state while dead-letter outbox rows exist';
    END IF;
END $$;

ALTER TABLE deployment_outbox
    DROP CONSTRAINT IF EXISTS deployment_outbox_state_check;

ALTER TABLE deployment_outbox
    ADD CONSTRAINT deployment_outbox_state_check
    CHECK (state IN ('pending', 'publishing', 'sent', 'failed'));
