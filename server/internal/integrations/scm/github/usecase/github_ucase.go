// Package usecase contains GitHub integration business logic
package usecase

import (
	"Zero_Devops/server/internal/domain"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	appmiddleware "Zero_Devops/server/internal/middleware"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type githubAppUsecase struct {
	githubRepo domain.GithubRepository
}

// NewGithubAppUsecase creates a new GithubUsecase
func NewGithubAppUsecase(githubRepo domain.GithubRepository) domain.GithubUsecase {
	return &githubAppUsecase{
		githubRepo: githubRepo,
	}
}

// GithubTokenResponse represents an OAuth token response from GitHub
type GithubTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

// Installation represents a GitHub App installation
type Installation struct {
	ID      int64 `json:"id"`
	Account struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	}
	AppID int64 `json:"app_id"`
}

// GithubInstallationList represents a list of GitHub App installations
type GithubInstallationList struct {
	TotalCount    int            `json:"total_count"`
	Installations []Installation `json:"installations"`
}

func (g *githubAppUsecase) InstallGithubApp(ctx context.Context, client *http.Client, code, userID string) error {
	log := appmiddleware.LoggerFromContext(ctx)
	log.Info("Starting GitHub App installation", zap.String("user_id", userID))

	githubAppClientID := viper.GetString("GITHUB_APP_CLIENT_ID")
	githubAppClientSecret := viper.GetString("GITHUB_APP_CLIENT_SECRET")
	githubAppID := viper.GetInt64("GITHUB_APP_ID")

	data := url.Values{}
	data.Add("client_id", githubAppClientID)
	data.Add("client_secret", githubAppClientSecret)
	data.Add("code", code)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))

	if err != nil {
		return domain.ErrInvalidCode
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	response, err := client.Do(req)
	if err != nil {
		log.Error("Failed to perform access token request", zap.Error(err))
		return err
	}

	defer func() {
		if err := response.Body.Close(); err != nil {
			log.Error("failed to close response body", zap.Error(err))
		}
	}()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return domain.ErrInvalidCode
	}

	var githubTokenResponse GithubTokenResponse
	if err := json.NewDecoder(response.Body).Decode(&githubTokenResponse); err != nil {
		return err
	}

	reqInstallation, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/installations", http.NoBody)

	if err != nil {
		return err
	}

	reqInstallation.Header.Set("Authorization", "Bearer "+githubTokenResponse.AccessToken)
	reqInstallation.Header.Set("Accept", "application/vnd.github+json")

	responseInstallation, err := client.Do(reqInstallation)
	if err != nil {
		log.Error("Failed to perform installations fetch request", zap.Error(err))
		return err
	}

	defer func() {
		if err := responseInstallation.Body.Close(); err != nil {
			log.Error("failed to close installation response body", zap.Error(err))
		}
	}()

	if responseInstallation.StatusCode < 200 || responseInstallation.StatusCode >= 300 {
		return domain.ErrGithubInstallationFetchFailed
	}

	var githubAppInstallationList GithubInstallationList
	if err := json.NewDecoder(responseInstallation.Body).Decode(&githubAppInstallationList); err != nil {
		return err
	}

	for _, inst := range githubAppInstallationList.Installations {
		if inst.Account.Type == "User" && inst.AppID == githubAppID {
			githubAppInstallation := domain.GithubInstallation{
				UserID:         userID,
				InstallationID: inst.ID,
				AccountType:    inst.Account.Type,
				AccountLogin:   inst.Account.Login,
				Status:         domain.GithubInstallationStatusActive,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			}
			err := g.githubRepo.StoreInstallation(ctx, &githubAppInstallation)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *githubAppUsecase) GetGithubAppInstallation(ctx context.Context, userID string) (*domain.GithubInstallation, error) {
	githubRepo, err := g.githubRepo.GetInstallationByUserID(ctx, userID)

	if err != nil {
		return nil, err
	}

	return githubRepo, nil
}

func (g *githubAppUsecase) DeleteGithubApp(ctx context.Context, userID string) error {
	return g.githubRepo.DeleteInstallationByUserID(ctx, userID)
}
