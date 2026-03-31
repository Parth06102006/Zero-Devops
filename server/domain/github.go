package domain

import "context"

type GithubInstalltion struct {
	ID				int64		`json:"id"`
	UserID			User		`json:"user_id"`
	InstalltionID	int64		`json:"installtion_id"`
	AccountName		string		`json:"acount_name"`	
}

type GithubRepositry interface {
	StoreInstalltion(ctx context.Context , inst *GithubInstallation) error
	GetInstallationByUserID(ctx context.Context , userID int64) (*GithubInstallation, error)
}