import (
	"context",
	"database/sql"
	"fmt"
	"github.com/sirupsen/logrus"
	"server/domain"
)

type pgSqlGithubRepository struct {
	Conn *sql.DB
}

func NewPgSqlGithubRepository(conn *sql.DB) domain.GithubRepository {
	return &pgSqlGithubRepository{conn}
}

func (m *pgSqlGithubRepository) StoreInstallation(ctx context.Context, inst *domain.GithubInstallation) error {
	query := `INSERT INTO Github (UserID , InstallationID , AccountName) VALUES ($1 , $2 , $3)`
	stmt,err := m.Conn.PrepareContext(ctx,query)
	
	if err != nil {
		logrus.Error(error)
		return err
	}
	
	res,err := stmt.ExecContext(ctx , inst.UserID, inst.InstallationID, inst.AccountName)
	
	if err != nil {
		logrus.Error(err)
		return err
	}
	
	inst.ID = res.LastInsertId()
	
	return nil
}

func (m *pgSqlGithubRepository) GetInstallationByUserID(ctx context.Context , userId int64) (*domain.GithubInstallation, error) {
	query := `SELECT * FROM Github WHERE UserID = $1`
	res,err := m.Conn.QueryRowContext(ctx , query , userId)
	
	if err != nil {
		logrus.Error(err)
		return nil , err
	}
	
	inst := domain.GithubInstallation{}
	err = res.Scan(
		&inst.ID,
		&inst.UserID,
		&inst.InstallationID,
		&inst.AccountName,
	)
	
	if err != nil {
		logrus.Error(err)
		return nil , err
	}
	
	return &inst , nil
}