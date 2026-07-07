package usecase

import (
	"Zero_Devops/server/domain"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
)

type deploymentUsecase struct {
	deploymentRepo domain.DeploymentRepository
	githubRepo     domain.GithubRepository
}

func NewDeploymentUsecase(deploymentRepo domain.DeploymentRepository, githubRepo domain.GithubRepository) domain.DeploymentUsecase {
	return &deploymentUsecase{
		deploymentRepo: deploymentRepo,
		githubRepo:     githubRepo,
	}
}

type installationTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

type githubRepoResponse struct {
	CloneURL string `json:"clone_url"`
}

func (d *deploymentUsecase) CreateDeployment(ctx context.Context, userID int64, repoID int64) (*domain.Deployment, error) {
	installation, err := d.githubRepo.GetInstallationByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get github installation: %w", err)
	}

	privateKeyPEM := viper.GetString("GITHUB_APP_PRIVATE_KEY")
	if privateKeyPEM == "" {
		return nil, fmt.Errorf("GITHUB_APP_PRIVATE_KEY not configured")
	}

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(privateKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	appID := viper.GetString("GITHUB_APP_ID")

	now := time.Now()
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iat": now.Unix(),
		"exp": now.Add(10 * time.Minute).Unix(),
		"iss": appID,
	})

	jwtSigned, err := jwtToken.SignedString(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign jwt: %w", err)
	}

	instToken, err := d.getInstallationToken(ctx, jwtSigned, installation.InstallationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get installation token: %w", err)
	}

	cloneURL, err := d.getRepoCloneURL(ctx, instToken, repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo info: %w", err)
	}
//job creation and ading to the queue logic to be added 
	deployment := &domain.Deployment{
		UserID:    userID,
		RepoID:    repoID,
		CloneURL:  cloneURL,
		Status:    domain.DeploymentStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = d.deploymentRepo.Store(ctx, deployment)
	if err != nil {
		return nil, err
	}

	return deployment, nil
}

func (d *deploymentUsecase) getInstallationToken(ctx context.Context, jwtToken string, installationID int64) (string, error) {
	url := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("github API returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp installationTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	return tokenResp.Token, nil
}

func (d *deploymentUsecase) getRepoCloneURL(ctx context.Context, token string, repoID int64) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repositories/%d", repoID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("github API returned %d: %s", resp.StatusCode, string(body))
	}

	var repoResp githubRepoResponse
	if err := json.NewDecoder(resp.Body).Decode(&repoResp); err != nil {
		return "", err
	}

	return repoResp.CloneURL, nil
}

func (d *deploymentUsecase) GetDeployments(ctx context.Context, userID int64) ([]domain.Deployment, error) {
	return d.deploymentRepo.GetByUserID(ctx, userID)
}

func (d *deploymentUsecase) GetDeploymentByID(ctx context.Context, userID, deploymentID int64) (*domain.Deployment, error) {
	return d.deploymentRepo.GetByID(ctx, userID, deploymentID)
}
