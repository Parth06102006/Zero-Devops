package usecase

import (
	"Zero_Devops/server/internal/domain"
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---- test doubles ----

type mockGithubRepositoryClient struct {
	listFn    func(ctx context.Context, token, cursor, query string, perPage int) (*domain.RepositoryList, error)
	detailsFn func(ctx context.Context, token string, repoID int64) (*domain.RepositoryPicker, error)
	resolveFn func(ctx context.Context, token, owner, repo, shaOrRef string) (string, error)
}

func (m *mockGithubRepositoryClient) ListRepositories(ctx context.Context, token, cursor, query string, perPage int) (*domain.RepositoryList, error) {
	if m.listFn != nil {
		return m.listFn(ctx, token, cursor, query, perPage)
	}
	return &domain.RepositoryList{}, nil
}

func (m *mockGithubRepositoryClient) GetRepositoryDetails(ctx context.Context, token string, repoID int64) (*domain.RepositoryPicker, error) {
	if m.detailsFn != nil {
		return m.detailsFn(ctx, token, repoID)
	}
	return &domain.RepositoryPicker{ID: repoID}, nil
}

func (m *mockGithubRepositoryClient) ResolveCommit(ctx context.Context, token, owner, repo, shaOrRef string) (string, error) {
	if m.resolveFn != nil {
		return m.resolveFn(ctx, token, owner, repo, shaOrRef)
	}
	return "0", nil
}

type mockInstallationTokenProvider struct {
	createFn func(ctx context.Context, installationID int64) (string, error)
}

func (m *mockInstallationTokenProvider) CreateInstallationToken(ctx context.Context, installationID int64) (string, error) {
	if m.createFn != nil {
		return m.createFn(ctx, installationID)
	}
	return "test-token", nil
}

type fakeRepositoryCache struct {
	mu     sync.Mutex
	data   map[string]*domain.RepositoryList
	getErr error
}

func (f *fakeRepositoryCache) Get(_ context.Context, key string) (*domain.RepositoryList, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.data[key], nil
}

func (f *fakeRepositoryCache) Set(_ context.Context, key string, value *domain.RepositoryList, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.data == nil {
		f.data = map[string]*domain.RepositoryList{}
	}
	f.data[key] = value
	return nil
}

func (f *fakeRepositoryCache) InvalidateInstallation(_ context.Context, _ int64) error { return nil }

func installationFor(userID string, installationID int64, status string) *mockGithubRepository {
	return &mockGithubRepository{
		getFn: func(_ context.Context, uid string) (*domain.GithubInstallation, error) {
			if uid != userID {
				return nil, errors.New("unexpected user")
			}
			return &domain.GithubInstallation{
				UserID:         uid,
				InstallationID: installationID,
				Status:         status,
			}, nil
		},
	}
}

func TestListRepositories_InactiveInstallation(t *testing.T) {
	uc := NewGithubAppUsecase(installationFor("u", 10, domain.GithubInstallationStatusSuspended))
	_, err := uc.ListRepositories(context.Background(), "u", "", "", 30)
	if !errors.Is(err, domain.ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestListRepositories_CacheMissFetchesAndCaches(t *testing.T) {
	var calls int32
	client := &mockGithubRepositoryClient{
		listFn: func(_ context.Context, _ , _ , _ string, perPage int) (*domain.RepositoryList, error) {
			atomic.AddInt32(&calls, 1)
			if perPage != 30 {
				t.Fatalf("expected perPage 30, got %d", perPage)
			}
			return &domain.RepositoryList{Repositories: []domain.RepositoryPicker{{ID: 1, Name: "a"}}}, nil
		},
	}
	cache := &fakeRepositoryCache{}
	uc := NewGithubAppUsecase(installationFor("u", 10, domain.GithubInstallationStatusActive), &mockInstallationTokenProvider{}, client, cache)

	got, err := uc.ListRepositories(context.Background(), "u", "", "", 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Repositories[0].Name != "a" {
		t.Fatalf("unexpected repositories: %+v", got)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected 1 backend call, got %d", calls)
	}
	if len(cache.data) != 1 {
		t.Fatalf("expected 1 cached key, got %d", len(cache.data))
	}
}

func TestListRepositories_CacheHitSkipsClient(t *testing.T) {
	var calls int32
	client := &mockGithubRepositoryClient{
		listFn: func(_ context.Context, _ , _ , _ string, _ int) (*domain.RepositoryList, error) {
			atomic.AddInt32(&calls, 1)
			return &domain.RepositoryList{Repositories: []domain.RepositoryPicker{{ID: 1, Name: "a"}}}, nil
		},
	}
	cache := &fakeRepositoryCache{}
	uc := NewGithubAppUsecase(installationFor("u", 10, domain.GithubInstallationStatusActive), &mockInstallationTokenProvider{}, client, cache)

	if _, err := uc.ListRepositories(context.Background(), "u", "", "", 30); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := uc.ListRepositories(context.Background(), "u", "", "", 30); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected 1 backend call (cache hit), got %d", calls)
	}
}

func TestListRepositories_CacheFailureFallsThrough(t *testing.T) {
	var calls int32
	client := &mockGithubRepositoryClient{
		listFn: func(_ context.Context, _ , _ , _ string, _ int) (*domain.RepositoryList, error) {
			atomic.AddInt32(&calls, 1)
			return &domain.RepositoryList{Repositories: []domain.RepositoryPicker{{ID: 1, Name: "a"}}}, nil
		},
	}
	cache := &fakeRepositoryCache{getErr: errors.New("redis down")}
	uc := NewGithubAppUsecase(installationFor("u", 10, domain.GithubInstallationStatusActive), &mockInstallationTokenProvider{}, client, cache)

	if _, err := uc.ListRepositories(context.Background(), "u", "", "", 30); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected 1 backend call after cache failure, got %d", calls)
	}
}

func TestListRepositories_KeyIsolationByInstallation(t *testing.T) {
	var mu sync.Mutex
	var totalCalls int
	client := &mockGithubRepositoryClient{
		listFn: func(_ context.Context, _ , _ , _ string, _ int) (*domain.RepositoryList, error) {
			mu.Lock()
			totalCalls++
			mu.Unlock()
			return &domain.RepositoryList{Repositories: []domain.RepositoryPicker{{ID: int64(totalCalls)}}}, nil
		},
	}
	cache := &fakeRepositoryCache{}
	repo := &mockGithubRepository{
		getFn: func(_ context.Context, uid string) (*domain.GithubInstallation, error) {
			if uid == "a" {
				return &domain.GithubInstallation{UserID: "a", InstallationID: 10, Status: domain.GithubInstallationStatusActive}, nil
			}
			return &domain.GithubInstallation{UserID: "b", InstallationID: 20, Status: domain.GithubInstallationStatusActive}, nil
		},
	}
	uc := NewGithubAppUsecase(repo, &mockInstallationTokenProvider{}, client, cache)

	if _, err := uc.ListRepositories(context.Background(), "a", "", "", 30); err != nil {
		t.Fatalf("unexpected error for a: %v", err)
	}
	if _, err := uc.ListRepositories(context.Background(), "b", "", "", 30); err != nil {
		t.Fatalf("unexpected error for b: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if totalCalls != 2 {
		t.Fatalf("expected 2 backend calls (separate keys), got %d", totalCalls)
	}
	if len(cache.data) != 2 {
		t.Fatalf("expected 2 cache keys, got %d", len(cache.data))
	}
	for k := range cache.data {
		if !strings.Contains(k, "github:installation:") || !(strings.Contains(k, ":10:") || strings.Contains(k, ":20:")) {
			t.Fatalf("unexpected cache key shape: %s", k)
		}
	}
}

func TestListRepositories_SingleFlight(t *testing.T) {
	var calls int32
	client := &mockGithubRepositoryClient{
		listFn: func(_ context.Context, _ , _ , _ string, _ int) (*domain.RepositoryList, error) {
			atomic.AddInt32(&calls, 1)
			time.Sleep(20 * time.Millisecond)
			return &domain.RepositoryList{Repositories: []domain.RepositoryPicker{{ID: 1}}}, nil
		},
	}
	uc := NewGithubAppUsecase(installationFor("u", 10, domain.GithubInstallationStatusActive), &mockInstallationTokenProvider{}, client, &fakeRepositoryCache{})

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := uc.ListRepositories(context.Background(), "u", "", "", 30); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected 1 backend call due to single-flight, got %d", calls)
	}
}

func TestListRepositories_NoDependenciesFailsClosed(t *testing.T) {
	uc := NewGithubAppUsecase(installationFor("u", 10, domain.GithubInstallationStatusActive))
	_, err := uc.ListRepositories(context.Background(), "u", "", "", 30)
	if !errors.Is(err, domain.ErrInternalServerError) {
		t.Fatalf("expected ErrInternalServerError, got %v", err)
	}
}

func TestGetRepositoryDetails_InactiveInstallation(t *testing.T) {
	uc := NewGithubAppUsecase(installationFor("u", 10, domain.GithubInstallationStatusSuspended))
	_, err := uc.GetRepositoryDetails(context.Background(), "u", 5)
	if !errors.Is(err, domain.ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestInvalidateRepositoryCache_NoCacheIsNoop(t *testing.T) {
	uc := NewGithubAppUsecase(installationFor("u", 10, domain.GithubInstallationStatusActive))
	if err := uc.InvalidateRepositoryCache(context.Background(), 10); err != nil {
		t.Fatalf("expected nil error with no cache, got %v", err)
	}
}

func TestListRepositories_TokenErrorIsAuthorizationFailure(t *testing.T) {
	var clientCalled int32
	client := &mockGithubRepositoryClient{
		listFn: func(_ context.Context, _ , _ , _ string, _ int) (*domain.RepositoryList, error) {
			atomic.AddInt32(&clientCalled, 1)
			return &domain.RepositoryList{}, nil
		},
	}
	tp := &mockInstallationTokenProvider{
		createFn: func(_ context.Context, _ int64) (string, error) {
			return "", errors.New("github authorization failed")
		},
	}
	uc := NewGithubAppUsecase(installationFor("u", 10, domain.GithubInstallationStatusActive), tp, client, &fakeRepositoryCache{})

	_, err := uc.ListRepositories(context.Background(), "u", "", "", 30)
	if err == nil {
		t.Fatal("expected error when installation token creation fails")
	}
	if !strings.Contains(err.Error(), "authorization") {
		t.Fatalf("expected authorization failure, got %v", err)
	}
	if atomic.LoadInt32(&clientCalled) != 0 {
		t.Fatalf("github client must not be called when token creation fails, got %d calls", clientCalled)
	}
}

func TestListRepositories_PaginationForwardsParams(t *testing.T) {
	var gotCursor string
	var gotPerPage int
	client := &mockGithubRepositoryClient{
		listFn: func(_ context.Context, _ , cursor, _ string, perPage int) (*domain.RepositoryList, error) {
			gotCursor = cursor
			gotPerPage = perPage
			return &domain.RepositoryList{
				Repositories: []domain.RepositoryPicker{{ID: 1, Name: "a"}},
				NextCursor:   "next123",
			}, nil
		},
	}
	uc := NewGithubAppUsecase(installationFor("u", 10, domain.GithubInstallationStatusActive), &mockInstallationTokenProvider{}, client, &fakeRepositoryCache{})

	res, err := uc.ListRepositories(context.Background(), "u", "abc", "", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotCursor != "abc" {
		t.Fatalf("expected cursor 'abc' forwarded to client, got %q", gotCursor)
	}
	if gotPerPage != 50 {
		t.Fatalf("expected perPage 50 forwarded to client, got %d", gotPerPage)
	}
	if res.NextCursor != "next123" {
		t.Fatalf("expected next cursor 'next123' returned to caller, got %q", res.NextCursor)
	}
}

// fakeRepositoryCacheWithInvalidation mimics the Redis SCAN-based invalidation
// so webhook-driven eviction can be tested without a live Redis.
type fakeRepositoryCacheWithInvalidation struct {
	fakeRepositoryCache
}

func (f *fakeRepositoryCacheWithInvalidation) InvalidateInstallation(_ context.Context, installationID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	prefix := "github:installation:" + strconv.FormatInt(installationID, 10)
	for k := range f.data {
		if strings.HasPrefix(k, prefix) {
			delete(f.data, k)
		}
	}
	return nil
}

func TestInvalidateRepositoryCache_RemovesCachedKeys(t *testing.T) {
	var calls int32
	client := &mockGithubRepositoryClient{
		listFn: func(_ context.Context, _ , _ , _ string, _ int) (*domain.RepositoryList, error) {
			atomic.AddInt32(&calls, 1)
			return &domain.RepositoryList{Repositories: []domain.RepositoryPicker{{ID: 1}}}, nil
		},
	}
	cache := &fakeRepositoryCacheWithInvalidation{}
	uc := NewGithubAppUsecase(installationFor("u", 10, domain.GithubInstallationStatusActive), &mockInstallationTokenProvider{}, client, cache)

	if _, err := uc.ListRepositories(context.Background(), "u", "", "", 30); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected 1 backend call before invalidation, got %d", calls)
	}

	if err := uc.InvalidateRepositoryCache(context.Background(), 10); err != nil {
		t.Fatalf("unexpected invalidation error: %v", err)
	}

	if _, err := uc.ListRepositories(context.Background(), "u", "", "", 30); err != nil {
		t.Fatalf("unexpected error after invalidation: %v", err)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("expected a cache miss after invalidation (2 backend calls), got %d", calls)
	}
}
