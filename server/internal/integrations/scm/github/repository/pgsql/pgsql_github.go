// Package pgsql provides PostgreSQL repository implementations
package pgsql

import (
	"Zero_Devops/server/internal/domain"
	appmiddleware "Zero_Devops/server/internal/middleware"
	"context"
	"database/sql"
	"fmt"
	"time"

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
	log := appmiddleware.LoggerFromContext(ctx)

	tx, err := m.Conn.BeginTx(ctx, nil)
	if err != nil {
		log.Error("failed to begin transaction for store installation", zap.Error(err))
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	now := time.Now()
	if inst.CreatedAt.IsZero() {
		inst.CreatedAt = now
	}
	if inst.UpdatedAt.IsZero() {
		inst.UpdatedAt = now
	}
	if inst.Status == "" {
		inst.Status = domain.GithubInstallationStatusActive
	}
	if inst.ConnectionStatus == "" {
		inst.ConnectionStatus = domain.GithubConnectionStatusConnected
	}

	// 1. Check if an installation with this installation_id already exists
	checkQuery := `
		SELECT id, user_id, status
		FROM github_installations
		WHERE installation_id = $1
		FOR UPDATE
	`
	var existingID string
	var existingUserID string
	var existingStatus string
	err = tx.QueryRowContext(ctx, checkQuery, inst.InstallationID).Scan(&existingID, &existingUserID, &existingStatus)

	if err == nil {
		// Security Check: Ensure installation belongs to the authenticated user.
		if existingUserID != inst.UserID {
			log.Warn("installation_id already claimed by another user",
				zap.Int64("installation_id", inst.InstallationID),
				zap.String("existing_user_id", existingUserID),
				zap.String("request_user_id", inst.UserID),
			)
			return domain.ErrInstallationOwnershipConflict
		}

		// Disconnect any other currently connected installations for this user
		disconnectQuery := `
			UPDATE github_installations
			SET connection_status = 'disconnected', updated_at = $1
			WHERE user_id = $2 AND connection_status = 'connected' AND id != $3
		`
		if _, err := tx.ExecContext(ctx, disconnectQuery, now, inst.UserID, existingID); err != nil {
			log.Error("failed to disconnect previous installations", zap.Error(err))
			return err
		}

		// Reactivate/update the existing installation
		updateQuery := `
			UPDATE github_installations
			SET account_type = $1, account_login = $2, status = $3, connection_status = $4, updated_at = $5
			WHERE id = $6
		`
		inst.Status = domain.GithubInstallationStatusActive
		inst.ConnectionStatus = domain.GithubConnectionStatusConnected
		inst.UpdatedAt = now
		inst.ID = existingID

		_, updateErr := tx.ExecContext(ctx, updateQuery,
			inst.AccountType, inst.AccountLogin,
			domain.GithubInstallationStatusActive, domain.GithubConnectionStatusConnected, now, existingID,
		)
		if updateErr != nil {
			log.Error("failed to reactivate/update github installation", zap.Error(updateErr))
			return updateErr
		}

		if err := tx.Commit(); err != nil {
			log.Error("failed to commit update github installation", zap.Error(err))
			return err
		}
		return nil
	} else if err != sql.ErrNoRows {
		log.Error("failed to query existing github installation", zap.Error(err))
		return err
	}

	// 2. New installation: disconnect any other currently connected installations for this user
	disconnectQuery := `
		UPDATE github_installations
		SET connection_status = 'disconnected', updated_at = $1
		WHERE user_id = $2 AND connection_status = 'connected'
	`
	if _, err := tx.ExecContext(ctx, disconnectQuery, now, inst.UserID); err != nil {
		log.Error("failed to disconnect previous installations", zap.Error(err))
		return err
	}

	// Insert the new installation (old installations for this user are retained as history/saved)
	insertQuery := `
		INSERT INTO github_installations (user_id, installation_id, account_type, account_login, status, connection_status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`
	err = tx.QueryRowContext(ctx, insertQuery,
		inst.UserID, inst.InstallationID, inst.AccountType,
		inst.AccountLogin, inst.Status, inst.ConnectionStatus, inst.CreatedAt, inst.UpdatedAt,
	).Scan(&inst.ID)

	if err != nil {
		log.Error("failed to insert github installation", zap.Error(err))
		return err
	}

	if err := tx.Commit(); err != nil {
		log.Error("failed to commit insert github installation", zap.Error(err))
		return err
	}

	return nil
}

func (m *pgSQLGithubRepository) GetInstallationByUserID(ctx context.Context, userID string) (*domain.GithubInstallation, error) {
	query := `
		SELECT id, user_id, installation_id, account_type, account_login, status, connection_status, created_at, updated_at
		FROM github_installations
		WHERE user_id = $1
		ORDER BY (connection_status = 'connected') DESC, (status = 'active') DESC, updated_at DESC
		LIMIT 1
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
		&inst.ConnectionStatus,
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

func (m *pgSQLGithubRepository) GetInstallationIDByGithubInstallationID(ctx context.Context, installationID int64) (string, error) {
	query := `
		SELECT id
		FROM github_installations
		WHERE installation_id = $1
	`

	var githubInstallationDBID string
	err := m.Conn.QueryRowContext(ctx, query, installationID).Scan(&githubInstallationDBID)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", domain.ErrNotFound
		}
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to get github installation", zap.Error(err))
		return "", err
	}

	return githubInstallationDBID, nil
}

func (m *pgSQLGithubRepository) GetInstallationStatusByID(ctx context.Context, installationDBID string) (string, error) {
	query := `
		SELECT status
		FROM github_installations
		WHERE id = $1
	`

	var status string
	err := m.Conn.QueryRowContext(ctx, query, installationDBID).Scan(&status)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", domain.ErrNotFound
		}
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to get github installation status", zap.Error(err))
		return "", err
	}

	return status, nil
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

func (m *pgSQLGithubRepository) UpdateInstallationStatusByGithubInstallationID(ctx context.Context, installationID int64, status string) error {
	if status != domain.GithubInstallationStatusActive &&
		status != domain.GithubInstallationStatusSuspended &&
		status != domain.GithubInstallationStatusUninstalled {
		return domain.ErrInvalidStatus
	}

	query := `UPDATE github_installations SET status = $1 WHERE installation_id = $2`

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

	res, err := stmt.ExecContext(ctx, status, installationID)
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

func (m *pgSQLGithubRepository) UpdateInstallationExternalIDByID(ctx context.Context, installationID, githubInstallationDBID string) error {
	query := `UPDATE github_installations SET installation_id = $1 WHERE id = $2`

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

	res, err := stmt.ExecContext(ctx, installationID, githubInstallationDBID)
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

func (m *pgSQLGithubRepository) GetInstallationByID(ctx context.Context, id string) (*domain.GithubInstallation, error) {
	query := `
		SELECT id, user_id, installation_id, account_type, account_login, status, connection_status, created_at, updated_at
		FROM github_installations
		WHERE id = $1
	`
	res := m.Conn.QueryRowContext(ctx, query, id)

	inst := domain.GithubInstallation{}
	err := res.Scan(
		&inst.ID,
		&inst.UserID,
		&inst.InstallationID,
		&inst.AccountType,
		&inst.AccountLogin,
		&inst.Status,
		&inst.ConnectionStatus,
		&inst.CreatedAt,
		&inst.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to get github installation by id", zap.Error(err))
		return nil, err
	}

	return &inst, nil
}

func (m *pgSQLGithubRepository) ListInstallationsByUserID(ctx context.Context, userID string) ([]*domain.GithubInstallation, error) {
	query := `
		SELECT id, user_id, installation_id, account_type, account_login, status, connection_status, created_at, updated_at
		FROM github_installations
		WHERE user_id = $1
		ORDER BY (connection_status = 'connected') DESC, updated_at DESC
	`
	rows, err := m.Conn.QueryContext(ctx, query, userID)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to list github installations", zap.Error(err))
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var installations []*domain.GithubInstallation
	for rows.Next() {
		inst := &domain.GithubInstallation{}
		if err := rows.Scan(
			&inst.ID,
			&inst.UserID,
			&inst.InstallationID,
			&inst.AccountType,
			&inst.AccountLogin,
			&inst.Status,
			&inst.ConnectionStatus,
			&inst.CreatedAt,
			&inst.UpdatedAt,
		); err != nil {
			return nil, err
		}
		installations = append(installations, inst)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return installations, nil
}

func (m *pgSQLGithubRepository) UpdateConnectionStatus(ctx context.Context, userID, installationDBID string, status domain.GithubConnectionStatus) error {
	if !status.IsValid() {
		return domain.ErrInvalidStatus
	}

	log := appmiddleware.LoggerFromContext(ctx)
	tx, err := m.Conn.BeginTx(ctx, nil)
	if err != nil {
		log.Error("failed to begin transaction for update connection status", zap.Error(err))
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	now := time.Now()
	if status == domain.GithubConnectionStatusConnected {
		// Disconnect any currently connected installation for this user first
		disconnectQuery := `
			UPDATE github_installations
			SET connection_status = 'disconnected', updated_at = $1
			WHERE user_id = $2 AND connection_status = 'connected' AND id != $3
		`
		if _, err := tx.ExecContext(ctx, disconnectQuery, now, userID, installationDBID); err != nil {
			log.Error("failed to disconnect previous installations", zap.Error(err))
			return err
		}

		connectQuery := `
			UPDATE github_installations
			SET connection_status = 'connected', updated_at = $1
			WHERE user_id = $2 AND id = $3
		`
		res, err := tx.ExecContext(ctx, connectQuery, now, userID, installationDBID)
		if err != nil {
			log.Error("failed to connect target installation", zap.Error(err))
			return err
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if rowsAffected == 0 {
			return domain.ErrNotFound
		}
	} else {
		disconnectQuery := `
			UPDATE github_installations
			SET connection_status = 'disconnected', updated_at = $1
			WHERE user_id = $2 AND id = $3
		`
		res, err := tx.ExecContext(ctx, disconnectQuery, now, userID, installationDBID)
		if err != nil {
			log.Error("failed to disconnect target installation", zap.Error(err))
			return err
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if rowsAffected == 0 {
			return domain.ErrNotFound
		}
	}

	if err := tx.Commit(); err != nil {
		log.Error("failed to commit update connection status", zap.Error(err))
		return err
	}

	return nil
}
