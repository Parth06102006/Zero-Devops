// Package pgsql provides PostgreSQL repository implementations
package pgsql

import (
	"Zero_Devops/server/internal/deployments/contract"
	"Zero_Devops/server/internal/domain"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	appmiddleware "Zero_Devops/server/internal/middleware"

	"github.com/lib/pq"
	"go.uber.org/zap"
)

type pgSQLDeploymentRepository struct {
	Conn *sql.DB
}

// NewPgSQLDeploymentRepository creates a new DeploymentRepository backed by PostgreSQL
func NewPgSQLDeploymentRepository(conn *sql.DB) domain.DeploymentRepository {
	return &pgSQLDeploymentRepository{conn}
}

const deploymentColumns = `
	id, user_id, repo_id, clone_url, status,
	project_id, github_installation_id, commit_sha, requested_ref, trigger,
	desired_revision_generation, configuration_snapshot, configuration_version,
	command_policy_version, command_scan_result, manual_idempotency_key,
	output_url, error_message, created_at, updated_at, build_number
`

type deploymentRowScanner interface {
	Scan(dest ...any) error
}

// isUniqueViolation reports whether err is a PostgreSQL unique-violation
// (SQLSTATE 23505) on one of the named constraints. Other unique violations
// are surfaced to the caller as plain errors instead of domain.ErrConflict.
func isUniqueViolation(err error, constraints ...string) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) || pqErr.Code != "23505" {
		return false
	}
	for _, constraint := range constraints {
		if pqErr.Constraint == constraint {
			return true
		}
	}
	return false
}

func scanDeployment(row deploymentRowScanner) (domain.Deployment, error) {
	var d domain.Deployment
	var projectID, githubInstallationID, commitSHA, requestedRef, commandPolicyVersion sql.NullString
	var desiredRevisionGeneration sql.NullInt64
	var configurationVersion sql.NullInt32
	var manualIdempotencyKey, outputURL, errorMessage sql.NullString
	var buildNumber sql.NullInt64
	var configSnapshot, scanResult []byte

	if err := row.Scan(
		&d.ID, &d.UserID, &d.RepoID, &d.CloneURL, &d.Status,
		&projectID, &githubInstallationID, &commitSHA, &requestedRef, &d.Trigger,
		&desiredRevisionGeneration, &configSnapshot, &configurationVersion,
		&commandPolicyVersion, &scanResult, &manualIdempotencyKey,
		&outputURL, &errorMessage, &d.CreatedAt, &d.UpdatedAt, &buildNumber,
	); err != nil {
		return domain.Deployment{}, err
	}

	d.ProjectID = projectID.String
	d.GithubInstallationID = githubInstallationID.String
	d.CommitSHA = commitSHA.String
	d.RequestedRef = requestedRef.String
	d.DesiredRevisionGeneration = desiredRevisionGeneration.Int64
	d.BuildNumber = buildNumber.Int64
	d.ConfigurationVersion = int(configurationVersion.Int32)
	d.CommandPolicyVersion = commandPolicyVersion.String
	d.ManualIdempotencyKey = manualIdempotencyKey.String
	d.OutputURL = outputURL.String
	d.ErrorMessage = errorMessage.String

	if len(configSnapshot) > 0 {
		if err := json.Unmarshal(configSnapshot, &d.ConfigurationSnapshot); err != nil {
			return domain.Deployment{}, fmt.Errorf("unmarshal configuration snapshot: %w", err)
		}
	}
	if len(scanResult) > 0 {
		if err := json.Unmarshal(scanResult, &d.CommandScanResult); err != nil {
			return domain.Deployment{}, fmt.Errorf("unmarshal command scan result: %w", err)
		}
	}

	return d, nil
}

// (Legacy Store/StoreProjectBuild were removed 2026-09-06: Store's only
// caller was the deleted CreateDeployment, and StoreProjectBuild lost its
// last caller when the manual-build usecase switched to
// StoreProjectBuildWithOutbox. Writes go exclusively through the
// WithOutbox variants, which add the transactional outbox event.)

// StoreWebhookBuildWithOutbox durably records a webhook-triggered build and its
// deploy.jobs V1 outbox event in one transaction. The deployments row snapshots
// the project's approved configuration and the exact pushed commit; the outbox
// row carries the complete V1 BuildRequestV1 payload (with the generated
// deployment ID) so a later dispatcher can publish it to RabbitMQ without
// losing an accepted build. The project's desired-revision generation is
// advanced and a per-project build number is consumed inside the same
// transaction, so a redelivered webhook delivery that violates the unique
// deployments.webhook_delivery_id index rolls the generation bump back with the
// failed insert and is returned as domain.ErrConflict.
func (m *pgSQLDeploymentRepository) StoreWebhookBuildWithOutbox(ctx context.Context, params domain.StoreWebhookBuildParams) (*domain.Deployment, error) {
	configSnapshot, err := json.Marshal(params.ConfigurationSnapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal configuration snapshot: %w", err)
	}
	scanResult, err := json.Marshal(params.CommandScanResult)
	if err != nil {
		return nil, fmt.Errorf("marshal command scan result: %w", err)
	}

	now := time.Now()
	tx, err := m.Conn.BeginTx(ctx, nil)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to begin webhook build outbox transaction", zap.Error(err))
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Advance the desired-revision generation and consume a build number in
	// the same transaction as the build insert (bug fix 2026-09-06): the
	// generation used to be incremented by the webhook handler before this
	// call, so a redelivered delivery bumped the counter and then hit the
	// (webhook_delivery_id) unique violation — leaving the newest real build
	// permanently reported as IsCurrentGeneration=false. Doing both here
	// means a duplicate rolls the bump back with the failed insert.
	var generation, buildNumber int64
	if err := tx.QueryRowContext(ctx, `
		UPDATE projects
		SET desired_revision_generation = desired_revision_generation + 1,
			next_build_number = next_build_number + 1
		WHERE id = $1
		RETURNING desired_revision_generation, next_build_number
	`, params.ProjectID).Scan(&generation, &buildNumber); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to advance project revision generation and build number", zap.Error(err))
		return nil, err
	}

	deployment := &domain.Deployment{
		UserID:                    params.UserID,
		RepoID:                    params.RepoID,
		CloneURL:                  params.CloneURL,
		Status:                    domain.DeploymentStatusPending,
		ProjectID:                 params.ProjectID,
		GithubInstallationID:      params.GithubInstallationID,
		CommitSHA:                 params.CommitSHA,
		RequestedRef:              params.RequestedRef,
		Trigger:                   contract.TriggerWebhookPush,
		DesiredRevisionGeneration: generation,
		BuildNumber:               buildNumber,
		ConfigurationSnapshot:     params.ConfigurationSnapshot,
		ConfigurationVersion:      params.ConfigurationVersion,
		CommandPolicyVersion:      params.CommandPolicyVersion,
		CommandScanResult:         params.CommandScanResult,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}

	query := `
		INSERT INTO deployments (
			user_id, repo_id, clone_url, status,
			project_id, github_installation_id, commit_sha, requested_ref, trigger,
			desired_revision_generation, build_number, configuration_snapshot, configuration_version,
			command_policy_version, command_scan_result, webhook_delivery_id,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		RETURNING id
	`
	if err := tx.QueryRowContext(ctx, query,
		deployment.UserID, deployment.RepoID, deployment.CloneURL, deployment.Status,
		deployment.ProjectID, deployment.GithubInstallationID, deployment.CommitSHA, deployment.RequestedRef, deployment.Trigger,
		deployment.DesiredRevisionGeneration, deployment.BuildNumber, configSnapshot, deployment.ConfigurationVersion,
		deployment.CommandPolicyVersion, scanResult, params.WebhookDeliveryID,
		deployment.CreatedAt, deployment.UpdatedAt,
	).Scan(&deployment.ID); err != nil {
		if isUniqueViolation(err, "deployments_webhook_delivery_id_key") {
			return nil, domain.ErrConflict
		}
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to store webhook build deployment", zap.Error(err))
		return nil, err
	}

	job := contract.BuildRequestV1{
		Version:        contract.VersionV1,
		EventID:        params.EventID,
		DeploymentID:   deployment.ID,
		ProjectID:      params.ProjectID,
		InstallationID: params.InstallationID,
		RepositoryID:   params.RepoID,
		CloneURL:       params.CloneURL,
		CommitSHA:      params.CommitSHA,
		RequestedRef:   params.RequestedRef,
		Trigger:        contract.TriggerWebhookPush,
		Generation:     generation,
		RetryCount:     0,
		CorrelationID:  params.CorrelationID,
		Configuration: contract.Configuration{
			Executable:           params.ConfigurationSnapshot.Executable,
			Args:                 params.ConfigurationSnapshot.Args,
			WorkingDir:           params.ConfigurationSnapshot.WorkingDir,
			ScannerPolicyVersion: params.ConfigurationSnapshot.ScannerPolicyVersion,
		},
	}
	if err := job.Validate(); err != nil {
		return nil, fmt.Errorf("invalid deploy.jobs V1 outbox payload: %w", err)
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return nil, fmt.Errorf("marshal deploy.jobs V1 payload: %w", err)
	}

	outboxQuery := `
		INSERT INTO deployment_outbox (
			deployment_id, event_type, message_version, payload, state
		) VALUES ($1, $2, $3, $4, $5)
	`
	if _, err := tx.ExecContext(ctx, outboxQuery,
		deployment.ID, domain.WebhookBuildEventType, contract.VersionV1, payload, domain.OutboxStatePending,
	); err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to store webhook build outbox event", zap.Error(err))
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to commit webhook build outbox transaction", zap.Error(err))
		return nil, err
	}

	return deployment, nil
}

func (m *pgSQLDeploymentRepository) GetByUserID(ctx context.Context, userID string) ([]domain.Deployment, error) {
	query := `
		SELECT ` + deploymentColumns + `
		FROM deployments
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	return m.queryDeployments(ctx, query, userID)
}

func (m *pgSQLDeploymentRepository) GetByProjectID(ctx context.Context, userID, projectID string) ([]domain.Deployment, error) {
	query := `
		SELECT ` + deploymentColumns + `
		FROM deployments
		WHERE project_id = $1 AND user_id = $2
		ORDER BY created_at DESC
	`
	return m.queryDeployments(ctx, query, projectID, userID)
}

func (m *pgSQLDeploymentRepository) GetByID(ctx context.Context, userID, id string) (*domain.Deployment, error) {
	query := `
		SELECT ` + deploymentColumns + `
		FROM deployments
		WHERE id = $1 AND user_id = $2
	`
	d, err := scanDeployment(m.Conn.QueryRowContext(ctx, query, id, userID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to scan deployment by ID", zap.Error(err))
		return nil, err
	}

	return &d, nil
}

func (m *pgSQLDeploymentRepository) queryDeployments(ctx context.Context, query string, args ...any) ([]domain.Deployment, error) {
	rows, err := m.Conn.QueryContext(ctx, query, args...)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to query deployments", zap.Error(err))
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			appmiddleware.LoggerFromContext(ctx).Error("failed to close rows", zap.Error(err))
		}
	}()

	deployments := []domain.Deployment{}
	for rows.Next() {
		d, err := scanDeployment(rows)
		if err != nil {
			log := appmiddleware.LoggerFromContext(ctx)
			log.Error("failed to scan deployment", zap.Error(err))
			return nil, err
		}
		deployments = append(deployments, d)
	}
	if err := rows.Err(); err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to iterate deployments", zap.Error(err))
		return nil, err
	}

	return deployments, nil
}

// (Legacy UpdateStatus/UpdateOutputURL/UpdateErrorMessage repository methods
// were deleted 2026-09-06 together with the legacy auto-ack consumer: the
// durable deploy.status consumer applies all three fields atomically through
// ApplyStatusUpdate, and these single-field updates had no other callers.)

// StoreProjectBuildWithOutbox durably records a manual project build and its
// deploy.jobs V1 outbox event in one transaction. Duplicate manual idempotency
// keys for the same project map to domain.ErrConflict.
func (m *pgSQLDeploymentRepository) StoreProjectBuildWithOutbox(ctx context.Context, params domain.StoreProjectBuildWithOutboxParams) (*domain.Deployment, error) {
	if params.Deployment == nil {
		return nil, fmt.Errorf("deployment is required")
	}
	d := params.Deployment

	configSnapshot, err := json.Marshal(d.ConfigurationSnapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal configuration snapshot: %w", err)
	}
	scanResult, err := json.Marshal(d.CommandScanResult)
	if err != nil {
		return nil, fmt.Errorf("marshal command scan result: %w", err)
	}

	now := time.Now()
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	if d.UpdatedAt.IsZero() {
		d.UpdatedAt = now
	}
	if d.Status == "" {
		d.Status = domain.DeploymentStatusPending
	}
	if d.Trigger == "" {
		d.Trigger = contract.TriggerManual
	}
	if params.DesiredRevisionGeneration != 0 {
		d.DesiredRevisionGeneration = params.DesiredRevisionGeneration
	}

	tx, err := m.Conn.BeginTx(ctx, nil)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to begin project build outbox transaction", zap.Error(err))
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Consume a per-project build number inside the transaction (shared
	// sequence with webhook builds). The projects row lock is held until
	// commit, so concurrent builds can never receive the same number, and a
	// failed build rolls the number back with the insert.
	var buildNumber int64
	if err := tx.QueryRowContext(ctx, `
		UPDATE projects
		SET next_build_number = next_build_number + 1
		WHERE id = $1
		RETURNING next_build_number
	`, d.ProjectID).Scan(&buildNumber); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to consume project build number", zap.Error(err))
		return nil, err
	}
	d.BuildNumber = buildNumber

	query := `
		INSERT INTO deployments (
			user_id, repo_id, clone_url, status,
			project_id, github_installation_id, commit_sha, requested_ref, trigger,
			desired_revision_generation, build_number, configuration_snapshot, configuration_version,
			command_policy_version, command_scan_result, manual_idempotency_key,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		RETURNING id
	`
	if err := tx.QueryRowContext(ctx, query,
		d.UserID, d.RepoID, d.CloneURL, d.Status,
		d.ProjectID, d.GithubInstallationID, d.CommitSHA, d.RequestedRef, d.Trigger,
		d.DesiredRevisionGeneration, d.BuildNumber, configSnapshot, d.ConfigurationVersion,
		d.CommandPolicyVersion, scanResult, d.ManualIdempotencyKey,
		d.CreatedAt, d.UpdatedAt,
	).Scan(&d.ID); err != nil {
		if isUniqueViolation(err, "deployments_project_manual_idempotency_key") {
			return nil, domain.ErrConflict
		}
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to store project build deployment", zap.Error(err))
		return nil, err
	}

	job := contract.BuildRequestV1{
		Version:        contract.VersionV1,
		EventID:        params.EventID,
		DeploymentID:   d.ID,
		ProjectID:      d.ProjectID,
		InstallationID: params.InstallationID,
		RepositoryID:   d.RepoID,
		CloneURL:       d.CloneURL,
		CommitSHA:      d.CommitSHA,
		RequestedRef:   d.RequestedRef,
		Trigger:        contract.TriggerManual,
		Generation:     d.DesiredRevisionGeneration,
		RetryCount:     0,
		CorrelationID:  params.CorrelationID,
		Configuration: contract.Configuration{
			Executable:           d.ConfigurationSnapshot.Executable,
			Args:                 d.ConfigurationSnapshot.Args,
			WorkingDir:           d.ConfigurationSnapshot.WorkingDir,
			ScannerPolicyVersion: d.ConfigurationSnapshot.ScannerPolicyVersion,
		},
	}
	if err := job.Validate(); err != nil {
		return nil, fmt.Errorf("invalid deploy.jobs V1 outbox payload: %w", err)
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return nil, fmt.Errorf("marshal deploy.jobs V1 payload: %w", err)
	}

	outboxQuery := `
		INSERT INTO deployment_outbox (
			deployment_id, event_type, message_version, payload, state
		) VALUES ($1, $2, $3, $4, $5)
	`
	if _, err := tx.ExecContext(ctx, outboxQuery,
		d.ID, domain.WebhookBuildEventType, contract.VersionV1, payload, domain.OutboxStatePending,
	); err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to store project build outbox event", zap.Error(err))
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to commit project build outbox transaction", zap.Error(err))
		return nil, err
	}

	return d, nil
}

const (
	outboxClaimLease         = 30 * time.Second
	maxOutboxPublishAttempts = 5
)

func (m *pgSQLDeploymentRepository) ClaimOutboxBatch(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	if limit <= 0 {
		return []domain.OutboxEvent{}, nil
	}

	// The inner SELECT locks only the rows selected by this poller. The outer
	// UPDATE changes and returns those rows in the same atomic statement. A
	// single statement is sufficient here; the row locks are held until it
	// completes, and are not held while the caller publishes to RabbitMQ.
	query := `
		UPDATE deployment_outbox AS o
		SET state = $1,
			attempt_count = o.attempt_count + 1,
			available_at = NOW(),
			updated_at = NOW()
		WHERE o.id IN (
			SELECT candidate.id
			FROM deployment_outbox AS candidate
			WHERE (
					(
						candidate.state IN ($2, $3)
						AND candidate.attempt_count < $5
						AND candidate.available_at <= NOW()
					)
					OR (
						candidate.state = $4
						AND candidate.attempt_count < $5
						AND candidate.updated_at < $6
					)
				)
			ORDER BY candidate.available_at ASC, candidate.created_at ASC
			LIMIT $7
			FOR UPDATE SKIP LOCKED
		)
		RETURNING
			o.id,
			o.deployment_id,
			o.event_type,
			o.message_version,
			o.payload,
			o.state,
			o.attempt_count,
			o.available_at,
			o.sent_at,
			o.last_error,
			o.created_at,
			o.updated_at
	`

	rows, err := m.Conn.QueryContext(
		ctx,
		query,
		domain.OutboxStatePublishing,
		domain.OutboxStatePending,
		domain.OutboxStateFailed,
		domain.OutboxStatePublishing,
		maxOutboxPublishAttempts,
		time.Now().Add(-outboxClaimLease),
		limit,
	)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to claim outbox batch", zap.Int("limit", limit), zap.Error(err))
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			appmiddleware.LoggerFromContext(ctx).Error("failed to close claimed outbox rows", zap.Error(err))
		}
	}()

	events := make([]domain.OutboxEvent, 0, limit)
	for rows.Next() {
		var event domain.OutboxEvent
		var sentAt sql.NullTime
		var lastError sql.NullString

		if err := rows.Scan(
			&event.ID,
			&event.DeploymentID,
			&event.EventType,
			&event.MessageVersion,
			&event.Payload,
			&event.State,
			&event.AttemptCount,
			&event.AvailableAt,
			&sentAt,
			&lastError,
			&event.CreatedAt,
			&event.UpdatedAt,
		); err != nil {
			log := appmiddleware.LoggerFromContext(ctx)
			log.Error("failed to scan claimed outbox event", zap.Error(err))
			return nil, err
		}

		if sentAt.Valid {
			event.SentAt = &sentAt.Time
		}
		if lastError.Valid {
			event.LastError = lastError.String
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to iterate claimed outbox events", zap.Error(err))
		return nil, err
	}

	return events, nil
}

func (m *pgSQLDeploymentRepository) MarkOutboxSent(ctx context.Context, id string) error {
	query := `
		UPDATE deployment_outbox
		SET state = $1,
			sent_at = NOW(),
			last_error = NULL,
			updated_at = NOW()
		WHERE id = $2
			AND state = $3
	`

	result, err := m.Conn.ExecContext(
		ctx,
		query,
		domain.OutboxStateSent,
		id,
		domain.OutboxStatePublishing,
	)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to mark outbox event sent", zap.String("outbox_id", id), zap.Error(err))
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to determine whether outbox event was marked sent", zap.String("outbox_id", id), zap.Error(err))
		return err
	}
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (m *pgSQLDeploymentRepository) MarkOutboxPublishFailed(ctx context.Context, id string, errMsg string, nextAvailableAt time.Time) error {
	query := `
		UPDATE deployment_outbox
		SET state = CASE
				WHEN attempt_count >= $1 THEN $2
				ELSE $3
			END,
			last_error = $4,
			available_at = $5,
			updated_at = NOW()
		WHERE id = $6
			AND state = $7
	`

	result, err := m.Conn.ExecContext(
		ctx,
		query,
		maxOutboxPublishAttempts,
		domain.OutboxStateDeadLetter,
		domain.OutboxStateFailed,
		errMsg,
		nextAvailableAt,
		id,
		domain.OutboxStatePublishing,
	)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to mark outbox event publish failed", zap.String("outbox_id", id), zap.Error(err))
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to determine whether outbox event was marked failed", zap.String("outbox_id", id), zap.Error(err))
		return err
	}
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// ResetStuckPublishing returns outbox rows stuck in the publishing state back
// to pending so a crashed poller's claims can be retried. attempt_count is
// preserved so the normal retry/dead-letter policy still applies.
func (m *pgSQLDeploymentRepository) ResetStuckPublishing(ctx context.Context, olderThan time.Duration) (int64, error) {
	if olderThan <= 0 {
		return 0, domain.ErrBadParamInput
	}

	cutoff := time.Now().Add(-olderThan)
	query := `
		UPDATE deployment_outbox
		SET state = $1,
			available_at = NOW(),
			updated_at = NOW()
		WHERE state = $2
			AND updated_at < $3
	`

	result, err := m.Conn.ExecContext(ctx, query,
		domain.OutboxStatePending,
		domain.OutboxStatePublishing,
		cutoff,
	)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to reset stuck publishing outbox events", zap.Duration("older_than", olderThan), zap.Error(err))
		return 0, err
	}

	reset, err := result.RowsAffected()
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to determine reset stuck publishing outbox events", zap.Duration("older_than", olderThan), zap.Error(err))
		return 0, err
	}

	return reset, nil
}

func isTerminalDeploymentStatus(status domain.DeploymentStatus) bool {
	switch status {
	case domain.DeploymentStatusSuccess, domain.DeploymentStatusFailed, domain.DeploymentStatusCanceled:
		return true
	default:
		return false
	}
}

func (m *pgSQLDeploymentRepository) ApplyStatusUpdate(ctx context.Context, params domain.ApplyStatusParams) (*domain.ApplyStatusResult, error) {
	if params.DeploymentID == "" {
		return nil, domain.ErrBadParamInput
	}

	switch params.Status {
	case domain.DeploymentStatusPending,
		domain.DeploymentStatusBuilding,
		domain.DeploymentStatusSuccess,
		domain.DeploymentStatusFailed,
		domain.DeploymentStatusCanceled:
	default:
		return nil, domain.ErrInvalidStatus
	}

	tx, err := m.Conn.BeginTx(ctx, nil)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to begin deployment status update transaction", zap.Error(err))
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Lock the deployment while reading it so concurrent worker status messages
	// cannot update the same row based on stale data.
	selectQuery := `
		SELECT ` + deploymentColumns + `
		FROM deployments
		WHERE id = $1
		FOR UPDATE
	`
	deploymentRow, err := scanDeployment(tx.QueryRowContext(ctx, selectQuery, params.DeploymentID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to load deployment for status update", zap.String("deployment_id", params.DeploymentID), zap.Error(err))
		return nil, err
	}

	// Terminal statuses are immutable: once a deployment is success, failed, or
	// canceled, only an idempotent update to the same status is accepted. This
	// stops out-of-order or duplicate worker messages from regressing a finished
	// deployment (for example success back to building).
	if isTerminalDeploymentStatus(deploymentRow.Status) && deploymentRow.Status != params.Status {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Warn("rejected deployment status transition from terminal state",
			zap.String("deployment_id", params.DeploymentID),
			zap.String("current_status", string(deploymentRow.Status)),
			zap.String("requested_status", string(params.Status)))
		return nil, domain.ErrInvalidStatusTransition
	}

	var desiredGeneration sql.NullInt64
	if deploymentRow.ProjectID != "" {
		// Lock the project row as well so IsCurrentGeneration is evaluated at the
		// same serialization point as the deployment update; a concurrent project
		// generation bump cannot slip in between the read and the commit.
		projectQuery := `
			SELECT desired_revision_generation
			FROM projects
			WHERE id = $1
			FOR UPDATE
		`
		if err := tx.QueryRowContext(ctx, projectQuery, deploymentRow.ProjectID).Scan(&desiredGeneration); err != nil && err != sql.ErrNoRows {
			log := appmiddleware.LoggerFromContext(ctx)
			log.Error("failed to load project generation for deployment status update", zap.String("deployment_id", params.DeploymentID), zap.Error(err))
			return nil, err
		}
	}

	updateQuery := `
		UPDATE deployments
		SET status = $1,
			output_url = CASE WHEN $2 <> '' THEN $2 ELSE output_url END,
			error_message = CASE WHEN $3 <> '' THEN $3 ELSE error_message END,
			updated_at = NOW()
		WHERE id = $4
		RETURNING updated_at
	`
	if err := tx.QueryRowContext(ctx, updateQuery,
		params.Status,
		params.OutputURL,
		params.ErrorMessage,
		params.DeploymentID,
	).Scan(&deploymentRow.UpdatedAt); err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to apply deployment status update", zap.String("deployment_id", params.DeploymentID), zap.Error(err))
		return nil, err
	}

	deploymentRow.Status = params.Status
	if params.OutputURL != "" {
		deploymentRow.OutputURL = params.OutputURL
	}
	if params.ErrorMessage != "" {
		deploymentRow.ErrorMessage = params.ErrorMessage
	}

	if err := tx.Commit(); err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to commit deployment status update", zap.String("deployment_id", params.DeploymentID), zap.Error(err))
		return nil, err
	}

	return &domain.ApplyStatusResult{
		Deployment:          &deploymentRow,
		IsCurrentGeneration: desiredGeneration.Valid && deploymentRow.DesiredRevisionGeneration == desiredGeneration.Int64,
	}, nil
}

func (m *pgSQLDeploymentRepository) DeleteSentOutboxOlderThan(ctx context.Context, olderThan time.Duration) (int64, error) {
	if olderThan <= 0 {
		return 0, domain.ErrBadParamInput
	}

	cutoff := time.Now().Add(-olderThan)
	query := `
		DELETE FROM deployment_outbox
		WHERE state = $1
			AND sent_at IS NOT NULL
			AND sent_at < $2
	`

	result, err := m.Conn.ExecContext(ctx, query, domain.OutboxStateSent, cutoff)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to delete sent outbox events", zap.Error(err))
		return 0, err
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to determine deleted sent outbox events", zap.Error(err))
		return 0, err
	}

	return deleted, nil
}
