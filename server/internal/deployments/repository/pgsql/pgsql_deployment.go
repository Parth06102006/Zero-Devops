// Package pgsql provides PostgreSQL repository implementations
package pgsql

import (
	"Zero_Devops/server/internal/deployments/contract"
	"Zero_Devops/server/internal/domain"
	"context"
	"database/sql"
	"encoding/json"
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
	output_url, error_message, created_at, updated_at
`

type deploymentRowScanner interface {
	Scan(dest ...any) error
}

func scanDeployment(row deploymentRowScanner) (domain.Deployment, error) {
	var d domain.Deployment
	var projectID, githubInstallationID, commitSHA, requestedRef, commandPolicyVersion sql.NullString
	var desiredRevisionGeneration sql.NullInt64
	var configurationVersion sql.NullInt32
	var manualIdempotencyKey, outputURL, errorMessage sql.NullString
	var configSnapshot, scanResult []byte

	if err := row.Scan(
		&d.ID, &d.UserID, &d.RepoID, &d.CloneURL, &d.Status,
		&projectID, &githubInstallationID, &commitSHA, &requestedRef, &d.Trigger,
		&desiredRevisionGeneration, &configSnapshot, &configurationVersion,
		&commandPolicyVersion, &scanResult, &manualIdempotencyKey,
		&outputURL, &errorMessage, &d.CreatedAt, &d.UpdatedAt,
	); err != nil {
		return domain.Deployment{}, err
	}

	d.ProjectID = projectID.String
	d.GithubInstallationID = githubInstallationID.String
	d.CommitSHA = commitSHA.String
	d.RequestedRef = requestedRef.String
	d.DesiredRevisionGeneration = desiredRevisionGeneration.Int64
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

func (m *pgSQLDeploymentRepository) Store(ctx context.Context, d *domain.Deployment) error {
	query := `
		INSERT INTO deployments (user_id, repo_id, clone_url, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	err := m.Conn.QueryRowContext(ctx, query,
		d.UserID, d.RepoID, d.CloneURL, d.Status, d.CreatedAt, d.UpdatedAt,
	).Scan(&d.ID)

	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to store deployment", zap.Error(err))
		return err
	}

	return nil
}

// StoreProjectBuild stores a project-scoped manual build with the immutable
// source/configuration snapshot required by deploy.jobs V1. Duplicate manual
// idempotency keys for the same project are treated as conflicts by PostgreSQL's
// unique index and are returned to the caller unchanged.
func (m *pgSQLDeploymentRepository) StoreProjectBuild(ctx context.Context, d *domain.Deployment) error {
	configSnapshot, err := json.Marshal(d.ConfigurationSnapshot)
	if err != nil {
		return fmt.Errorf("marshal configuration snapshot: %w", err)
	}
	scanResult, err := json.Marshal(d.CommandScanResult)
	if err != nil {
		return fmt.Errorf("marshal command scan result: %w", err)
	}

	query := `
		INSERT INTO deployments (
			user_id, repo_id, clone_url, status,
			project_id, github_installation_id, commit_sha, requested_ref, trigger,
			desired_revision_generation, configuration_snapshot, configuration_version,
			command_policy_version, command_scan_result, manual_idempotency_key,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		RETURNING id
	`
	err = m.Conn.QueryRowContext(ctx, query,
		d.UserID, d.RepoID, d.CloneURL, d.Status,
		d.ProjectID, d.GithubInstallationID, d.CommitSHA, d.RequestedRef, d.Trigger,
		d.DesiredRevisionGeneration, configSnapshot, d.ConfigurationVersion,
		d.CommandPolicyVersion, scanResult, d.ManualIdempotencyKey,
		d.CreatedAt, d.UpdatedAt,
	).Scan(&d.ID)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return domain.ErrConflict
		}
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to store project build deployment", zap.Error(err))
		return err
	}
	return nil
}

// StoreWebhookBuildWithOutbox durably records a webhook-triggered build and its
// deploy.jobs V1 outbox event in one transaction. The deployments row snapshots
// the project's approved configuration and the exact pushed commit; the outbox
// row carries the complete V1 BuildRequestV1 payload (with the generated
// deployment ID) so a later dispatcher can publish it to RabbitMQ without
// losing an accepted build. A redelivered webhook delivery violates the unique
// deployments.webhook_delivery_id index and is returned as domain.ErrConflict.
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
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

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
		DesiredRevisionGeneration: params.DesiredRevisionGeneration,
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
			desired_revision_generation, configuration_snapshot, configuration_version,
			command_policy_version, command_scan_result, webhook_delivery_id,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		RETURNING id
	`
	if err := tx.QueryRowContext(ctx, query,
		deployment.UserID, deployment.RepoID, deployment.CloneURL, deployment.Status,
		deployment.ProjectID, deployment.GithubInstallationID, deployment.CommitSHA, deployment.RequestedRef, deployment.Trigger,
		deployment.DesiredRevisionGeneration, configSnapshot, deployment.ConfigurationVersion,
		deployment.CommandPolicyVersion, scanResult, params.WebhookDeliveryID,
		deployment.CreatedAt, deployment.UpdatedAt,
	).Scan(&deployment.ID); err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
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
		Generation:     params.DesiredRevisionGeneration,
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

func (m *pgSQLDeploymentRepository) UpdateStatus(ctx context.Context, deploymentID string, status domain.DeploymentStatus) error {
	query := `
		UPDATE deployments
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`
	_, err := m.Conn.ExecContext(ctx, query, status, deploymentID)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to update deployment status", zap.Error(err))
		return err
	}

	return nil
}

func (m *pgSQLDeploymentRepository) UpdateOutputURL(ctx context.Context, deploymentID, outputURL string) error {
	query := `
		UPDATE deployments
		SET output_url = $1, updated_at = NOW()
		WHERE id = $2
	`
	_, err := m.Conn.ExecContext(ctx, query, outputURL, deploymentID)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to update deployment output URL", zap.Error(err))
		return err
	}

	return nil
}

func (m *pgSQLDeploymentRepository) UpdateErrorMessage(ctx context.Context, deploymentID, errMsg string) error {
	query := `
		UPDATE deployments
		SET error_message = $1, updated_at = NOW()
		WHERE id = $2
	`
	_, err := m.Conn.ExecContext(ctx, query, errMsg, deploymentID)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to update deployment error message", zap.Error(err))
		return err
	}

	return nil
}
