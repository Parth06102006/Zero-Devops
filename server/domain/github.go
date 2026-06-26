package domain

import (
	"context"
	"time"
	"net/http"
)

type GithubInstallation struct {
	ID				int64		`json:"id"`
	UserID			int64		`json:"user_id"`
	InstallationID	int64		`json:"installation_id"`
	Account_Type	string		`json:"account_type"`
	Account_Login	string		`json:"account_login"`
	CreatedAt    time.Time `json:"created_at"`		
	UpdatedAt 	time.Time	`json:"updated_at"`	
}

type GithubUsecase interface {
	InstallGithubApp(ctx context.Context, client *http.Client,code string,user_id int64) (error)
	DeleteGithubApp(ctx context.Context, userID int64) error
	GetGithubAppInstallation(ctx context.Context, userID int64) (*GithubInstallation, error)
}

type GithubRepository interface {
	StoreInstallation(ctx context.Context , inst *GithubInstallation) error
	GetInstallationByUserID(ctx context.Context , userID int64) (*GithubInstallation, error)
	DeleteInstallationByUserID(ctx context.Context, userID int64) error
}