// Package pgsql provides PostgreSQL repository implementations
package pgsql

import (
	"Zero_Devops/server/domain"
	"context"
	"database/sql"
	"fmt"

	appmiddleware "Zero_Devops/server/middleware"

	"go.uber.org/zap"
)

type pqSQLUserRepository struct {
	Conn *sql.DB
}

// NewPgSQLUserRepository creates a new UserRepository backed by PostgreSQL
func NewPgSQLUserRepository(conn *sql.DB) domain.UserRepository {
	return &pqSQLUserRepository{conn}
}

func (m *pqSQLUserRepository) GetByID(ctx context.Context, id int64) (domain.User, error) {
	query := `
		SELECT id, provider_id, provider, username, COALESCE(email, ''), COALESCE(avatar_url, ''), created_at, COALESCE(refresh_token, '')
		FROM users
		WHERE id = $1
	`
	row := m.Conn.QueryRowContext(ctx, query, id)

	var u domain.User
	err := row.Scan(
		&u.ID,
		&u.ProviderID,
		&u.Provider,
		&u.Username,
		&u.Email,
		&u.AvatarURL,
		&u.CreatedAt,
		&u.RefreshToken,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.User{}, domain.ErrNotFound
		}
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to scan user", zap.Error(err))
		return domain.User{}, err
	}

	return u, nil
}

func (m *pqSQLUserRepository) GetByUsername(ctx context.Context, username string) (domain.User, error) {
	query := `
		SELECT id, provider_id, provider, username, COALESCE(email, ''), COALESCE(avatar_url, ''), created_at, COALESCE(refresh_token, '')
		FROM users
		WHERE username = $1
	`
	row := m.Conn.QueryRowContext(ctx, query, username)

	u := domain.User{}
	err := row.Scan(
		&u.ID,
		&u.ProviderID,
		&u.Provider,
		&u.Username,
		&u.Email,
		&u.AvatarURL,
		&u.CreatedAt,
		&u.RefreshToken,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return domain.User{}, domain.ErrNotFound
		}
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to scan user by username", zap.Error(err))
		return domain.User{}, err
	}

	return u, nil
}

func (m *pqSQLUserRepository) GetProviderByID(ctx context.Context, providerID int64) (domain.User, error) {
	query := `
		SELECT id, provider_id, provider, username, COALESCE(email, ''), COALESCE(avatar_url, ''), created_at, COALESCE(refresh_token, '')
		FROM users
		WHERE provider_id = $1
	`
	row := m.Conn.QueryRowContext(ctx, query, providerID)

	u := domain.User{}
	err := row.Scan(
		&u.ID,
		&u.ProviderID,
		&u.Provider,
		&u.Username,
		&u.Email,
		&u.AvatarURL,
		&u.CreatedAt,
		&u.RefreshToken,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return domain.User{}, domain.ErrNotFound
		}
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to scan user by provider", zap.Error(err))
		return domain.User{}, err
	}

	return u, nil
}

func (m *pqSQLUserRepository) Store(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (provider_id, provider, username, email, avatar_url, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`

	err := m.Conn.QueryRowContext(ctx, query, user.ProviderID, user.Provider, user.Username, user.Email, user.AvatarURL, user.CreatedAt).Scan(&user.ID)

	if err != nil {
		return err
	}

	return nil
}

func (m *pqSQLUserRepository) UpdateRefreshToken(ctx context.Context, id int64, refreshToken string) error {
	query := `UPDATE users SET refresh_token = $1 WHERE id = $2`

	stmt, err := m.Conn.PrepareContext(ctx, query)

	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to prepare update query", zap.Error(err))
		return err
	}

	defer func() {
		if err := stmt.Close(); err != nil {
			appmiddleware.LoggerFromContext(ctx).Error("failed to close statement", zap.Error(err))
		}
	}()

	res, err := stmt.ExecContext(ctx, refreshToken, id)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to execute update query", zap.Error(err))
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
