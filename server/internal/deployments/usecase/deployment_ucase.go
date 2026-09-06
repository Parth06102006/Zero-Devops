// Package usecase contains deployment business logic.
//
// ─────────────────────────────────────────────────────────────────────────────
// DEAD / LEGACY CODE — REMOVED 2026-09-06
// ─────────────────────────────────────────────────────────────────────────────
// The legacy chain (jwtExpiryMinutes, githubRepoResponse, CreateDeployment,
// the POST /deploy route, and the repo methods Store/StoreProjectBuild) was
// deleted on 2026-09-06 after the durable status consumer landed.
//
// The live paths are:
//
//	build:  CreateProjectBuild → StoreProjectBuildWithOutbox → the outbox
//	        dispatcher in internal/deployments/dispatcher publishes deploy.jobs.
//	status: durable consumer (consumeStatusUpdate) listens on deploy.status,
//	        applies ApplyStatusUpdate atomically, acks only after commit.
//
// ─────────────────────────────────────────────────────────────────────────────
package usecase

import (
	"Zero_Devops/server/internal/deployments/contract"
	"Zero_Devops/server/internal/domain"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	appmiddleware "Zero_Devops/server/internal/middleware"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// jwtExpiryMinutes was deleted 2026-09-06 (dead code); token TTLs are owned
// by the token provider, which has its own copy of the constant.

type deploymentUsecase struct {
	deploymentRepo   domain.DeploymentRepository
	githubRepo       domain.GithubRepository
	tokenProvider    domain.InstallationTokenProvider
	projectRepo      domain.ProjectRepository
	repositoryClient domain.GithubRepositoryClient

	// rmqConn feeds the durable deploy.status consumer (see
	// consumeStatusUpdate). It is no longer used for publishing; deploy.jobs
	// is published exclusively by the outbox dispatcher.
	rmqConn *amqp.Connection
}

// deploymentStatusUpdate is the wire format of the worker's deploy.status
// message. Decoded by the durable status consumer and applied via
// ApplyStatusUpdate.
type deploymentStatusUpdate struct {
	DeploymentID string `json:"deployment_id"`
	Status       string `json:"status"`
	OutputURL    string `json:"output_url"`
	ErrorMessage string `json:"error_message"`
}

// NewDeploymentUsecase creates a new deployment use case. ctx governs the
// background deploy.status consumer's lifetime: it stops accepting messages
// and cancels in-flight database work when ctx is cancelled (the server's
// signal context). Pass context.Background() when no lifecycle control is
// needed (tests).
func NewDeploymentUsecase(ctx context.Context, deploymentRepo domain.DeploymentRepository, githubRepo domain.GithubRepository, tokenProvider domain.InstallationTokenProvider, rmqConn *amqp.Connection, dependencies ...interface{}) domain.DeploymentUsecase {
	uc := &deploymentUsecase{
		deploymentRepo: deploymentRepo,
		githubRepo:     githubRepo,
		tokenProvider:  tokenProvider,
		rmqConn:        rmqConn,
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
		// Durable deploy.status consumer (Task 5, plan-server-12-08.md):
		// applies worker status updates atomically via ApplyStatusUpdate and
		// acknowledges only after the database commit. The restart wrapper
		// keeps the consumer alive across channel/connection failures and
		// stops cleanly on ctx cancellation.
		go uc.runStatusConsumerLoop(ctx)
	}

	return uc
}

// githubRepoResponse was deleted 2026-09-06 (dead code, used only by the
// removed CreateDeployment); superseded by domain.GithubRepositoryClient.

// statusAcknowledger is the broker-acknowledgement seam of an amqp.Delivery
// (Ack/Nack). Extracted as an interface so the consumer's message handling is
// unit-testable without a live broker; amqp.Delivery satisfies it.
type statusAcknowledger interface {
	Ack(multiple bool) error
	Nack(multiple, requeue bool) error
}

const (
	// statusConsumePrefetch bounds unacknowledged in-flight status messages.
	// With autoAck=false and no Qos the broker would push unlimited
	// unacknowledged deliveries into this process.
	statusConsumePrefetch = 32

	// statusMaxApplyAttempts bounds in-process retries for transient
	// ApplyStatusUpdate failures (database blips). Exhausted attempts
	// dead-letter the message instead of requeueing: RabbitMQ redelivers
	// requeued messages immediately, which would hot-loop against a
	// struggling database.
	statusMaxApplyAttempts = 3

	// statusRestartBackoff bounds the consumer restart loop after an
	// unexpected channel/connection loss.
	statusRestartBackoffStart = time.Second
	statusRestartBackoffMax   = 30 * time.Second
	statusSessionResetAfter   = time.Minute
)

// statusApplyBackoff sleeps between transient-failure retry attempts. A
// package var so tests can shorten it.
var statusApplyBackoff = []time.Duration{200 * time.Millisecond, 400 * time.Millisecond}

// runStatusConsumerLoop keeps a durable deploy.status consumer alive for the
// lifetime of ctx. Each iteration runs one consume session; on unexpected
// session end (channel or connection loss, broker restart) it reconnects
// with capped exponential backoff. A session that ran for a while resets the
// backoff so one old failure does not leave the consumer permanently slow.
func (d *deploymentUsecase) runStatusConsumerLoop(ctx context.Context) {
	backoff := statusRestartBackoffStart
	for {
		if ctx.Err() != nil {
			return
		}

		started := time.Now()
		err := d.consumeStatusUpdate(ctx)
		if ctx.Err() != nil {
			return // planned shutdown
		}

		if time.Since(started) >= statusSessionResetAfter {
			backoff = statusRestartBackoffStart
		}

		zap.L().Error("deploy.status consumer session ended; restarting",
			zap.Error(err), zap.Duration("restart_in", backoff))

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > statusRestartBackoffMax {
			backoff = statusRestartBackoffMax
		}
	}
}

// consumeStatusUpdate runs one durable consume session on deploy.status.
// It returns nil on planned shutdown (ctx cancelled) and an error when the
// session ended unexpectedly, so runStatusConsumerLoop can restart it.
func (d *deploymentUsecase) consumeStatusUpdate(ctx context.Context) error {
	if d.rmqConn == nil {
		return errors.New("status consumer requires a RabbitMQ connection")
	}

	consumerCh, err := d.rmqConn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open consumer channel: %w", err)
	}

	// Close the channel exactly once, either from the ctx watcher (shutdown)
	// or after the delivery loop drains. Closing the channel is what ends the
	// `for range msgs` loop, so the watcher guarantees prompt shutdown even
	// while no messages are arriving.
	var closeOnce sync.Once
	closeChannel := func() {
		closeOnce.Do(func() { _ = consumerCh.Close() })
	}
	watcherDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			closeChannel()
		case <-watcherDone:
		}
	}()
	defer close(watcherDone)
	defer closeChannel()

	// Ack must happen after the durable commit, so autoAck is false; prefetch
	// bounds the unacknowledged in-flight window.
	if err := consumerCh.Qos(statusConsumePrefetch, 0, false); err != nil {
		return fmt.Errorf("failed to set status consumer prefetch: %w", err)
	}

	msgs, err := consumerCh.Consume(
		"deploy.status",
		"deploy-status-consumer",
		false, // autoAck: acknowledge only after ApplyStatusUpdate commits
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register status consumer: %w", err)
	}

	for msg := range msgs {
		d.handleStatusMessage(ctx, msg, msg.Body)
	}

	if ctx.Err() != nil {
		return nil // planned shutdown; the channel was closed by the watcher
	}
	return errors.New("deploy.status delivery channel closed unexpectedly")
}

// handleStatusMessage applies one deploy.status message durably and only then
// acknowledges it. Failure handling follows the classification documented in
// plan-server-12-08.md (Task 5):
//
//   - malformed message (bad JSON, empty ID, unknown status): poison — retry
//     can never fix it; Nack(requeue=false) dead-letters to deploy.status.dlq.
//   - ErrNotFound / ErrInvalidStatus(Transition) / ErrBadParamInput from
//     ApplyStatusUpdate: permanent — the same rejection every time;
//     dead-letter immediately.
//   - any other error (database down, deadlock, timeout): transient — retry
//     up to statusMaxApplyAttempts in-process. Still failing after that is
//     also dead-lettered (never requeued: requeue redelivers immediately and
//     hot-loops). The deployment row stays in its current non-terminal state;
//     stuck-deployment reconciliation and/or DLQ replay recover it.
//
// A crash between the durable commit and the Ack redelivers the message;
// ApplyStatusUpdate is idempotent (terminal states are immutable, same-status
// duplicates are no-ops), so redelivery converges instead of corrupting state.
func (d *deploymentUsecase) handleStatusMessage(ctx context.Context, ack statusAcknowledger, body []byte) {
	log := zap.L()

	var update deploymentStatusUpdate
	err := json.Unmarshal(body, &update)
	if err == nil && (update.DeploymentID == "" || !isValidWorkerStatus(domain.DeploymentStatus(update.Status))) {
		err = fmt.Errorf("invalid deployment_id %q or status %q", update.DeploymentID, update.Status)
	}
	if err != nil {
		log.Warn("dead-lettering malformed deploy.status message (poison)", zap.Error(err))
		if nackErr := ack.Nack(false, false); nackErr != nil {
			log.Error("failed to nack malformed deploy.status message", zap.Error(nackErr))
		}
		return
	}

	params := domain.ApplyStatusParams{
		DeploymentID: update.DeploymentID,
		Status:       domain.DeploymentStatus(update.Status),
		OutputURL:    update.OutputURL,
		ErrorMessage: update.ErrorMessage,
	}

	var result *domain.ApplyStatusResult
	var applyErr error
	for attempt := 1; attempt <= statusMaxApplyAttempts; attempt++ {
		result, applyErr = d.deploymentRepo.ApplyStatusUpdate(ctx, params)
		if applyErr == nil || isPermanentStatusError(applyErr) {
			break
		}
		if attempt == statusMaxApplyAttempts {
			break
		}
		wait := statusApplyBackoff[attempt-1]
		select {
		case <-ctx.Done():
			// Shutdown mid-retry: neither ack nor nack — the broker
			// redelivers the unacknowledged message and the idempotent
			// apply converges after restart.
			return
		case <-time.After(wait):
		}
	}

	switch {
	case applyErr == nil:
		if result != nil && !result.IsCurrentGeneration {
			// Applied durably, but a newer accepted push superseded this
			// generation. Not an error: the deployment row keeps its true
			// outcome and currency is derived at read time; log for
			// observability of out-of-order pushes.
			log.Warn("status applied for superseded generation",
				zap.String("deployment_id", update.DeploymentID),
				zap.String("status", update.Status))
		}
		if ackErr := ack.Ack(false); ackErr != nil {
			// The commit happened but the ack failed (e.g. the channel
			// dropped): the broker redelivers and the idempotent apply runs
			// again. Log and continue.
			log.Error("deploy.status ack failed after durable commit",
				zap.String("deployment_id", update.DeploymentID),
				zap.Error(ackErr))
		}
	case isPermanentStatusError(applyErr):
		log.Warn("dead-lettering unapplicable deploy.status update",
			zap.String("deployment_id", update.DeploymentID),
			zap.String("status", update.Status),
			zap.Error(applyErr))
		if nackErr := ack.Nack(false, false); nackErr != nil {
			log.Error("failed to nack unapplicable deploy.status message", zap.Error(nackErr))
		}
	default:
		log.Error("dead-lettering deploy.status update after exhausted retries",
			zap.String("deployment_id", update.DeploymentID),
			zap.String("status", update.Status),
			zap.Error(applyErr))
		if nackErr := ack.Nack(false, false); nackErr != nil {
			log.Error("failed to nack exhausted deploy.status message", zap.Error(nackErr))
		}
	}
}

// isValidWorkerStatus reports whether the status string is one the worker may
// report on deploy.status.
func isValidWorkerStatus(status domain.DeploymentStatus) bool {
	switch status {
	case domain.DeploymentStatusPending,
		domain.DeploymentStatusBuilding,
		domain.DeploymentStatusSuccess,
		domain.DeploymentStatusFailed,
		domain.DeploymentStatusCanceled:
		return true
	default:
		return false
	}
}

// isPermanentStatusError reports whether an ApplyStatusUpdate failure can
// never succeed on retry (unknown deployment, illegal transition, bad input).
func isPermanentStatusError(err error) bool {
	return errors.Is(err, domain.ErrNotFound) ||
		errors.Is(err, domain.ErrInvalidStatus) ||
		errors.Is(err, domain.ErrInvalidStatusTransition) ||
		errors.Is(err, domain.ErrBadParamInput)
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
	correlationID := strings.TrimSpace(params.CorrelationID)
	if correlationID == "" {
		correlationID = uuid.NewString()
	}

	// Transactional outbox (Task 5, plan-server-12-08.md): the deployments row
	// and its deploy.jobs V1 outbox event are committed in one transaction by
	// StoreProjectBuildWithOutbox. The outbox dispatcher is the only deploy.jobs
	// producer; there is no synchronous publish here, so a crash after the DB
	// commit can no longer lose an accepted build.
	deployment, err = d.deploymentRepo.StoreProjectBuildWithOutbox(ctx, domain.StoreProjectBuildWithOutboxParams{
		Deployment:                deployment,
		EventID:                   uuid.NewString(),
		InstallationID:            installation.InstallationID,
		CorrelationID:             correlationID,
		DesiredRevisionGeneration: project.DesiredRevisionGeneration,
	})
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			// Duplicate (project, manual idempotency key): the build already
			// exists and its outbox event is already recorded or sent.
			log.Info("manual project build already exists for idempotency key",
				zap.String("project_id", projectID),
				zap.String("idempotency_key", idempotencyKey.String()))
			return nil, domain.ErrConflict
		}
		log.Error("failed to store manual project build with outbox event", zap.Error(err), zap.String("project_id", projectID))
		return nil, err
	}

	log.Info("manual project build created with outbox event",
		zap.String("project_id", projectID),
		zap.String("deployment_id", deployment.ID),
		zap.String("commit_sha", commitSHA),
		zap.String("trigger", contract.TriggerManual))

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
