package pgsql

import (
	"Zero_Devops/server/domain"
	"context"
	"database/sql"
	"fmt"
	"github.com/sirupsen/logrus"
)

type pgSqlGithubRepository struct {
	Conn *sql.DB
}

func NewPgSqlGithubRepository(conn *sql.DB) domain.GithubRepository {
	return &pgSqlGithubRepository{conn}
}

func (m *pgSqlGithubRepository) StoreInstallation(ctx context.Context, inst *domain.GithubInstallation) error {
	query := `
		INSERT INTO github_installations (user_id, installation_id, account_name)
		VALUES ($1, $2, $3)
		RETURNING id
	`
	err := m.Conn.QueryRowContext(ctx, query, inst.UserID, inst.InstallationID, inst.AccountName).Scan(&inst.ID)

	if err != nil {
		logrus.Error(err)
		return err
	}

	return nil
}

func (m *pgSqlGithubRepository) GetInstallationByUserID(ctx context.Context, userId int64) (*domain.GithubInstallation, error) {
	query := `
		SELECT id, user_id, installation_id, account_name
		FROM github_installations
		WHERE user_id = $1
	`
	res := m.Conn.QueryRowContext(ctx, query, userId)

	inst := domain.GithubInstallation{}
	err := res.Scan(
		&inst.ID,
		&inst.UserID,
		&inst.InstallationID,
		&inst.AccountName,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		logrus.Error(err)
		return nil, err
	}

	return &inst, nil
}

func (m *pgSqlGithubRepository) DeleteInstallationByUserID(ctx context.Context, userId int64) error {
	query := `DELETE FROM github_installations WHERE user_id = $1`
	stmt, err := m.Conn.PrepareContext(ctx, query)

	if err != nil {
		logrus.Error(err)
		return err
	}

	res, err := stmt.ExecContext(ctx, userId)
	if err != nil {
		logrus.Error(err)
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
