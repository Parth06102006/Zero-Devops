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
	"strings"
	"sync"
	"time"

	appmiddleware "Zero_Devops/server/internal/middleware"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

const jwtExpiryMinutes = 10

type deploymentUsecase struct {
	deploymentRepo   domain.DeploymentRepository
	githubRepo       domain.GithubRepository
	tokenProvider    domain.InstallationTokenProvider
	projectRepo      domain.ProjectRepository
	repositoryClient domain.GithubRepositoryClient
	rmqConn          *amqp.Connection
	publishCh        *amqp.Channel
	pubMutex         sync.Mutex
}

type deploymentStatusUpdate struct {
	DeploymentID string `json:"deployment_id"`
	Status       string `json:"status"`
	OutputURL    string `json:"output_url"`
	ErrorMessage string `json:"error_message"`
}

// NewDeploymentUsecase creates a new deployment use case.
func NewDeploymentUsecase(deploymentRepo domain.DeploymentRepository, githubRepo domain.GithubRepository, tokenProvider domain.InstallationTokenProvider, rmqConn *amqp.Connection, dependencies ...interface{}) domain.DeploymentUsecase {
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
		tokenProvider:  tokenProvider,
		rmqConn:        rmqConn,
		publishCh:      publishCh,
	}
	for _, dependency := range dependencies {
		switch value := dependency.(type) {
		case domain.ProjectRepository:
			uc.projectRepo = value
		case domain.GithubRepositoryClient:
			uc.repositoryClient = value
		}
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

	token, err := d.tokenProvider.CreateInstallationToken(ctx, installation.InstallationID)
	if err != nil {
		log.Error("Failed to create installation token", zap.Error(err), zap.Int64("installation_id", installation.InstallationID))
		return nil, err
	}

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

func (d *deploymentUsecase) CreateProjectBuild(ctx context.Context, userID string, params domain.CreateProjectBuildParams) (*domain.Deployment, error) {
	log := appmiddleware.LoggerFromContext(ctx)
	projectID := strings.TrimSpace(params.ProjectID)
	shaOrRef := strings.TrimSpace(params.ShaOrRef)
	if projectID == "" || shaOrRef == "" || strings.TrimSpace(params.IdempotencyKey) == "" {
		return nil, domain.ErrBadParamInput
	}

	idempotencyKey, err := uuid.Parse(strings.TrimSpace(params.IdempotencyKey))
	if err != nil {
		return nil, domain.ErrBadParamInput
	}
	if d.projectRepo == nil || d.repositoryClient == nil || d.tokenProvider == nil {
		return nil, fmt.Errorf("project build dependencies are unavailable")
	}

	project, err := d.projectRepo.GetByID(ctx, userID, projectID)
	if err != nil {
		log.Error("failed to get project for manual build", zap.Error(err), zap.String("project_id", projectID), zap.String("user_id", userID))
		return nil, err
	}

	installation, err := d.githubRepo.GetInstallationByUserID(ctx, userID)
	if err != nil {
		log.Error("failed to get github installation for manual build", zap.Error(err), zap.String("user_id", userID))
		return nil, err
	}
	if installation.Status != domain.GithubInstallationStatusActive {
		return nil, domain.ErrInvalidStatus
	}
	if project.InstallationID != installation.ID {
		return nil, domain.ErrNotFound
	}

	token, err := d.tokenProvider.CreateInstallationToken(ctx, installation.InstallationID)
	if err != nil {
		log.Error("failed to create installation token for manual build", zap.Error(err), zap.Int64("installation_id", installation.InstallationID))
		return nil, err
	}

	repoDetails, err := d.repositoryClient.GetRepositoryDetails(ctx, token, project.GitHubRepositoryID)
	if err != nil {
		log.Error("failed to refresh repository details for manual build", zap.Error(err), zap.Int64("repository_id", project.GitHubRepositoryID))
		return nil, err
	}
	commitSHA, err := d.repositoryClient.ResolveCommit(ctx, token, project.RepositoryOwner, project.RepositoryName, shaOrRef)
	if err != nil {
		log.Error("failed to resolve manual build ref", zap.Error(err), zap.String("sha_or_ref", shaOrRef), zap.String("project_id", projectID))
		return nil, err
	}

	config := project.BuildConfiguration
	if config.ScannerPolicyVersion == "" {
		config.ScannerPolicyVersion = project.CommandPolicyVersion
	}
	if config.ScannerPolicyVersion == "" {
		return nil, domain.ErrBadParamInput
	}

	now := time.Now()
	deployment := &domain.Deployment{
		UserID:                    userID,
		RepoID:                    project.GitHubRepositoryID,
		CloneURL:                  repoDetails.CloneURL,
		Status:                    domain.DeploymentStatusPending,
		ProjectID:                 project.ID,
		GithubInstallationID:      installation.ID,
		CommitSHA:                 commitSHA,
		RequestedRef:              shaOrRef,
		Trigger:                   contract.TriggerManual,
		DesiredRevisionGeneration: project.DesiredRevisionGeneration,
		ConfigurationSnapshot:     config,
		ConfigurationVersion:      project.ConfigurationVersion,
		CommandPolicyVersion:      project.CommandPolicyVersion,
		CommandScanResult:         project.CommandScanResult,
		ManualIdempotencyKey:      idempotencyKey.String(),
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	if err := d.deploymentRepo.StoreProjectBuild(ctx, deployment); err != nil {
		log.Error("failed to store manual project build", zap.Error(err), zap.String("project_id", projectID))
		return nil, err
	}

	correlationID := strings.TrimSpace(params.CorrelationID)
	if correlationID == "" {
		correlationID = uuid.NewString()
	}
	job := contract.BuildRequestV1{
		Version:        contract.VersionV1,
		EventID:        uuid.NewString(),
		DeploymentID:   deployment.ID,
		ProjectID:      project.ID,
		InstallationID: installation.InstallationID,
		RepositoryID:   project.GitHubRepositoryID,
		CloneURL:       repoDetails.CloneURL,
		CommitSHA:      commitSHA,
		RequestedRef:   shaOrRef,
		Trigger:        contract.TriggerManual,
		Generation:     project.DesiredRevisionGeneration,
		RetryCount:     0,
		CorrelationID:  correlationID,
		Configuration: contract.Configuration{
			Executable:           config.Executable,
			Args:                 config.Args,
			WorkingDir:           config.WorkingDir,
			ScannerPolicyVersion: config.ScannerPolicyVersion,
		},
	}
	if err := d.publishBuildRequestV1(job); err != nil {
		log.Error("failed to publish manual project build", zap.Error(err), zap.String("deployment_id", deployment.ID))
		return nil, err
	}

	return deployment, nil
}

func (d *deploymentUsecase) GetDeployments(ctx context.Context, userID string) ([]domain.Deployment, error) {
	return d.deploymentRepo.GetByUserID(ctx, userID)
}

func (d *deploymentUsecase) GetDeploymentByID(ctx context.Context, userID, deploymentID string) (*domain.Deployment, error) {
	return d.deploymentRepo.GetByID(ctx, userID, deploymentID)
}

func (d *deploymentUsecase) ListProjectBuilds(ctx context.Context, userID, projectID string) ([]domain.Deployment, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, domain.ErrBadParamInput
	}
	return d.deploymentRepo.GetByProjectID(ctx, userID, projectID)
}

func (d *deploymentUsecase) GetBuild(ctx context.Context, userID, buildID string) (*domain.Deployment, error) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return nil, domain.ErrBadParamInput
	}
	return d.deploymentRepo.GetByID(ctx, userID, buildID)
}
