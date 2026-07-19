package usecase

import (
	"Zero_Devops/server/domain"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	appmiddleware "Zero_Devops/server/middleware"

	"github.com/golang-jwt/jwt/v5"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type deployJob struct {
	DeploymentID  int64  `json:"deployment_id"`
	CloneURL      string `json:"clone_url"`
	CallbackQueue string `json:"callback_queue"`
	RetryCount    int    `json:"retry_count"`
	RequestID	  string  `json:"request_id"`
}

type deploymentUsecase struct {
	deploymentRepo domain.DeploymentRepository
	githubRepo     domain.GithubRepository
	rmqConn        *amqp.Connection
	publishCh      *amqp.Channel
	pubMutex       sync.Mutex
}

type deploymentStatusUpdate struct {
	DeploymentID int64  `json:"deployment_id"`
	Status       string `json:"status"`
}

func NewDeploymentUsecase(deploymentRepo domain.DeploymentRepository, githubRepo domain.GithubRepository, rmqConn *amqp.Connection) domain.DeploymentUsecase {
	var publishCh *amqp.Channel
	var err error
	if rmqConn != nil {
		publishCh, err = rmqConn.Channel()
		if err != nil {
			zap.L().Fatal("failed to open publish channel", zap.Error(err))
		}
	}

	uc := &deploymentUsecase{
		deploymentRepo: deploymentRepo,
		githubRepo:     githubRepo,
		rmqConn:        rmqConn,
		publishCh:      publishCh,
	}

	if rmqConn != nil {
		go func() {
			if err := uc.consumeStatusUpdate(); err != nil {
				zap.L().Error("deployment status consumer stopped", zap.Error(err))
			}
		}()
	}

	return uc
}

type installationTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

type githubRepoResponse struct {
	CloneURL string `json:"clone_url"`
}

func (d *deploymentUsecase) publishJob(deploymentID int64, cloneURL string, requestID string) error {

	
	job := deployJob{
		DeploymentID:  deploymentID,
		CloneURL:      cloneURL,
		CallbackQueue: "deploy.status",
		RetryCount:    0,
		RequestID: requestID,
	}
	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal deploy job: %w", err)
	}

	d.pubMutex.Lock()
	defer d.pubMutex.Unlock()

	return d.publishCh.Publish(
		"",
		"deploy.jobs",
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
}

func (d *deploymentUsecase) consumeStatusUpdate() error {
	// Open a dedicated channel for the consumer goroutine
	consumerCh, err := d.rmqConn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open consumer channel: %w", err)
	}
	defer consumerCh.Close()

	msgs, err := consumerCh.Consume(
		"deploy.status",
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to consume deployment status queue: %w", err)
	}

	for msg := range msgs {
		var update deploymentStatusUpdate
		if err := json.Unmarshal(msg.Body, &update); err != nil {
			zap.L().Error("failed to decode deployment status update", zap.Error(err))
			_ = msg.Nack(false, false)
			continue
		}

		status, ok := normalizeDeploymentStatus(update.Status)
		if !ok {
			zap.L().Error("invalid deployment status update", zap.String("status", update.Status), zap.Int64("deployment_id", update.DeploymentID))
			_ = msg.Nack(false, false)
			continue
		}

		if err := d.deploymentRepo.UpdateStatus(context.Background(), update.DeploymentID, status); err != nil {
			zap.L().Error("failed to update deployment status", zap.Int64("deployment_id", update.DeploymentID), zap.String("status", string(status)), zap.Error(err))
			_ = msg.Nack(false, true)
			continue
		}

		if err := msg.Ack(false); err != nil {
			zap.L().Error("failed to ack deployment status update", zap.Int64("deployment_id", update.DeploymentID), zap.Error(err))
		}
	}

	return nil
}

func normalizeDeploymentStatus(status string) (domain.DeploymentStatus, bool) {
	switch status {
	case "queued", string(domain.DeploymentStatusPending):
		return domain.DeploymentStatusPending, true
	case "building", string(domain.DeploymentStatusRunning):
		return domain.DeploymentStatusRunning, true
	case "done", string(domain.DeploymentStatusSuccess):
		return domain.DeploymentStatusSuccess, true
	case string(domain.DeploymentStatusFailed):
		return domain.DeploymentStatusFailed, true
	case string(domain.DeploymentStatusCanceled):
		return domain.DeploymentStatusCanceled, true
	default:
		return "", false
	}
}

func (d *deploymentUsecase) CreateDeployment(ctx context.Context, userID int64, repoID int64, requestID string) (*domain.Deployment, error) {
	log := appmiddleware.LoggerFromContext(ctx)
	log.Info("Starting deployment creation", zap.Int64("user_id", userID), zap.Int64("repo_id", repoID))

	installation, err := d.githubRepo.GetInstallationByUserID(ctx, userID)
	if err != nil {
		log.Error("Failed to get github installation", zap.Error(err))
		return nil, fmt.Errorf("failed to get github installation: %w", err)
	}

	privateKeyPath := viper.GetString("GITHUB_APP_PRIVATE_KEY_PATH")
	if privateKeyPath == "" {
		return nil, fmt.Errorf("GITHUB_APP_PRIVATE_KEY_PATH not configured")
	}

	pemBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(pemBytes)
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
		log.Error("Failed to get installation token", zap.Error(err))
		return nil, fmt.Errorf("failed to get installation token: %w", err)
	}

	cloneURL, err := d.getRepoCloneURL(ctx, instToken, repoID)
	if err != nil {
		log.Error("Failed to get repo info", zap.Error(err))
		return nil, fmt.Errorf("failed to get repo info: %w", err)
	}

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
		log.Error("Failed to store deployment", zap.Error(err))
		return nil, err
	}

	if err := d.publishJob(deployment.ID, cloneURL, requestID); err != nil {
		log.Error("Failed to publish deploy job", zap.Error(err))
		return deployment, fmt.Errorf("failed to publish deploy job: %w", err)
	}

	log.Info("Successfully triggered deployment job", zap.Int64("deployment_id", deployment.ID))
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
