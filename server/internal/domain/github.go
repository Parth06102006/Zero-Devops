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
	ListRepositories(ctx context.Context, userID, cursor, query string, perPage int) (*RepositoryList, error)
	GetRepositoryDetails(ctx context.Context, userID string, repoID int64) (*RepositoryPicker, error)
	InvalidateRepositoryCache(ctx context.Context, installationID int64) error
}

// GithubRepository defines the interface for GitHub installation data operations
type GithubRepository interface {
	StoreInstallation(ctx context.Context, inst *GithubInstallation) error
	GetInstallationByUserID(ctx context.Context, userID string) (*GithubInstallation, error)
	GetInstallationIdByGithubInstallationID(ctx context.Context, installationID int64) (string, error)
	GetInstallationStatusByID(ctx context.Context, installationDBID string) (string, error)
	DeleteInstallationByUserID(ctx context.Context, userID string) error
	UpdateInstallationStatus(ctx context.Context, userID string, status string) error
	UpdateInstallationStatusByGithubInstallationID(ctx context.Context, installationID int64, status string) error
	UpdateInstallationExternalIDByID(ctx context.Context, installationID string, github_installation_db_id string) error
}

type InstallationTokenProvider interface {
	CreateInstallationToken(ctx context.Context, installationID int64) (string, error)
}

type RepositoryPicker struct {
	ID            int64  `json:"id"`
	Owner         string `json:"owner"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
	CloneURL      string `json:"clone_url"`
	Private       bool   `json:"private"`
}

type RepositoryList struct {
	Repositories []RepositoryPicker `json:"repositories"`
	NextCursor   string             `json:"next_cursor,omitempty"`
}

type GithubRepositoryClient interface {
	ListRepositories(ctx context.Context, installationToken string, cursor string, query string, perPage int) (*RepositoryList, error)
	GetRepositoryDetails(ctx context.Context, installationToken string, repoID int64) (*RepositoryPicker, error)
	ResolveCommit(ctx context.Context, installationToken, owner, repo, shaOrRef string) (string, error)
}
