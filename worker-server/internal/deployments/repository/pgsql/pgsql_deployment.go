// Package pgsql implements the DeploymentRepository interface using PostgreSQL.
package pgsql

import (
	"Zero_Devops/worker_server/internal/domain"
	"context"
	"database/sql"
	"fmt"
)

type pgSQLDeploymentRepository struct {
	Conn *sql.DB
}

// NewPgSQLDeploymentRepository creates a new DeploymentRepository backed by PostgreSQL.
func NewPgSQLDeploymentRepository(conn *sql.DB) domain.DeploymentRepository {
	return &pgSQLDeploymentRepository{conn}
}

func (m *pgSQLDeploymentRepository) Insert(ctx context.Context, job domain.DeployJob) error {
	imageTag := fmt.Sprintf("deploy-%s:latest", job.DeploymentID)
	query := `
		INSERT INTO deployments (id, clone_url, status, retry_count, image_tag)
		VALUES ($1, $2, 'pending', $3, $4)
		ON CONFLICT (id) DO UPDATE SET
			clone_url = EXCLUDED.clone_url,
			status = 'pending',
			retry_count = EXCLUDED.retry_count,
			image_tag = EXCLUDED.image_tag,
			updated_at = now()
	`
	_, err := m.Conn.ExecContext(ctx, query,
		job.DeploymentID, job.CloneURL, job.RetryCount, imageTag,
	)
	return err
}

func (m *pgSQLDeploymentRepository) UpdateStatus(ctx context.Context, deploymentID, status string, retryCount int) error {
	query := `UPDATE deployments SET status = $1, retry_count = $2, updated_at = NOW() WHERE id = $3`
	_, err := m.Conn.ExecContext(ctx, query, status, retryCount, deploymentID)
	return err
}

func (m *pgSQLDeploymentRepository) UpdateOutputURL(ctx context.Context, deploymentID, outputURL string) error {
	query := `UPDATE deployments SET output_url = $1, updated_at = NOW() WHERE id = $2`
	_, err := m.Conn.ExecContext(ctx, query, outputURL, deploymentID)
	return err
}

func (m *pgSQLDeploymentRepository) ReadImageTag(ctx context.Context, deploymentID string) (string, error) {
	query := `SELECT image_tag FROM deployments WHERE id = $1`
	row := m.Conn.QueryRowContext(ctx, query, deploymentID)
	var imageTag string
	if err := row.Scan(&imageTag); err != nil {
		return "", err
	}
	return imageTag, nil
}

func (m *pgSQLDeploymentRepository) MarkBuilding(ctx context.Context, deploymentID string) error {
	query := `UPDATE deployments SET status = 'building', started_at = NOW(), updated_at = NOW() WHERE id = $1`
	_, err := m.Conn.ExecContext(ctx, query, deploymentID)
	return err
}

func (m *pgSQLDeploymentRepository) MarkFailed(ctx context.Context, deploymentID, errMsg string) error {
	query := `UPDATE deployments SET status = 'failed', error_message = $1, finished_at = NOW(), updated_at = NOW() WHERE id = $2`
	_, err := m.Conn.ExecContext(ctx, query, errMsg, deploymentID)
	return err
}

func (m *pgSQLDeploymentRepository) MarkCanceled(ctx context.Context, deploymentID, errMsg string) error {
	query := `UPDATE deployments SET status = 'canceled', error_message = $1, finished_at = NOW(), updated_at = NOW() WHERE id = $2`
	_, err := m.Conn.ExecContext(ctx, query, errMsg, deploymentID)
	return err
}

func (m *pgSQLDeploymentRepository) MarkFinished(ctx context.Context, deploymentID, outputURL string) error {
	query := `UPDATE deployments SET status = 'success', error_message = '', output_url = $1, finished_at = NOW(), updated_at = NOW() WHERE id = $2`
	_, err := m.Conn.ExecContext(ctx, query, outputURL, deploymentID)
	return err
}
