package pgsql

import (
	"Zero_Devops/server/domain"
	"context"
	"database/sql"

	"github.com/sirupsen/logrus"
)

type pgSqlDeploymentRepository struct {
	Conn *sql.DB
}

func NewPgSqlDeploymentRepository(conn *sql.DB) domain.DeploymentRepository {
	return &pgSqlDeploymentRepository{conn}
}

func (m *pgSqlDeploymentRepository) Store(ctx context.Context, d *domain.Deployment) error {
	query := `
		INSERT INTO deployments (user_id, repo_id, clone_url, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	err := m.Conn.QueryRowContext(ctx, query,
		d.UserID, d.RepoID, d.CloneURL, d.Status, d.CreatedAt, d.UpdatedAt,
	).Scan(&d.ID)

	if err != nil {
		logrus.Error(err)
		return err
	}

	return nil
}

func (m *pgSqlDeploymentRepository) GetByUserID(ctx context.Context, userID int64) ([]domain.Deployment, error) {
	query := `
		SELECT id, user_id, repo_id, clone_url, status, created_at, updated_at
		FROM deployments
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := m.Conn.QueryContext(ctx, query, userID)
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	defer rows.Close()

	var deployments []domain.Deployment
	for rows.Next() {
		var d domain.Deployment
		err := rows.Scan(&d.ID, &d.UserID, &d.RepoID, &d.CloneURL, &d.Status, &d.CreatedAt, &d.UpdatedAt)
		if err != nil {
			logrus.Error(err)
			return nil, err
		}
		deployments = append(deployments, d)
	}

	if deployments == nil {
		deployments = []domain.Deployment{}
	}

	return deployments, nil
}

func (m *pgSqlDeploymentRepository) GetByID(ctx context.Context, userID, id int64) (*domain.Deployment, error) {
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
		logrus.Error(err)
		return nil, err
	}

	return &d, nil
}
