// Package github provides GitHub webhook handling
package github

import (
	"Zero_Devops/server/internal/domain"
	"Zero_Devops/server/internal/integrations/scm/github/cache"
	appmiddleware "Zero_Devops/server/internal/middleware"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// allowedGitHubEvents are the GitHub webhook event types this handler accepts.
var allowedGitHubEvents = []domain.Event{
	domain.InstallationEvent,
	domain.InstallationRepositoriesEvent,
	domain.PushEventP,
	domain.CreateEvent,
	domain.DeleteEvent,
	domain.ReleaseEvent,
	domain.RepositoryEvent,
}

type webhookUsecase struct {
	secret         string
	maxPayloadSize int64
	webhookRepo    domain.WebhookRepository
	githubRepo     domain.GithubRepository
	projectRepo    domain.ProjectRepository
	deploymentRepo domain.DeploymentRepository
	cache          cache.RepositoryListCache
}

// Option is a configuration option for the webhook
type Option func(*webhookUsecase) error

// WebhookOptions is a namespace for configuration option methods
type WebhookOptions struct{}

// Options is a namespace var for configuration options
var Options = WebhookOptions{}

// Secret configures the GitHub webhook shared secret.
func (WebhookOptions) Secret(secret string) Option {
	return func(webH *webhookUsecase) error {
		if secret == "" {
			return domain.ErrMissingSecret
		}

		webH.secret = secret
		return nil
	}
}

// MaxPayloadSize configures the largest accepted webhook body.
func (WebhookOptions) MaxPayloadSize(maxPayloadSize int64) Option {
	return func(webH *webhookUsecase) error {
		if maxPayloadSize <= 0 {
			return domain.ErrInvalidPayloadSize
		}
		webH.maxPayloadSize = maxPayloadSize
		return nil
	}
}

// NewWebhookUsecase creates a GitHub webhook use case.
func NewWebhookUsecase(
	webhookRepo domain.WebhookRepository,
	githubRepo domain.GithubRepository,
	projectRepo domain.ProjectRepository,
	deploymentRepo domain.DeploymentRepository,
	repositoryListCache *cache.RedisRepositoryListCache,
	options ...Option,
) (domain.WebhookUsecase, error) {
	hook := new(webhookUsecase)
	for _, opt := range options {
		if err := opt(hook); err != nil {
			return nil, err
		}
	}

	hook.webhookRepo = webhookRepo
	hook.githubRepo = githubRepo
	hook.projectRepo = projectRepo
	hook.deploymentRepo = deploymentRepo
	hook.cache = repositoryListCache

	if hook.secret == "" {
		return nil, domain.ErrMissingSecret
	}
	if hook.maxPayloadSize <= 0 {
		return nil, domain.ErrInvalidPayloadSize
	}

	return hook, nil
}

func (webH *webhookUsecase) handleInstallationEvent(ctx context.Context, payload domain.InstallationPayload) error {
	if webH.githubRepo == nil {
		return domain.ErrInternalServerError
	}

	installationID := payload.Installation.ID
	if installationID <= 0 {
		return domain.ErrBadParamInput
	}

	var status string
	switch payload.Action {
	case "created", "unsuspend", "new_permissions_accepted", "reinstalled":
		status = domain.GithubInstallationStatusActive
	case "suspend":
		status = domain.GithubInstallationStatusSuspended
	case "deleted":
		status = domain.GithubInstallationStatusUninstalled
	default:
		return nil
	}

	if payload.Action == "created" || payload.Action == "reinstalled" {
		githubInstallationDBID, err := webH.githubRepo.GetInstallationIDByGithubInstallationID(ctx, installationID)
		if err == nil && githubInstallationDBID != "" {
			if err := webH.githubRepo.UpdateInstallationExternalIDByID(ctx, strconv.FormatInt(installationID, 10), githubInstallationDBID); err != nil {
				return err
			}
		} else if err != nil && err != domain.ErrNotFound {
			return err
		}
	}

	err := webH.githubRepo.UpdateInstallationStatusByGithubInstallationID(ctx, installationID, status)
	if err == domain.ErrNotFound {
		return nil
	}
	return err
}

// handleInstallationRepositoriesEvent synchronizes selected project availability
// after a GitHub installation repository change.
func (webH *webhookUsecase) handleInstallationRepositoriesEvent(
	ctx context.Context,
	githubInstallationDBID string,
	payload domain.InstallationRepositoriesPayload,
) error {
	if webH.projectRepo == nil || webH.cache == nil {
		return domain.ErrInternalServerError
	}

	projectRepositoryAvailability, err := webH.projectRepo.GetProjectRepoAvailability(ctx, githubInstallationDBID)
	if err != nil {
		return err
	}

	for _, repository := range payload.RepositoriesAdded {
		if _, selected := projectRepositoryAvailability[repository.ID]; selected {
			projectRepositoryAvailability[repository.ID] = true
		}
	}

	for _, repository := range payload.RepositoriesRemoved {
		if _, selected := projectRepositoryAvailability[repository.ID]; selected {
			projectRepositoryAvailability[repository.ID] = false
		}
	}

	if err := webH.projectRepo.UpdateProjectRepoAvailability(ctx, githubInstallationDBID, projectRepositoryAvailability); err != nil {
		return err
	}

	return webH.cache.InvalidateInstallation(ctx, payload.Installation.ID)
}

// zeroSHA is GitHub's all-zero Git object ID, used for the `before`/`after`
// fields when a ref is newly created or deleted.
const zeroSHA = "0000000000000000000000000000000000000000"

// isZeroSHA reports whether sha is the all-zero object ID (or missing), which
// for `after` means a deleted branch push.
func isZeroSHA(sha string) bool {
	return strings.TrimSpace(sha) == "" || strings.EqualFold(strings.TrimSpace(sha), zeroSHA)
}

// handlePushEvent decides whether a push event creates a build and creates it.
// Eligibility: not a deleted branch (all-zero after), installation resolves to
// a local row and is active, the repository is a selected project, the project
// has webhook builds enabled and its repository is still available on the
// installation, and the pushed ref exactly matches the project's persisted
// configured branch. An eligible push then advances the project's desired-
// revision generation and durably records the deployment plus its deploy.jobs
// V1 outbox event in one repository transaction; a later dispatcher publishes
// the outbox row to RabbitMQ.
func (webH *webhookUsecase) handlePushEvent(
	ctx context.Context,
	githubInstallationDBID, githubDeliveryID, webhookDeliveryDBID string,
	payload domain.PushPayload,
) error {
	log := appmiddleware.LoggerFromContext(ctx)

	// A deleted branch (after is all zeros) never creates a build. A force push
	// is not a deletion: it carries a real after SHA and builds exactly that SHA.
	if payload.Deleted || isZeroSHA(payload.After) {
		log.Info("push event skipped: branch deleted",
			zap.String("github_delivery_id", githubDeliveryID),
			zap.String("webhook_delivery_db_id", webhookDeliveryDBID),
			zap.String("ref", payload.Ref),
			zap.String("commit_sha", payload.After),
		)
		return nil
	}

	// The push's installation does not resolve to a local row (e.g. it arrived
	// before the install flow stored one). Nothing to validate against; the
	// delivery is still accepted.
	if githubInstallationDBID == "" {
		log.Info("push event skipped: installation not resolved to a local row",
			zap.String("github_delivery_id", githubDeliveryID),
			zap.String("webhook_delivery_db_id", webhookDeliveryDBID),
			zap.String("ref", payload.Ref),
			zap.Int64("github_repository_id", payload.Repository.ID),
			zap.Int64("github_installation_external_id", int64(payload.Installation.ID)),
		)
		return nil
	}

	if webH.projectRepo == nil || webH.deploymentRepo == nil {
		return domain.ErrInternalServerError
	}

	// Global installation gate: GitHub only delivers webhooks for an active
	// installation. A suspended/uninstalled installation blocks webhook builds
	// for every project of the user, independent of per-project settings.
	installationStatus, err := webH.githubRepo.GetInstallationStatusByID(ctx, githubInstallationDBID)
	if err == domain.ErrNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	if installationStatus != domain.GithubInstallationStatusActive {
		log.Info("push event skipped: installation not active",
			zap.String("github_delivery_id", githubDeliveryID),
			zap.String("webhook_delivery_db_id", webhookDeliveryDBID),
			zap.String("installation_db_id", githubInstallationDBID),
			zap.String("installation_status", installationStatus),
			zap.Int64("github_repository_id", payload.Repository.ID),
		)
		return nil
	}

	project, err := webH.projectRepo.GetByInstallationAndRepositoryID(ctx, githubInstallationDBID, payload.Repository.ID)
	if err == domain.ErrNotFound {
		// The pushed repository is not a selected project: no build, not an error.
		log.Info("push event skipped: repository not a selected project",
			zap.String("github_delivery_id", githubDeliveryID),
			zap.String("webhook_delivery_db_id", webhookDeliveryDBID),
			zap.String("installation_db_id", githubInstallationDBID),
			zap.Int64("github_repository_id", payload.Repository.ID),
		)
		return nil
	}
	if err != nil {
		return err
	}

	// Webhook builds require the repository to still be available on the
	// installation and the project to have webhook builds enabled.
	if !project.RepositoryAvailable || !project.ProjectWebhookEnabled {
		log.Info("push event skipped: project not available or webhook builds disabled",
			zap.String("github_delivery_id", githubDeliveryID),
			zap.String("webhook_delivery_db_id", webhookDeliveryDBID),
			zap.String("project_id", project.ID),
			zap.Bool("repository_available", project.RepositoryAvailable),
			zap.Bool("webhook_enabled", project.ProjectWebhookEnabled),
		)
		return nil
	}

	// Only pushes whose ref exactly matches the persisted configured branch
	// (stored as a full ref, e.g. refs/heads/main) create a build. The webhook
	// never trusts a client-supplied branch.
	if payload.Ref != project.ConfiguredBranch {
		log.Info("push event skipped: ref does not match configured branch",
			zap.String("github_delivery_id", githubDeliveryID),
			zap.String("webhook_delivery_db_id", webhookDeliveryDBID),
			zap.String("project_id", project.ID),
			zap.String("ref", payload.Ref),
			zap.String("configured_branch", project.ConfiguredBranch),
		)
		return nil
	}

	// The desired-revision generation is advanced by
	// StoreWebhookBuildWithOutbox inside the same transaction as the build
	// insert, so an older queued/running result cannot become the project's
	// current result after this revision is accepted — and a redelivered
	// delivery cannot bump the counter without creating a build.
	deployment, err := webH.deploymentRepo.StoreWebhookBuildWithOutbox(ctx, domain.StoreWebhookBuildParams{
		ProjectID:             project.ID,
		UserID:                project.UserID,
		RepoID:                payload.Repository.ID,
		CloneURL:              payload.Repository.CloneURL,
		GithubInstallationID:  githubInstallationDBID,
		WebhookDeliveryID:     webhookDeliveryDBID,
		CommitSHA:             payload.After,
		RequestedRef:          payload.Ref,
		ConfigurationSnapshot: project.BuildConfiguration,
		ConfigurationVersion:  project.ConfigurationVersion,
		CommandPolicyVersion:  project.CommandPolicyVersion,
		CommandScanResult:     project.CommandScanResult,
		EventID:               uuid.NewString(),
		InstallationID:        int64(payload.Installation.ID),
		CorrelationID:         uuid.NewString(),
	})
	if err == domain.ErrConflict {
		// The unique (webhook_delivery_id) index already recorded a build for
		// this delivery (concurrent redelivery): not an error, build exists.
		log.Info("push event redelivery: build already exists for delivery",
			zap.String("github_delivery_id", githubDeliveryID),
			zap.String("webhook_delivery_db_id", webhookDeliveryDBID),
			zap.String("project_id", project.ID),
		)
		return nil
	}
	if err != nil {
		return err
	}

	log.Info("webhook push created build",
		zap.String("github_delivery_id", githubDeliveryID),
		zap.String("webhook_delivery_db_id", webhookDeliveryDBID),
		zap.String("deployment_id", deployment.ID),
		zap.Int64("github_repository_id", payload.Repository.ID),
		zap.String("commit_sha", payload.After),
		zap.Int64("desired_revision_generation", deployment.DesiredRevisionGeneration),
		zap.Int64("build_number", deployment.BuildNumber),
	)
	return nil
}

// Parse verifies and parses the events specified and returns the payload object or an error
func (webH *webhookUsecase) HandleGithubWebhook(ctx context.Context, r *http.Request) (interface{}, error) {
	defer func() {
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
	}()

	deliveryID := r.Header.Get("X-Github-Delivery")

	if deliveryID == "" {
		return nil, domain.ErrMissingGithubDeliveryHeader
	}

	eventName := r.Header.Get("X-GitHub-Event")

	gitHubEvent, err := webH.checkEvents(eventName, allowedGitHubEvents...)

	if err != nil {
		return nil, err
	}

	payload, err := webH.verifyAndCheck(r)
	if err != nil {
		return nil, err
	}

	webhookDelivery := domain.WebhookDelivery{}

	webhookDelivery.DeliveryID = deliveryID
	webhookDelivery.EventName = eventName
	webhookDelivery.ProcessingStatus = domain.ProcessingStatusReceived
	webhookDelivery.ReceivedAt = time.Now()

	inserted, deliveryDBID, err := webH.webhookRepo.TryInsertDelivery(ctx, webhookDelivery)

	if err != nil {
		webH.markDeliveryFailed(ctx, deliveryID, err)
		return nil, err
	}

	if !inserted {
		return nil, nil
	}

	parsed, err := webH.parsePayload(gitHubEvent, payload)
	if err != nil {
		webH.markDeliveryFailed(ctx, deliveryID, err)
		return nil, err
	}

	webH.populateDelivery(&webhookDelivery, parsed)

	if webhookDelivery.GitHubInstallationExternalID > 0 && webH.githubRepo != nil {
		githubInstallationDBID, err := webH.githubRepo.GetInstallationIDByGithubInstallationID(ctx, webhookDelivery.GitHubInstallationExternalID)
		if err == nil {
			webhookDelivery.GitHubInstallationDBID = githubInstallationDBID
		} else if err != domain.ErrNotFound {
			webH.markDeliveryFailed(ctx, deliveryID, err)
			return nil, err
		}
	}

	err = webH.webhookRepo.UpdateDeliveryMetadata(ctx, deliveryID, webhookDelivery)

	if err != nil {
		webH.markDeliveryFailed(ctx, deliveryID, err)
		return nil, err
	}

	if err := webH.eventHandler(ctx, gitHubEvent, webhookDelivery.GitHubInstallationDBID, deliveryID, deliveryDBID, parsed); err != nil {
		webH.markDeliveryFailed(ctx, deliveryID, err)
		return nil, err
	}

	err = webH.webhookRepo.UpdateDeliveryStatus(ctx, deliveryID, domain.ProcessingStatusAccepted, nil)

	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (webH *webhookUsecase) markDeliveryFailed(ctx context.Context, deliveryID string, processingErr error) {
	if err := webH.webhookRepo.UpdateDeliveryStatus(ctx, deliveryID, domain.ProcessingStatusFailed, P(processingErr.Error())); err != nil {
		appmiddleware.LoggerFromContext(ctx).Error("failed to mark webhook delivery as failed", zap.Error(err), zap.String("delivery_id", deliveryID))
	}
}

func (webH *webhookUsecase) checkEvents(event string, events ...domain.Event) (domain.Event, error) {
	if len(events) == 0 {
		return "", domain.ErrEventNotSpecifiedToParse
	}

	if event == "" {
		return "", domain.ErrMissingGithubEventHeader
	}
	gitHubEvent := domain.Event(event)

	found := slices.Contains(events, gitHubEvent)

	if !found {
		return "", domain.ErrEventNotFound
	}

	return gitHubEvent, nil
}

// verifyAndCheck validates the HTTP request, event allow-list, payload size, and
// HMAC signature. It returns the verified event type and the raw payload.
func (webH *webhookUsecase) verifyAndCheck(r *http.Request) ([]byte, error) {
	if r.Method != http.MethodPost {
		return nil, domain.ErrInvalidHTTPMethod
	}

	payload, err := io.ReadAll(io.LimitReader(r.Body, webH.maxPayloadSize+1))
	if err != nil || len(payload) == 0 {
		return nil, domain.ErrParsingPayload
	}

	if webH.maxPayloadSize > 0 && int64(len(payload)) > webH.maxPayloadSize {
		return nil, domain.ErrPayloadTooLarge
	}

	if webH.secret != "" {
		signature := r.Header.Get("X-Hub-Signature-256")
		if signature == "" {
			return nil, domain.ErrMissingHubSignatureHeader
		}

		signature = strings.TrimPrefix(signature, "sha256=")

		mac := hmac.New(sha256.New, []byte(webH.secret))
		_, _ = mac.Write(payload)
		expectedMAC := hex.EncodeToString(mac.Sum(nil))

		if !hmac.Equal([]byte(signature), []byte(expectedMAC)) {
			return nil, domain.ErrHMACVerificationFailed
		}
	}

	return payload, nil
}

// parsePayload unmarshals the verified payload into the typed struct for its event.
func (webH *webhookUsecase) parsePayload(gitHubEvent domain.Event, payload []byte) (interface{}, error) {
	switch gitHubEvent {
	case domain.InstallationEvent:
		var pl domain.InstallationPayload
		err := json.Unmarshal(payload, &pl)
		return pl, err
	case domain.InstallationRepositoriesEvent:
		var pl domain.InstallationRepositoriesPayload
		err := json.Unmarshal(payload, &pl)
		return pl, err
	case domain.PushEventP:
		var pl domain.PushPayload
		err := json.Unmarshal(payload, &pl)
		return pl, err
	case domain.CreateEvent:
		var pl domain.CreatePayload
		err := json.Unmarshal(payload, &pl)
		return pl, err
	case domain.DeleteEvent:
		var pl domain.DeletePayload
		err := json.Unmarshal(payload, &pl)
		return pl, err
	case domain.ReleaseEvent:
		var pl domain.ReleasePayload
		err := json.Unmarshal(payload, &pl)
		return pl, err
	case domain.RepositoryEvent:
		var pl domain.RepositoryPayload
		err := json.Unmarshal(payload, &pl)
		return pl, err

	default:
		return nil, fmt.Errorf("unknown event %s", gitHubEvent)
	}
}

// populateDelivery fills the event-specific nullable fields on a WebhookDelivery
// from the already-parsed payload.
func (webH *webhookUsecase) populateDelivery(d *domain.WebhookDelivery, parsed interface{}) {
	switch p := parsed.(type) {
	case domain.InstallationPayload:
		d.GitHubInstallationExternalID = p.Installation.ID
		d.EventAction = p.Action
	case domain.InstallationRepositoriesPayload:
		d.GitHubInstallationExternalID = p.Installation.ID
		d.EventAction = p.Action
	case domain.PushPayload:
		d.GitHubInstallationExternalID = int64(p.Installation.ID)
		d.GitHubRepositoryID = p.Repository.ID
	case domain.CreatePayload:
		d.GitHubRepositoryID = p.Repository.ID
	case domain.DeletePayload:
		d.GitHubRepositoryID = p.Repository.ID
	case domain.ReleasePayload:
		d.GitHubRepositoryID = p.Repository.ID
		d.EventAction = p.Action
	case domain.RepositoryPayload:
		d.GitHubRepositoryID = p.Repository.ID
		d.EventAction = p.Action
	}
}

// eventHandler routes parsed payloads to their event handlers.
func (webH *webhookUsecase) eventHandler(
	ctx context.Context,
	gitHubEvent domain.Event,
	githubInstallationDBID, githubDeliveryID, webhookDeliveryDBID string,
	payload interface{},
) error {
	switch gitHubEvent {
	case domain.InstallationEvent:
		installationPayload, ok := payload.(domain.InstallationPayload)
		if !ok {
			return domain.ErrParsingPayload
		}
		return webH.handleInstallationEvent(ctx, installationPayload)
	case domain.InstallationRepositoriesEvent:
		installationRepositoriesPayload, ok := payload.(domain.InstallationRepositoriesPayload)

		if !ok {
			return domain.ErrParsingPayload
		}

		return webH.handleInstallationRepositoriesEvent(ctx, githubInstallationDBID, installationRepositoriesPayload)
	case domain.PushEventP:
		pushPayload, ok := payload.(domain.PushPayload)
		if !ok {
			return domain.ErrParsingPayload
		}
		return webH.handlePushEvent(ctx, githubInstallationDBID, githubDeliveryID, webhookDeliveryDBID, pushPayload)

	default:
		return nil
	}
}

// BasicType is a scalar type accepted by P.
type BasicType interface {
	~string | ~bool | ~int | ~int64
}

// P returns a pointer to a scalar value.
func P[T BasicType](t T) *T { return &t }
