// Package usecase contains deployment business logic
package usecase

import (
	"Zero_Devops/server/internal/domain"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	appmiddleware "Zero_Devops/server/internal/middleware"

	"github.com/golang-jwt/jwt/v5"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const jwtExpiryMinutes = 10

type deployJob struct {
	DeploymentID  string `json:"deployment_id"`
	CloneURL      string `json:"clone_url"`
	CallbackQueue string `json:"callback_queue"`
	RetryCount    int    `json:"retry_count"`
	RequestID     string `json:"request_id"`
	BuildType     string `json:"build_type"`
}

type deploymentUsecase struct {
	deploymentRepo domain.DeploymentRepository
	githubRepo     domain.GithubRepository
	rmqConn        *amqp.Connection
	publishCh      *amqp.Channel
	pubMutex       sync.Mutex
}

type deploymentStatusUpdate struct {
	DeploymentID string `json:"deployment_id"`
	Status       string `json:"status"`
	OutputURL    string `json:"output_url"`
	ErrorMessage string `json:"error_message"`
}

// NewDeploymentUsecase creates a new deployment use case
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

func (d *deploymentUsecase) publishJob(deploymentID, cloneURL, requestID string) error {
	job := deployJob{
		DeploymentID:  deploymentID,
		CloneURL:      cloneURL,
		CallbackQueue: "deploy.status",
		RetryCount:    0,
		RequestID:     requestID,
		BuildType:     "buildpacks",
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
			Body:         body,
			DeliveryMode: amqp.Persistent,
		},
	)
}

func (d *deploymentUsecase) consumeStatusUpdate() error {
	consumerCh, err := d.rmqConn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open consumer channel: %w", err)
	}
	defer func() {
		if err := consumerCh.Close(); err != nil {
			zap.L().Error("failed to close consumer channel", zap.Error(err))
		}
	}()

	msgs, err := consumerCh.Consume(
		"deploy.status",
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	for msg := range msgs {
		var update deploymentStatusUpdate
		if err := json.Unmarshal(msg.Body, &update); err != nil {
			zap.L().Error("failed to unmarshal status update", zap.Error(err))
			continue
		}
		ctx := context.Background()
		if err := d.deploymentRepo.UpdateStatus(ctx, update.DeploymentID, domain.DeploymentStatus(update.Status)); err != nil {
			zap.L().Error("failed to update deployment status", zap.Error(err))
		}
		if update.OutputURL != "" {
			if err := d.deploymentRepo.UpdateOutputURL(ctx, update.DeploymentID, update.OutputURL); err != nil {
				zap.L().Error("failed to update deployment output URL", zap.Error(err))
			}
		}
		if update.ErrorMessage != "" {
			if err := d.deploymentRepo.UpdateErrorMessage(ctx, update.DeploymentID, update.ErrorMessage); err != nil {
				zap.L().Error("failed to update deployment error message", zap.Error(err))
			}
		}
	}

	return nil
}

//nolint:funlen
func (d *deploymentUsecase) CreateDeployment(ctx context.Context, userID string, repoID int64, requestID string) (*domain.Deployment, error) {
	log := appmiddleware.LoggerFromContext(ctx)
	log.Info("Starting deployment creation", zap.String("user_id", userID), zap.Int64("repo_id", repoID))

	installation, err := d.githubRepo.GetInstallationByUserID(ctx, userID)
	if err != nil {
		log.Error("Failed to get github installation", zap.Error(err))
		return nil, err
	}

	appID := viper.GetInt64("GITHUB_APP_ID")
	privateKeyPath := viper.GetString("GITHUB_APP_PRIVATE_KEY_PATH")

	//nolint:gosec // path comes from trusted server config, not user input
	privateKeyPEM, err := os.ReadFile(privateKeyPath)
	if err != nil {
		log.Error("Failed to read GitHub App private key", zap.Error(err))
		return nil, err
	}

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		log.Error("Failed to parse GitHub App private key", zap.Error(err))
		return nil, err
	}

	now := time.Now()
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iat": now.Unix(),
		"exp": now.Add(jwtExpiryMinutes * time.Minute).Unix(),
		"iss": appID,
	})

	signedJWT, err := jwtToken.SignedString(privateKey)
	if err != nil {
		log.Error("Failed to sign JWT", zap.Error(err))
		return nil, err
	}

	tokenURL := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installation.InstallationID)
	tokenReq, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, http.NoBody)
	if err != nil {
		log.Error("Failed to create token request", zap.Error(err))
		return nil, err
	}
	tokenReq.Header.Set("Authorization", "Bearer "+signedJWT)
	tokenReq.Header.Set("Accept", "application/vnd.github+json")

	tokenResp, err := http.DefaultClient.Do(tokenReq)
	if err != nil {
		log.Error("Failed to get installation token", zap.Error(err))
		return nil, err
	}
	defer func() {
		if err := tokenResp.Body.Close(); err != nil {
			log.Error("failed to close token response body", zap.Error(err))
		}
	}()

	if tokenResp.StatusCode != http.StatusCreated {
		log.Error("Unexpected status from GitHub token API", zap.Int("status", tokenResp.StatusCode))
		return nil, fmt.Errorf("github token API returned status %d", tokenResp.StatusCode)
	}

	var tokenData installationTokenResponse
	if err := json.NewDecoder(tokenResp.Body).Decode(&tokenData); err != nil {
		log.Error("Failed to decode token response", zap.Error(err))
		return nil, err
	}

	repoURL := fmt.Sprintf("https://api.github.com/repositories/%d", repoID)
	repoReq, err := http.NewRequestWithContext(ctx, http.MethodGet, repoURL, http.NoBody)
	if err != nil {
		log.Error("Failed to create repo request", zap.Error(err))
		return nil, err
	}
	repoReq.Header.Set("Authorization", "Bearer "+tokenData.Token)
	repoReq.Header.Set("Accept", "application/vnd.github+json")

	repoResp, err := http.DefaultClient.Do(repoReq)
	if err != nil {
		log.Error("Failed to get repo info", zap.Error(err))
		return nil, err
	}
	defer func() {
		if err := repoResp.Body.Close(); err != nil {
			log.Error("failed to close repo response body", zap.Error(err))
		}
	}()

	if repoResp.StatusCode != http.StatusOK {
		log.Error("Unexpected status from GitHub repo API", zap.Int("status", repoResp.StatusCode))
		return nil, fmt.Errorf("github repo API returned status %d", repoResp.StatusCode)
	}

	body, err := io.ReadAll(repoResp.Body)
	if err != nil {
		log.Error("Failed to read repo response", zap.Error(err))
		return nil, err
	}

	var repoData githubRepoResponse
	if err := json.Unmarshal(body, &repoData); err != nil {
		log.Error("Failed to decode repo response", zap.Error(err))
		return nil, err
	}

	deployment := &domain.Deployment{
		UserID:    userID,
		RepoID:    repoID,
		CloneURL:  repoData.CloneURL,
		Status:    domain.DeploymentStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := d.deploymentRepo.Store(ctx, deployment); err != nil {
		log.Error("Failed to store deployment", zap.Error(err))
		return nil, err
	}

	if err := d.publishJob(deployment.ID, repoData.CloneURL, requestID); err != nil {
		log.Error("Failed to publish deployment job", zap.Error(err))
		return nil, err
	}

	log.Info("Deployment created successfully", zap.String("deployment_id", deployment.ID))
	return deployment, nil
}

func (d *deploymentUsecase) GetDeployments(ctx context.Context, userID string) ([]domain.Deployment, error) {
	return d.deploymentRepo.GetByUserID(ctx, userID)
}

func (d *deploymentUsecase) GetDeploymentByID(ctx context.Context, userID, deploymentID string) (*domain.Deployment, error) {
	return d.deploymentRepo.GetByID(ctx, userID, deploymentID)
}
