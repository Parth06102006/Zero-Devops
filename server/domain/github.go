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
	Account_Type   string    `json:"account_type"`
	Account_Login  string    `json:"account_login"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
type PushEvent struct {
    Ref        string `json:"ref"`
    Before     string `json:"before"`
    After      string `json:"after"`
    Repository struct {
        FullName string `json:"full_name"`
        CloneURL string `json:"clone_url"`
    } `json:"repository"`
    Commits []Commit `json:"commits"`
}
type Commit struct {
    ID      string `json:"id"`
    Message string `json:"message"`
    Author  struct {
        Name  string `json:"name"`
        Email string `json:"email"`
    } `json:"author"`
}
type PullRequestEvent struct {
    Action      string `json:"action"`
    Number      int    `json:"number"`
    PullRequest struct {
        Title string `json:"title"`
        Body  string `json:"body"`
        State string `json:"state"`
        Head  struct {
            Ref  string `json:"ref"`
            Sha  string `json:"sha"`
        } `json:"head"`
        Base struct {
            Ref  string `json:"ref"`
            Sha  string `json:"sha"`
        } `json:"base"`
    } `json:"pull_request"`
    Repository struct {
        FullName string `json:"full_name"`
    } `json:"repository"`
}
type GithubUsecase interface {
	InstallGithubApp(ctx context.Context, client *http.Client, code string, user_id int64) error
	DeleteGithubApp(ctx context.Context, userID int64) error
	GetGithubAppInstallation(ctx context.Context, userID int64) (*GithubInstallation, error)
	HandleWebhook(ctx context.Context, eventType string, payload []byte) error
}

type GithubRepository interface {
	StoreInstallation(ctx context.Context, inst *GithubInstallation) error
	GetInstallationByUserID(ctx context.Context, userID int64) (*GithubInstallation, error)
	DeleteInstallationByUserID(ctx context.Context, userID int64) error
	UpdateInstallationStatus(ctx context.Context, userID int64, status string) error
}
