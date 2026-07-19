package domain

import (
	"context"
	"net/http"
	"time"
)

const (
	GithubInstallationStatusActive      = "active"
	GithubInstallationStatusSuspended   = "suspended"
	GithubInstallationStatusUninstalled = "uninstalled"
)

type GithubInstallation struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	InstallationID int64     `json:"installation_id"`
	AccountType   string    `json:"account_type"`
	AccountLogin  string    `json:"account_login"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}





type GithubUsecase interface {
	InstallGithubApp(ctx context.Context, client *http.Client, code string, userID int64) error
	DeleteGithubApp(ctx context.Context, userID int64) error
	GetGithubAppInstallation(ctx context.Context, userID int64) (*GithubInstallation, error)
}

type GithubRepository interface {
	StoreInstallation(ctx context.Context, inst *GithubInstallation) error
	GetInstallationByUserID(ctx context.Context, userID int64) (*GithubInstallation, error)
	DeleteInstallationByUserID(ctx context.Context, userID int64) error
	UpdateInstallationStatus(ctx context.Context, userID int64, status string) error
}
