// Package pgsql provides PostgreSQL repository implementations
package pgsql

import (
	"Zero_Devops/server/internal/domain"
	appmiddleware "Zero_Devops/server/internal/middleware"
	"context"
	"database/sql"
	"fmt"

	"go.uber.org/zap"
)

type pgSQLGithubRepository struct {
	Conn *sql.DB
}

// NewPgSQLGithubRepository creates a new GithubRepository backed by PostgreSQL
func NewPgSQLGithubRepository(conn *sql.DB) domain.GithubRepository {
	return &pgSQLGithubRepository{conn}
}

func (m *pgSQLGithubRepository) StoreInstallation(ctx context.Context, inst *domain.GithubInstallation) error {
	query := `
		INSERT INTO github_installations (user_id, installation_id, account_type, account_login, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`
	if inst.Status == "" {
		inst.Status = domain.GithubInstallationStatusActive
	}
	err := m.Conn.QueryRowContext(ctx, query,
		inst.UserID, inst.InstallationID, inst.AccountType,
		inst.AccountLogin, inst.Status, inst.CreatedAt, inst.UpdatedAt,
	).Scan(&inst.ID)

	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to insert github installation", zap.Error(err))
		return err
	}

	return nil
}

func (m *pgSQLGithubRepository) GetInstallationByUserID(ctx context.Context, userID string) (*domain.GithubInstallation, error) {
	query := `
		SELECT id, user_id, installation_id, account_type, account_login, status, created_at, updated_at
		FROM github_installations
		WHERE user_id = $1
	`
	res := m.Conn.QueryRowContext(ctx, query, userID)

	inst := domain.GithubInstallation{}
	err := res.Scan(
		&inst.ID,
		&inst.UserID,
		&inst.InstallationID,
		&inst.AccountType,
		&inst.AccountLogin,
		&inst.Status,
		&inst.CreatedAt,
		&inst.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to get github installation", zap.Error(err))
		return nil, err
	}

	return &inst, nil
}

func (m *pgSQLGithubRepository) DeleteInstallationByUserID(ctx context.Context, userID string) error {
	query := `DELETE FROM github_installations WHERE user_id = $1`
	stmt, err := m.Conn.PrepareContext(ctx, query)

	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to prepare delete query", zap.Error(err))
		return err
	}

	defer func() {
		if err := stmt.Close(); err != nil {
			appmiddleware.LoggerFromContext(ctx).Error("failed to close statement", zap.Error(err))
		}
	}()

	res, err := stmt.ExecContext(ctx, userID)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to delete github installation", zap.Error(err))
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected != 1 {
		err = fmt.Errorf("weird  Behavior. Total Affected: %d", rowsAffected)
		return err
	}

	return nil
}

func (m *pgSQLGithubRepository) UpdateInstallationStatus(ctx context.Context, userID, status string) error {
	if status != domain.GithubInstallationStatusActive &&
		status != domain.GithubInstallationStatusSuspended &&
		status != domain.GithubInstallationStatusUninstalled {
		return domain.ErrInvalidStatus
	}

	query := `UPDATE github_installations SET status = $1 WHERE user_id = $2`

	stmt, err := m.Conn.PrepareContext(ctx, query)

	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to prepare status update query", zap.Error(err))
		return err
	}

	defer func() {
		if err := stmt.Close(); err != nil {
			appmiddleware.LoggerFromContext(ctx).Error("failed to close statement", zap.Error(err))
		}
	}()

	res, err := stmt.ExecContext(ctx, status, userID)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to update status", zap.Error(err))
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to get rows affected", zap.Error(err))
		return err
	}

	if rowsAffected != 1 {
		return fmt.Errorf("weird Behavior. Total Affected: %d", rowsAffected)
	}

	return nil
}
