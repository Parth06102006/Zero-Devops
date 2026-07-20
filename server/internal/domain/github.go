package domain

import (
	"context"
	"net/http"
	"time"
)

const (
	// GithubInstallationStatusActive indicates an active installation
	GithubInstallationStatusActive = "active"
	// GithubInstallationStatusSuspended indicates a suspended installation
	GithubInstallationStatusSuspended = "suspended"
	// GithubInstallationStatusUninstalled indicates an uninstalled installation
	GithubInstallationStatusUninstalled = "uninstalled"
)

// GithubInstallation represents a GitHub App installation record
type GithubInstallation struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	InstallationID int64     `json:"installation_id"`
	AccountType    string    `json:"account_type"`
	AccountLogin   string    `json:"account_login"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// GithubUsecase defines the interface for GitHub integration use cases
type GithubUsecase interface {
	InstallGithubApp(ctx context.Context, client *http.Client, code string, userID string) error
	DeleteGithubApp(ctx context.Context, userID string) error
	GetGithubAppInstallation(ctx context.Context, userID string) (*GithubInstallation, error)
}

// GithubRepository defines the interface for GitHub installation data operations
type GithubRepository interface {
	StoreInstallation(ctx context.Context, inst *GithubInstallation) error
	GetInstallationByUserID(ctx context.Context, userID string) (*GithubInstallation, error)
	DeleteInstallationByUserID(ctx context.Context, userID string) error
	UpdateInstallationStatus(ctx context.Context, userID string, status string) error
}
