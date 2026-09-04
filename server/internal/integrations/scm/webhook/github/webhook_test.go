package github

import (
	"Zero_Devops/server/internal/domain"
	"Zero_Devops/server/internal/integrations/scm/github/cache"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type fakeWebhookRepository struct {
	insertedDeliveries []domain.WebhookDelivery
	tryInsertResult    bool
	tryInsertDBID      string
	tryInsertErr       error
	metadata           *domain.WebhookDelivery
	statusUpdates      []domain.ProcessingStatus
}

func (f *fakeWebhookRepository) InsertDelivery(_ context.Context, d domain.WebhookDelivery) error {
	f.insertedDeliveries = append(f.insertedDeliveries, d)
	return nil
}

func (f *fakeWebhookRepository) TryInsertDelivery(_ context.Context, d domain.WebhookDelivery) (bool, string, error) {
	f.insertedDeliveries = append(f.insertedDeliveries, d)
	if f.tryInsertErr != nil {
		return false, "", f.tryInsertErr
	}
	if !f.tryInsertResult {
		return false, "", nil
	}
	if f.tryInsertDBID == "" {
		f.tryInsertDBID = "delivery-db-id"
	}
	return true, f.tryInsertDBID, nil
}

func (f *fakeWebhookRepository) UpdateDeliveryMetadata(_ context.Context, _ string, d domain.WebhookDelivery) error {
	cp := d
	f.metadata = &cp
	return nil
}

func (f *fakeWebhookRepository) UpdateDeliveryStatus(_ context.Context, _ string, status domain.ProcessingStatus, _ *string) error {
	f.statusUpdates = append(f.statusUpdates, status)
	return nil
}

func (f *fakeWebhookRepository) GetDeliveryId(_ context.Context, _ string) (*domain.WebhookDelivery, error) {
	return nil, domain.ErrNotFound
}

type fakeGithubRepository struct {
	dbIDByExternal map[int64]string
	statusByExt    map[int64]string
	statusByID     map[string]string
	updateErr      error
}

func (f *fakeGithubRepository) StoreInstallation(_ context.Context, _ *domain.GithubInstallation) error {
	return nil
}

func (f *fakeGithubRepository) GetInstallationByUserID(_ context.Context, _ string) (*domain.GithubInstallation, error) {
	return nil, domain.ErrNotFound
}

func (f *fakeGithubRepository) GetInstallationIdByGithubInstallationID(_ context.Context, installationID int64) (string, error) {
	if f.dbIDByExternal == nil {
		return "", domain.ErrNotFound
	}
	id, ok := f.dbIDByExternal[installationID]
	if !ok {
		return "", domain.ErrNotFound
	}
	return id, nil
}

func (f *fakeGithubRepository) DeleteInstallationByUserID(_ context.Context, _ string) error {
	return nil
}

func (f *fakeGithubRepository) UpdateInstallationStatus(_ context.Context, _, _ string) error {
	return nil
}

func (f *fakeGithubRepository) UpdateInstallationStatusByGithubInstallationID(_ context.Context, installationID int64, status string) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	if f.statusByExt == nil {
		f.statusByExt = map[int64]string{}
	}
	f.statusByExt[installationID] = status
	return nil
}

func (f *fakeGithubRepository) UpdateInstallationExternalIDByID(_ context.Context, _, _ string) error {
	return nil
}

func (f *fakeGithubRepository) GetInstallationStatusByID(_ context.Context, installationDBID string) (string, error) {
	if f.statusByID == nil {
		return "", domain.ErrNotFound
	}
	status, ok := f.statusByID[installationDBID]
	if !ok {
		return "", domain.ErrNotFound
	}
	return status, nil
}

type fakeProjectRepository struct {
	availability map[int64]bool
	updated      map[int64]bool
	project      *domain.Project
	projectErr   error
	lookups      []lookupArgs
	generations  int64
}

type lookupArgs struct {
	installationDBID string
	repositoryID     int64
}

func (f *fakeProjectRepository) Store(_ context.Context, _ *domain.Project) error { return nil }
func (f *fakeProjectRepository) ListByUserID(_ context.Context, _ string) ([]domain.Project, error) {
	return nil, nil
}
func (f *fakeProjectRepository) GetByID(_ context.Context, _, _ string) (*domain.Project, error) {
	return nil, nil
}
func (f *fakeProjectRepository) Update(_ context.Context, _ *domain.Project) error { return nil }
func (f *fakeProjectRepository) Delete(_ context.Context, _, _ string) error       { return nil }
func (f *fakeProjectRepository) GetProjectRepoAvailability(_ context.Context, _ string) (map[int64]bool, error) {
	return f.availability, nil
}
func (f *fakeProjectRepository) UpdateProjectRepoAvailability(_ context.Context, _ string, availability map[int64]bool) error {
	f.updated = availability
	return nil
}

func (f *fakeProjectRepository) GetByInstallationAndRepositoryID(_ context.Context, installationDBID string, repositoryID int64) (*domain.Project, error) {
	f.lookups = append(f.lookups, lookupArgs{installationDBID: installationDBID, repositoryID: repositoryID})
	if f.projectErr != nil {
		return nil, f.projectErr
	}
	if f.project == nil {
		return nil, domain.ErrNotFound
	}
	return f.project, nil
}
func (f *fakeProjectRepository) IncrementDesiredRevisionGeneration(_ context.Context, _ string, _ int64) (int64, error) {
	f.generations++
	return f.generations, nil
}

type fakeDeploymentRepository struct {
	stored     []domain.StoreWebhookBuildParams
	storeErr   error
	storedRepo *domain.Deployment
}

func (f *fakeDeploymentRepository) Store(_ context.Context, _ *domain.Deployment) error { return nil }
func (f *fakeDeploymentRepository) StoreProjectBuild(_ context.Context, _ *domain.Deployment) error {
	return nil
}
func (f *fakeDeploymentRepository) StoreWebhookBuildWithOutbox(_ context.Context, params domain.StoreWebhookBuildParams) (*domain.Deployment, error) {
	if f.storeErr != nil {
		return nil, f.storeErr
	}
	f.stored = append(f.stored, params)
	if f.storedRepo == nil {
		f.storedRepo = &domain.Deployment{ID: "deployment-db-id"}
	}
	return f.storedRepo, nil
}
func (f *fakeDeploymentRepository) GetByUserID(_ context.Context, _ string) ([]domain.Deployment, error) {
	return nil, nil
}
func (f *fakeDeploymentRepository) GetByID(_ context.Context, _, _ string) (*domain.Deployment, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeDeploymentRepository) GetByProjectID(_ context.Context, _, _ string) ([]domain.Deployment, error) {
	return nil, nil
}
func (f *fakeDeploymentRepository) UpdateStatus(_ context.Context, _ string, _ domain.DeploymentStatus) error {
	return nil
}
func (f *fakeDeploymentRepository) UpdateOutputURL(_ context.Context, _, _ string) error { return nil }
func (f *fakeDeploymentRepository) UpdateErrorMessage(_ context.Context, _, _ string) error {
	return nil
}

func newTestRepositoryListCache(t *testing.T) *cache.RedisRepositoryListCache {
	t.Helper()
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	return cache.NewRedisRepositoryListCache(redisClient, 0)
}

func signPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func newRequest(t *testing.T, event, deliveryID, secret, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", event)
	req.Header.Set("X-Github-Delivery", deliveryID)
	if secret != "" {
		req.Header.Set("X-Hub-Signature-256", signPayload([]byte(body), secret))
	}
	return req
}

func TestNewWebhookUsecase_RequiresSecret(t *testing.T) {
	_, err := NewWebhookUsecase(&fakeWebhookRepository{}, &fakeGithubRepository{}, nil, nil, nil,
		Options.MaxPayloadSize(1024))
	if !errors.Is(err, domain.ErrMissingSecret) {
		t.Fatalf("expected ErrMissingSecret, got %v", err)
	}
}

func TestHandleGithubWebhook_MissingDeliveryHeader(t *testing.T) {
	hook, _ := NewWebhookUsecase(&fakeWebhookRepository{}, &fakeGithubRepository{}, nil, nil, nil,
		Options.Secret("secret"), Options.MaxPayloadSize(1024))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader("{}"))
	req.Header.Set("X-GitHub-Event", "push")

	_, err := hook.HandleGithubWebhook(context.Background(), req)
	if !errors.Is(err, domain.ErrMissingGithubDeliveryHeader) {
		t.Fatalf("expected ErrMissingGithubDeliveryHeader, got %v", err)
	}
}

func TestHandleGithubWebhook_MissingSignature(t *testing.T) {
	hook, _ := NewWebhookUsecase(&fakeWebhookRepository{}, &fakeGithubRepository{}, nil, nil, nil,
		Options.Secret("secret"), Options.MaxPayloadSize(1024))

	req := newRequest(t, "push", "del-1", "", `{"ref":"refs/heads/main"}`)

	_, err := hook.HandleGithubWebhook(context.Background(), req)
	if !errors.Is(err, domain.ErrMissingHubSignatureHeader) {
		t.Fatalf("expected ErrMissingHubSignatureHeader, got %v", err)
	}
}

func TestHandleGithubWebhook_InvalidSignature(t *testing.T) {
	hook, _ := NewWebhookUsecase(&fakeWebhookRepository{}, &fakeGithubRepository{}, nil, nil, nil,
		Options.Secret("secret"), Options.MaxPayloadSize(1024))

	req := newRequest(t, "push", "del-1", "wrong", `{"ref":"refs/heads/main"}`)

	_, err := hook.HandleGithubWebhook(context.Background(), req)
	if !errors.Is(err, domain.ErrHMACVerificationFailed) {
		t.Fatalf("expected ErrHMACVerificationFailed, got %v", err)
	}
}

func TestHandleGithubWebhook_PushSuccess(t *testing.T) {
	repo := &fakeWebhookRepository{tryInsertResult: true}
	gh := &fakeGithubRepository{}
	hook, _ := NewWebhookUsecase(repo, gh, nil, nil, nil,
		Options.Secret("secret"), Options.MaxPayloadSize(1024))

	body := `{"ref":"refs/heads/main","repository":{"id":7,"full_name":"u/r"}}`
	req := newRequest(t, "push", "del-push", "secret", body)

	if _, err := hook.HandleGithubWebhook(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.metadata == nil || repo.metadata.GitHubRepositoryID != 7 {
		t.Fatalf("expected metadata github_repository_id 7, got %+v", repo.metadata)
	}
	if len(repo.statusUpdates) != 1 || repo.statusUpdates[0] != domain.ProcessingStatusAccepted {
		t.Fatalf("expected accepted status, got %v", repo.statusUpdates)
	}
}

func TestHandleGithubWebhook_InstallationSuspend(t *testing.T) {
	repo := &fakeWebhookRepository{tryInsertResult: true}
	gh := &fakeGithubRepository{dbIDByExternal: map[int64]string{42: "inst-uuid"}}
	hook, _ := NewWebhookUsecase(repo, gh, nil, nil, nil,
		Options.Secret("secret"), Options.MaxPayloadSize(1024))

	body := `{"action":"suspend","installation":{"id":42}}`
	req := newRequest(t, "installation", "del-inst", "secret", body)

	if _, err := hook.HandleGithubWebhook(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gh.statusByExt[42] != domain.GithubInstallationStatusSuspended {
		t.Fatalf("expected suspended status for installation 42, got %q", gh.statusByExt[42])
	}
	if repo.metadata == nil || repo.metadata.GitHubInstallationDBID != "inst-uuid" {
		t.Fatalf("expected GitHubInstallationDBID inst-uuid, got %+v", repo.metadata)
	}
}

func TestHandleGithubWebhook_InstallationReinstalledSetsActive(t *testing.T) {
	repo := &fakeWebhookRepository{tryInsertResult: true}
	gh := &fakeGithubRepository{dbIDByExternal: map[int64]string{99: "inst-uuid"}}
	hook, _ := NewWebhookUsecase(repo, gh, nil, nil, nil,
		Options.Secret("secret"), Options.MaxPayloadSize(1024))

	body := `{"action":"reinstalled","installation":{"id":99}}`
	req := newRequest(t, "installation", "del-reinst", "secret", body)

	if _, err := hook.HandleGithubWebhook(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gh.statusByExt[99] != domain.GithubInstallationStatusActive {
		t.Fatalf("expected active status for reinstalled installation 99, got %q", gh.statusByExt[99])
	}
}

func TestHandleGithubWebhook_DuplicateDelivery(t *testing.T) {
	repo := &fakeWebhookRepository{tryInsertResult: false}
	gh := &fakeGithubRepository{}
	hook, _ := NewWebhookUsecase(repo, gh, nil, nil, nil,
		Options.Secret("secret"), Options.MaxPayloadSize(1024))

	body := `{"action":"suspend","installation":{"id":42}}`
	req := newRequest(t, "installation", "del-dup", "secret", body)

	if _, err := hook.HandleGithubWebhook(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.statusUpdates) != 0 {
		t.Fatalf("duplicate delivery must not update status, got %v", repo.statusUpdates)
	}
}

func TestHandleGithubWebhook_InstallationRepositoriesAdded(t *testing.T) {
	webhookRepository := &fakeWebhookRepository{tryInsertResult: true}
	githubRepository := &fakeGithubRepository{dbIDByExternal: map[int64]string{42: "installation-db-id"}}
	projectRepository := &fakeProjectRepository{availability: map[int64]bool{7: false, 8: false}}
	hook, _ := NewWebhookUsecase(webhookRepository, githubRepository, projectRepository, nil, newTestRepositoryListCache(t),
		Options.Secret("secret"), Options.MaxPayloadSize(1024))

	body := `{"action":"added","installation":{"id":42},"repositories_added":[{"id":7}],"repositories_removed":[]}`
	if _, err := hook.HandleGithubWebhook(context.Background(), newRequest(t, "installation_repositories", "del-added", "secret", body)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !projectRepository.updated[7] || projectRepository.updated[8] {
		t.Fatalf("unexpected availability map: %v", projectRepository.updated)
	}
}

func TestHandleGithubWebhook_InstallationRepositoriesRemoved(t *testing.T) {
	webhookRepository := &fakeWebhookRepository{tryInsertResult: true}
	githubRepository := &fakeGithubRepository{dbIDByExternal: map[int64]string{42: "installation-db-id"}}
	projectRepository := &fakeProjectRepository{availability: map[int64]bool{7: true, 8: true}}
	hook, _ := NewWebhookUsecase(webhookRepository, githubRepository, projectRepository, nil, newTestRepositoryListCache(t),
		Options.Secret("secret"), Options.MaxPayloadSize(1024))

	body := `{"action":"removed","installation":{"id":42},"repositories_added":[],"repositories_removed":[{"id":7}]}`
	if _, err := hook.HandleGithubWebhook(context.Background(), newRequest(t, "installation_repositories", "del-removed", "secret", body)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if projectRepository.updated[7] || !projectRepository.updated[8] {
		t.Fatalf("unexpected availability map: %v", projectRepository.updated)
	}
}

func TestHandleGithubWebhook_InstallationRepositoriesMissingDependencies(t *testing.T) {
	hook, _ := NewWebhookUsecase(&fakeWebhookRepository{tryInsertResult: true}, &fakeGithubRepository{dbIDByExternal: map[int64]string{42: "installation-db-id"}}, nil, nil, nil,
		Options.Secret("secret"), Options.MaxPayloadSize(1024))

	body := `{"action":"added","installation":{"id":42},"repositories_added":[{"id":7}]}`
	_, err := hook.HandleGithubWebhook(context.Background(), newRequest(t, "installation_repositories", "del-missing-dependencies", "secret", body))
	if !errors.Is(err, domain.ErrInternalServerError) {
		t.Fatalf("expected internal server error, got %v", err)
	}
}

// eligibleProject returns a project that passes every push eligibility check:
// repository available, webhook builds enabled, configured branch refs/heads/main.
func eligibleProject() *domain.Project {
	return &domain.Project{
		ID:                    "proj-1",
		UserID:                "user-1",
		GitHubRepositoryID:    7,
		ConfiguredBranch:      "refs/heads/main",
		ProjectWebhookEnabled: true,
		RepositoryAvailable:   true,
	}
}

func assertAccepted(t *testing.T, repo *fakeWebhookRepository) {
	t.Helper()
	if len(repo.statusUpdates) != 1 || repo.statusUpdates[0] != domain.ProcessingStatusAccepted {
		t.Fatalf("expected exactly one accepted status update, got %v", repo.statusUpdates)
	}
}

func pushBody(ref, after string) string {
	return fmt.Sprintf(`{"ref":%q,"after":%q,"repository":{"id":7,"full_name":"u/r","clone_url":"https://github.com/u/r.git"},"installation":{"id":42}}`, ref, after)
}

func newPushHook(t *testing.T, projectRepository *fakeProjectRepository, deploymentRepository *fakeDeploymentRepository) (domain.WebhookUsecase, *fakeWebhookRepository) {
	t.Helper()
	repo := &fakeWebhookRepository{tryInsertResult: true}
	gh := &fakeGithubRepository{
		dbIDByExternal: map[int64]string{42: "installation-db-id"},
		statusByID:     map[string]string{"installation-db-id": domain.GithubInstallationStatusActive},
	}
	hook, err := NewWebhookUsecase(repo, gh, projectRepository, deploymentRepository, nil,
		Options.Secret("secret"), Options.MaxPayloadSize(1024))
	if err != nil {
		t.Fatalf("failed to create webhook usecase: %v", err)
	}
	return hook, repo
}

func TestHandleGithubWebhook_PushDeletedBranchSkipped(t *testing.T) {
	projectRepository := &fakeProjectRepository{project: eligibleProject()}
	hook, repo := newPushHook(t, projectRepository, &fakeDeploymentRepository{})

	body := `{"ref":"refs/heads/main","after":"0000000000000000000000000000000000000000","deleted":true,"repository":{"id":7},"installation":{"id":42}}`
	if _, err := hook.HandleGithubWebhook(context.Background(), newRequest(t, "push", "del-push-deleted", "secret", body)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projectRepository.lookups) != 0 {
		t.Fatalf("deleted-branch push must not look up the project, got %v", projectRepository.lookups)
	}
	assertAccepted(t, repo)
}

func TestHandleGithubWebhook_PushUnknownInstallationSkipped(t *testing.T) {
	projectRepository := &fakeProjectRepository{project: eligibleProject()}
	hook, repo := newPushHook(t, projectRepository, &fakeDeploymentRepository{})

	// No installation field: the push cannot resolve to a local installation.
	body := `{"ref":"refs/heads/main","after":"abc123","repository":{"id":7}}`
	if _, err := hook.HandleGithubWebhook(context.Background(), newRequest(t, "push", "del-push-unknown-inst", "secret", body)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projectRepository.lookups) != 0 {
		t.Fatalf("push without a resolvable installation must not look up the project, got %v", projectRepository.lookups)
	}
	assertAccepted(t, repo)
}

func TestHandleGithubWebhook_PushUnselectedRepositorySkipped(t *testing.T) {
	projectRepository := &fakeProjectRepository{} // project == nil -> ErrNotFound
	hook, repo := newPushHook(t, projectRepository, &fakeDeploymentRepository{})

	if _, err := hook.HandleGithubWebhook(context.Background(), newRequest(t, "push", "del-push-unselected", "secret", pushBody("refs/heads/main", "abc123"))); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projectRepository.lookups) != 1 {
		t.Fatalf("expected exactly one project lookup, got %v", projectRepository.lookups)
	}
	assertAccepted(t, repo)
}

func TestHandleGithubWebhook_PushDisabledProjectSkipped(t *testing.T) {
	project := eligibleProject()
	project.ProjectWebhookEnabled = false
	projectRepository := &fakeProjectRepository{project: project}
	hook, repo := newPushHook(t, projectRepository, &fakeDeploymentRepository{})

	if _, err := hook.HandleGithubWebhook(context.Background(), newRequest(t, "push", "del-push-disabled", "secret", pushBody("refs/heads/main", "abc123"))); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projectRepository.lookups) != 1 {
		t.Fatalf("expected exactly one project lookup, got %v", projectRepository.lookups)
	}
	assertAccepted(t, repo)
}

func TestHandleGithubWebhook_PushUnavailableRepositorySkipped(t *testing.T) {
	project := eligibleProject()
	project.RepositoryAvailable = false
	projectRepository := &fakeProjectRepository{project: project}
	hook, repo := newPushHook(t, projectRepository, &fakeDeploymentRepository{})

	if _, err := hook.HandleGithubWebhook(context.Background(), newRequest(t, "push", "del-push-unavailable", "secret", pushBody("refs/heads/main", "abc123"))); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertAccepted(t, repo)
}

func TestHandleGithubWebhook_PushBranchMismatchSkipped(t *testing.T) {
	projectRepository := &fakeProjectRepository{project: eligibleProject()}
	hook, repo := newPushHook(t, projectRepository, &fakeDeploymentRepository{})

	if _, err := hook.HandleGithubWebhook(context.Background(), newRequest(t, "push", "del-push-mismatch", "secret", pushBody("refs/heads/dev", "abc123"))); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertAccepted(t, repo)
}

func TestHandleGithubWebhook_PushEligibleCreatesBuild(t *testing.T) {
	project := eligibleProject()
	projectRepository := &fakeProjectRepository{project: project}
	deploymentRepository := &fakeDeploymentRepository{}
	hook, repo := newPushHook(t, projectRepository, deploymentRepository)

	if _, err := hook.HandleGithubWebhook(context.Background(), newRequest(t, "push", "del-push-eligible", "secret", pushBody("refs/heads/main", "abc123def456"))); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projectRepository.lookups) != 1 || projectRepository.lookups[0].installationDBID != "installation-db-id" || projectRepository.lookups[0].repositoryID != 7 {
		t.Fatalf("expected lookup with installation-db-id/repo 7, got %v", projectRepository.lookups)
	}
	if len(deploymentRepository.stored) != 1 {
		t.Fatalf("expected exactly one webhook build, got %d", len(deploymentRepository.stored))
	}

	stored := deploymentRepository.stored[0]
	if stored.ProjectID != "proj-1" || stored.UserID != "user-1" || stored.RepoID != 7 {
		t.Fatalf("unexpected identity fields: %+v", stored)
	}
	if stored.GithubInstallationID != "installation-db-id" || stored.WebhookDeliveryID != "delivery-db-id" {
		t.Fatalf("unexpected linkage fields: %+v", stored)
	}
	if stored.CommitSHA != "abc123def456" || stored.RequestedRef != "refs/heads/main" {
		t.Fatalf("unexpected source fields: %+v", stored)
	}
	if stored.DesiredRevisionGeneration != 1 {
		t.Fatalf("expected generation 1, got %d", stored.DesiredRevisionGeneration)
	}
	if stored.CloneURL != "https://github.com/u/r.git" {
		t.Fatalf("unexpected clone URL: %q", stored.CloneURL)
	}
	if stored.EventID == "" || stored.CorrelationID == "" || stored.InstallationID != 42 {
		t.Fatalf("expected V1 job metadata populated, got %+v", stored)
	}
	assertAccepted(t, repo)
}

func TestHandleGithubWebhook_PushForcePushBuildsAfterSHA(t *testing.T) {
	projectRepository := &fakeProjectRepository{project: eligibleProject()}
	deploymentRepository := &fakeDeploymentRepository{}
	hook, repo := newPushHook(t, projectRepository, deploymentRepository)

	body := `{"ref":"refs/heads/main","before":"1111111111111111111111111111111111111111","after":"9999999999999999999999999999999999999999","forced":true,"repository":{"id":7,"clone_url":"https://github.com/u/r.git"},"installation":{"id":42}}`
	if _, err := hook.HandleGithubWebhook(context.Background(), newRequest(t, "push", "del-push-force", "secret", body)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(deploymentRepository.stored) != 1 || deploymentRepository.stored[0].CommitSHA != "9999999999999999999999999999999999999999" {
		t.Fatalf("expected force push to build its after SHA, got %+v", deploymentRepository.stored)
	}
	assertAccepted(t, repo)
}

func TestHandleGithubWebhook_PushInactiveInstallationSkipped(t *testing.T) {
	projectRepository := &fakeProjectRepository{project: eligibleProject()}
	deploymentRepository := &fakeDeploymentRepository{}
	repo := &fakeWebhookRepository{tryInsertResult: true}
	gh := &fakeGithubRepository{
		dbIDByExternal: map[int64]string{42: "installation-db-id"},
		statusByID:     map[string]string{"installation-db-id": domain.GithubInstallationStatusSuspended},
	}
	hook, _ := NewWebhookUsecase(repo, gh, projectRepository, deploymentRepository, nil,
		Options.Secret("secret"), Options.MaxPayloadSize(1024))

	if _, err := hook.HandleGithubWebhook(context.Background(), newRequest(t, "push", "del-push-suspended", "secret", pushBody("refs/heads/main", "abc123"))); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(deploymentRepository.stored) != 0 {
		t.Fatalf("suspended installation must not create a build, got %+v", deploymentRepository.stored)
	}
	if len(projectRepository.lookups) != 0 {
		t.Fatalf("suspended installation must short-circuit before project lookup, got %v", projectRepository.lookups)
	}
	assertAccepted(t, repo)
}

func TestHandleGithubWebhook_PushBuildConflictIsNoOp(t *testing.T) {
	projectRepository := &fakeProjectRepository{project: eligibleProject()}
	deploymentRepository := &fakeDeploymentRepository{storeErr: domain.ErrConflict}
	hook, repo := newPushHook(t, projectRepository, deploymentRepository)

	if _, err := hook.HandleGithubWebhook(context.Background(), newRequest(t, "push", "del-push-conflict", "secret", pushBody("refs/heads/main", "abc123"))); err != nil {
		t.Fatalf("redelivery conflict must be a no-op, got %v", err)
	}

	assertAccepted(t, repo)
}

func TestHandleGithubWebhook_PushStoreErrorFailsDelivery(t *testing.T) {
	projectRepository := &fakeProjectRepository{project: eligibleProject()}
	deploymentRepository := &fakeDeploymentRepository{storeErr: domain.ErrInternalServerError}
	hook, repo := newPushHook(t, projectRepository, deploymentRepository)

	_, err := hook.HandleGithubWebhook(context.Background(), newRequest(t, "push", "del-push-store-err", "secret", pushBody("refs/heads/main", "abc123")))
	if !errors.Is(err, domain.ErrInternalServerError) {
		t.Fatalf("expected internal server error, got %v", err)
	}

	if len(repo.statusUpdates) != 1 || repo.statusUpdates[0] != domain.ProcessingStatusFailed {
		t.Fatalf("expected delivery marked failed, got %v", repo.statusUpdates)
	}
}

func TestHandleGithubWebhook_PushProjectLookupErrorFailsDelivery(t *testing.T) {
	projectRepository := &fakeProjectRepository{projectErr: domain.ErrInternalServerError}
	hook, repo := newPushHook(t, projectRepository, &fakeDeploymentRepository{})

	_, err := hook.HandleGithubWebhook(context.Background(), newRequest(t, "push", "del-push-lookup-err", "secret", pushBody("refs/heads/main", "abc123")))
	if !errors.Is(err, domain.ErrInternalServerError) {
		t.Fatalf("expected internal server error, got %v", err)
	}

	if len(repo.statusUpdates) != 1 || repo.statusUpdates[0] != domain.ProcessingStatusFailed {
		t.Fatalf("expected delivery marked failed, got %v", repo.statusUpdates)
	}
}
