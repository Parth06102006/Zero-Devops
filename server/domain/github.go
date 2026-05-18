package domain

import "context"

type GithubInstallation struct {
	ID				int64		`json:"id"`
	UserID			int64		`json:"user_id"`
	InstallationID	int64		`json:"installation_id"`
	AccountName		string		`json:"account_name"`	
}

type GithubUsecase interface {
	InstallGithubApp(ctx context.Context) (error)
	DeleteGithubApp(ctx context.Context) error
	GetGithubAppInstallation(ctx context.Context, userID int64) (*GithubInstallation, error)
}

type GithubRepository interface {
	StoreInstallation(ctx context.Context , inst *GithubInstallation) error
	GetInstallationByUserID(ctx context.Context , userID int64) (*GithubInstallation, error)
	DeleteInstallationByUserID(ctx context.Context, userID int64) error
}