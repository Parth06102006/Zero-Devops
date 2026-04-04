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
	query := `SELECT * FROM User WHERE ID = $1`
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
	)
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	return u, nil
}

func (m *pqSqlUserRepository) GetByUsername (ctx context.Context , username string) (domain.User , error){
	query := `SELECT * FROM User WHERE Username = $1`
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
	)
	
	if err != nil {
		logrus.Error(err)
		return nil , err
	}
	
	return u,nil
}

func (m *pqSqlUserRepository) Store(ctx context.Context , user *domain.User) (error){
	query := `INSERT INTO User (Github , Username , Email , AvatarURL , CreatedAt) VALUES ($1, $2, $3, $4, $5)`
	stmt , err = m.Conn.PrepareContext(ctx,query)
	
	if err != nil{
		logrus.Error(err)
		return err
	}
	
	res, err := stmt.ExecContext(ctx, user.ProviderId,user.Provider, user.Username, user.Email, user.AvatarURL, user.CreatedAt)
	if err != nil {
		logrus.Error(err)
		return err
	}
	
	user.ID = res.LastInsertId()
	return nil
}
