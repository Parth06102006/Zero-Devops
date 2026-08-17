// Package usecase contains deployment business logic
package usecase

import (
	"Zero_Devops/server/internal/deployments/contract"
	"Zero_Devops/server/internal/domain"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	appmiddleware "Zero_Devops/server/internal/middleware"
	
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

const jwtExpiryMinutes = 10

type deploymentUsecase struct {
	deploymentRepo domain.DeploymentRepository
	githubRepo     domain.GithubRepository
	tokenProvider  domain.InstallationTokenProvider
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
func NewDeploymentUsecase(deploymentRepo domain.DeploymentRepository, githubRepo domain.GithubRepository, tokenProvider domain.InstallationTokenProvider ,rmqConn *amqp.Connection) domain.DeploymentUsecase {
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

type githubRepoResponse struct {
	CloneURL string `json:"clone_url"`
}

// publishBuildRequestV1 is the only deploy.jobs producer. Callers must provide
// every immutable input; legacy repo-only deployment requests are intentionally
// rejected by the HTTP handler rather than being converted to this contract.
func (d *deploymentUsecase) publishBuildRequestV1(job contract.BuildRequestV1) error {
	publishing, err := contract.Publishing(job)
	if err != nil {
		return fmt.Errorf("invalid deploy.jobs V1 request: %w", err)
	}
	if d.publishCh == nil {
		return fmt.Errorf("deploy.jobs publisher is unavailable")
	}

	d.pubMutex.Lock()
	defer d.pubMutex.Unlock()
	return d.publishCh.Publish("", "deploy.jobs", false, false, publishing)
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

	//nolint:gosec // path comes from trusted server config, not user input

	// I have added the token provider instllation token

	token , err := d.tokenProvider.CreateInstallationToken(ctx,installation.InstallationID)
	

	repoURL := fmt.Sprintf("https://api.github.com/repositories/%d", repoID)
	repoReq, err := http.NewRequestWithContext(ctx, http.MethodGet, repoURL, http.NoBody)
	if err != nil {
		log.Error("Failed to create repo request", zap.Error(err))
		return nil, err
	}
	repoReq.Header.Set("Authorization", "Bearer "+token)
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

	// This path is currently unreachable: POST /deploy fails closed because it
	// lacks the required V1 immutable build inputs. Keep no legacy publisher.

	log.Info("Deployment created successfully", zap.String("deployment_id", deployment.ID))
	return deployment, nil
}

func (d *deploymentUsecase) GetDeployments(ctx context.Context, userID string) ([]domain.Deployment, error) {
	return d.deploymentRepo.GetByUserID(ctx, userID)
}

func (d *deploymentUsecase) GetDeploymentByID(ctx context.Context, userID, deploymentID string) (*domain.Deployment, error) {
	return d.deploymentRepo.GetByID(ctx, userID, deploymentID)
}
