package pgsql

import (
	"Zero_Devops/server/domain"
	"context"
	"database/sql"
	"fmt"
	appmiddleware "Zero_Devops/server/middleware"
	"go.uber.org/zap"
)

type pgSqlGithubRepository struct {
	Conn *sql.DB
}

func NewPgSqlGithubRepository(conn *sql.DB) domain.GithubRepository {
	return &pgSqlGithubRepository{conn}
}

func (m *pgSqlGithubRepository) StoreInstallation(ctx context.Context, inst *domain.GithubInstallation) error {
	query := `
		INSERT INTO github_installations (user_id, installation_id, account_type, account_login, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`
	if inst.Status == "" {
		inst.Status = domain.GithubInstallationStatusActive
	}
	err := m.Conn.QueryRowContext(ctx, query, inst.UserID, inst.InstallationID, inst.AccountType, inst.AccountLogin, inst.Status, inst.CreatedAt, inst.UpdatedAt).Scan(&inst.ID)

	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to insert github installation", zap.Error(err))
		return err
	}

	return nil
}

func (m *pgSqlGithubRepository) GetInstallationByUserID(ctx context.Context, userId int64) (*domain.GithubInstallation, error) {
	query := `
		SELECT id, user_id, installation_id, account_type, account_login, status, created_at, updated_at
		FROM github_installations
		WHERE user_id = $1
	`
	res := m.Conn.QueryRowContext(ctx, query, userId)

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

func (m *pgSqlGithubRepository) DeleteInstallationByUserID(ctx context.Context, userId int64) error {
	query := `DELETE FROM github_installations WHERE user_id = $1`
	stmt, err := m.Conn.PrepareContext(ctx, query)

	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to prepare delete query", zap.Error(err))
		return err
	}

	res, err := stmt.ExecContext(ctx, userId)
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

func (m *pgSqlGithubRepository) UpdateInstallationStatus(ctx context.Context, userId int64, status string) error {
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

	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, status, userId)
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
