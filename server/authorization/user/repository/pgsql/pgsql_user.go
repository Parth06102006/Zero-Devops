import (
	"context"
	"database/sql"
	"fmt"
	"github.com/sirupsen/logrus"
	"server/domain"
)

type pqSqlUserRepository struct {
	Conn *sql.DB
}

func NewPgSqlUserRepository(conn *sql.DB) domain.UserRepository {
	return &pqSqlUserRepository{conn}
}

func (m *pqSqlUserRepository) GetByID (ctx context.Context , id int64) (domain.User, error) {
	query := `SELECT * FROM users WHERE ID = $1`
	row,err := m.Conn.QueryRowContext(ctx,query,id)
	if err != nil {
		logrus.Error(err)
		return nil ,err
	}
	
	defer func(){
		errRow := row.Close()
		if errRow != nil {
			logrus.Error(errRow)
		}
	}
	
	var u domain.User
	err = row.Scan(
		&u.ID,
		&u.ProviderId,
		&u.Provider,
		&u.Username,
		&u.Email,
		&u.AvatarURL,
		&u.CreatedAt,
		&u.RefreshToken,
	)
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	return u, nil
}

func (m *pqSqlUserRepository) GetByUsername (ctx context.Context , username string) (domain.User , error){
	query := `SELECT * FROM users WHERE Username = $1`
	row,err := m.Conn.QueryRowContext(ctx,query,username)
	
	if err != nil{
		logrus.Error(err)
		return nil,err
	}
	
	defer func(){
		errRow := row.Close()
		if errRow != nil {
			logrus.Error(errRow)
		}
	}
	
	u = domain.User{}
	err = row.Scan(
		&u.ID,
		&u.ProviderId,
		&u.Provider,
		&u.Username,
		&u.Email,
		&u.AvatarURL,
		&u.CreatedAt,
		&u.RefreshToken,
	)
	
	if err != nil {
		logrus.Error(err)
		return nil , err
	}
	
	return u,nil
}

func (m* pqSqlUserRepository) GetProviderById(ctx context.Context , providerId int64) (domain.User , error){
	query := `SELECT * FROM users WHERE ProviderId = $1`
	row,err := m.Conn.QueryRowContext(ctx,query,providerId)

	if err != nil {
		logrus.Error(err)
		return nil , err
	}

	defer func(){
		errRow := row.Close()
		if errRow != nil{
			logrus.Error(errRow)
		}
	}

	u = domain.User{}
	err = row.Scan(
		&u.ID,
		&u.ProviderId,
		&u.Provider,
		&u.Username,
		&u.Email,
		&u.AvatarURL,
		&u.CreatedAt,
		&u.RefreshToken,
	)

	if err != nil {
		logrus.Error(err)
		return nil , err
	}

	return u , nil
}

func (m *pqSqlUserRepository) Store(ctx context.Context , user *domain.User) (error){
	query := `INSERT INTO users (ProviderId , Provider , Username , Email , AvatarURL , CreatedAt) VALUES ($1, $2, $3, $4, $5 , $6 ) RETURNING ID`

	err := m.Conn.QueryRowContext(ctx,query,user.ProviderId,user.Provider, user.Username, user.Email, user.AvatarURL, user.CreatedAt).Scan(&user.ID)
	
	if err != nil {
		return err
	}
	
	return nil
}

func (m* pgSqlUserRepository) Update(ctx context.Context , id int64 , refreshToken string) error {
	query := `UPDATE users SET AccessToken = $1 , RefreshToken = $2 WHERE ID = $3`
	
	stmt,err := m.Conn.PrepareContext(ctx,query)

	if err != nil {
		logrus.Error(err)
		return nil
	}

	res,err := stmt.ExecContext(ctx,refreshToken,id)
	if err != nil {
		logrus.Error(err)
		return nil
	}

	rowsAffected , err := res.RowsAffected()
	if err != nil {
		logrus.Error(err)
		return nil
	}

	if rowsAffected != 1 {
		return fmt.Errorf("weird Behavior. Total Affected: %d", rowsAffected)
	}

	return nil
}