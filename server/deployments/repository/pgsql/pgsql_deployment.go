// Package pgsql provides PostgreSQL repository implementations
package pgsql

import (
	"Zero_Devops/server/domain"
	"context"
	"database/sql"

	appmiddleware "Zero_Devops/server/middleware"

	"go.uber.org/zap"
)

type pgSQLDeploymentRepository struct {
	Conn *sql.DB
}

// NewPgSQLDeploymentRepository creates a new DeploymentRepository backed by PostgreSQL
func NewPgSQLDeploymentRepository(conn *sql.DB) domain.DeploymentRepository {
	return &pgSQLDeploymentRepository{conn}
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

func (m *pgSQLDeploymentRepository) GetByUserID(ctx context.Context, userID int64) ([]domain.Deployment, error) {
	query := `
		SELECT id, user_id, repo_id, clone_url, status, created_at, updated_at
		FROM deployments
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := m.Conn.QueryContext(ctx, query, userID)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to query deployments by user ID", zap.Error(err))
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			appmiddleware.LoggerFromContext(ctx).Error("failed to close rows", zap.Error(err))
		}
	}()

	var deployments []domain.Deployment
	for rows.Next() {
		var d domain.Deployment
		err := rows.Scan(&d.ID, &d.UserID, &d.RepoID, &d.CloneURL, &d.Status, &d.CreatedAt, &d.UpdatedAt)
		if err != nil {
			log := appmiddleware.LoggerFromContext(ctx)
			log.Error("failed to scan deployment", zap.Error(err))
			return nil, err
		}
		deployments = append(deployments, d)
	}

	if deployments == nil {
		deployments = []domain.Deployment{}
	}

	return deployments, nil
}

func (m *pgSQLDeploymentRepository) GetByID(ctx context.Context, userID, id int64) (*domain.Deployment, error) {
	query := `
		SELECT id, user_id, repo_id, clone_url, status, created_at, updated_at
		FROM deployments
		WHERE id = $1 AND user_id = $2
	`
	res := m.Conn.QueryRowContext(ctx, query, id, userID)

	var d domain.Deployment
	err := res.Scan(&d.ID, &d.UserID, &d.RepoID, &d.CloneURL, &d.Status, &d.CreatedAt, &d.UpdatedAt)
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

func (m *pgSQLDeploymentRepository) UpdateStatus(ctx context.Context, deploymentID int64, status domain.DeploymentStatus) error {
	query := `
		UPDATE deployments
		SET status = $1
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
