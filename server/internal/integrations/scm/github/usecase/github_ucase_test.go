package usecase

import (
	"Zero_Devops/server/internal/domain"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

type mockGithubRepository struct {
	storeFn      func(ctx context.Context, inst *domain.GithubInstallation) error
	getFn        func(ctx context.Context, userID string) (*domain.GithubInstallation, error)
	getByIDFn    func(ctx context.Context, id string) (*domain.GithubInstallation, error)
	listFn       func(ctx context.Context, userID string) ([]*domain.GithubInstallation, error)
	updateConnFn func(ctx context.Context, userID, installationDBID string, status domain.GithubConnectionStatus) error
	deleteFn     func(ctx context.Context, userID string) error
}

func (m *mockGithubRepository) StoreInstallation(ctx context.Context, inst *domain.GithubInstallation) error {
	if m.storeFn != nil {
		return m.storeFn(ctx, inst)
	}
	return nil
}

func (m *mockGithubRepository) GetInstallationByUserID(ctx context.Context, userID string) (*domain.GithubInstallation, error) {
	if m.getFn != nil {
		return m.getFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockGithubRepository) DeleteInstallationByUserID(ctx context.Context, userID string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, userID)
	}
	return nil
}

func (m *mockGithubRepository) UpdateInstallationStatus(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockGithubRepository) UpdateInstallationStatusByGithubInstallationID(_ context.Context, _ int64, _ string) error {
	return nil
}

func (m *mockGithubRepository) GetInstallationIDByGithubInstallationID(_ context.Context, _ int64) (string, error) {
	return "", nil
}

func (m *mockGithubRepository) UpdateInstallationExternalIDByID(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockGithubRepository) GetInstallationStatusByID(_ context.Context, _ string) (string, error) {
	return domain.GithubInstallationStatusActive, nil
}

func (m *mockGithubRepository) GetInstallationByID(ctx context.Context, id string) (*domain.GithubInstallation, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, domain.ErrNotFound
}

func (m *mockGithubRepository) ListInstallationsByUserID(ctx context.Context, userID string) ([]*domain.GithubInstallation, error) {
	if m.listFn != nil {
		return m.listFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockGithubRepository) UpdateConnectionStatus(ctx context.Context, userID, installationDBID string, status domain.GithubConnectionStatus) error {
	if m.updateConnFn != nil {
		return m.updateConnFn(ctx, userID, installationDBID, status)
	}
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestClient(fn roundTripFunc) *http.Client {
	return &http.Client{Transport: fn}
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func setGithubUsecaseConfig() {
	viper.Set("GITHUB_APP_CLIENT_ID", "client-id")
	viper.Set("GITHUB_APP_CLIENT_SECRET", "client-secret")
	viper.Set("GITHUB_APP_ID", int64(1001))
}

func TestInstallGithubApp_Success(t *testing.T) {
	setGithubUsecaseConfig()

	repo := &mockGithubRepository{}
	var stored *domain.GithubInstallation
	repo.storeFn = func(_ context.Context, inst *domain.GithubInstallation) error {
		stored = inst
		return nil
	}

	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.String() == "https://github.com/login/oauth/access_token":
			if got := req.Header.Get("Accept"); got != "application/json" {
				t.Fatalf("expected accept header application/json, got %s", got)
			}
			return jsonResponse(http.StatusOK, `{"access_token":"token-123","token_type":"bearer","scope":"repo"}`), nil
		case req.Method == http.MethodGet && req.URL.String() == "https://api.github.com/user/installations":
			if got := req.Header.Get("Authorization"); got != "Bearer token-123" {
				t.Fatalf("expected bearer token header, got %s", got)
			}
			return jsonResponse(http.StatusOK, `{"total_count":1,"installations":[{"id":55,"app_id":1001,"account":{"login":"octocat","type":"User"}}]}`), nil
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
			return nil, errors.New("unexpected request")
		}
	})

	uc := NewGithubAppUsecase(repo)
	if err := uc.InstallGithubApp(context.Background(), client, "code-abc", "77"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if stored == nil {
		t.Fatal("expected installation to be stored")
	}
	if stored.UserID != "77" || stored.InstallationID != 55 || stored.AccountLogin != "octocat" {
		t.Fatalf("unexpected stored installation: %+v", stored)
	}
	if stored.Status != domain.GithubInstallationStatusActive {
		t.Fatalf("expected active status, got %s", stored.Status)
	}
	if stored.CreatedAt.IsZero() || stored.UpdatedAt.IsZero() {
		t.Fatal("expected timestamps to be set")
	}
}

func TestInstallGithubApp_InvalidCodeExchange(t *testing.T) {
	setGithubUsecaseConfig()

	repo := &mockGithubRepository{}
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPost {
			return jsonResponse(http.StatusUnauthorized, `{}`), nil
		}
		t.Fatal("unexpected request after failed code exchange")
		return nil, errors.New("unexpected request")
	})

	uc := NewGithubAppUsecase(repo)
	if err := uc.InstallGithubApp(context.Background(), client, "bad-code", "77"); err != domain.ErrInvalidCode {
		t.Fatalf("expected ErrInvalidCode, got %v", err)
	}
}

func TestInstallGithubApp_FetchInstallationsError(t *testing.T) {
	setGithubUsecaseConfig()

	repo := &mockGithubRepository{}
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "https://github.com/login/oauth/access_token":
			return jsonResponse(http.StatusOK, `{"access_token":"token-123"}`), nil
		case "https://api.github.com/user/installations":
			return jsonResponse(http.StatusForbidden, `{}`), nil
		default:
			t.Fatal("unexpected request")
			return nil, errors.New("unexpected request")
		}
	})

	uc := NewGithubAppUsecase(repo)
	if err := uc.InstallGithubApp(context.Background(), client, "code-abc", "77"); err != domain.ErrGithubInstallationFetchFailed {
		t.Fatalf("expected ErrGithubInstallationFetchFailed, got %v", err)
	}
}

func TestGetGithubAppInstallation(t *testing.T) {
	expected := &domain.GithubInstallation{UserID: "11", InstallationID: 22, Status: domain.GithubInstallationStatusActive}
	repo := &mockGithubRepository{
		getFn: func(_ context.Context, userID string) (*domain.GithubInstallation, error) {
			if userID != "11" {
				t.Fatalf("expected userID 11, got %s", userID)
			}
			return expected, nil
		},
	}

	uc := NewGithubAppUsecase(repo)
	got, err := uc.GetGithubAppInstallation(context.Background(), "11")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.InstallationID != expected.InstallationID {
		t.Fatalf("expected installationID %d, got %d", expected.InstallationID, got.InstallationID)
	}
}

func TestDeleteGithubApp(t *testing.T) {
	called := false
	repo := &mockGithubRepository{
		deleteFn: func(_ context.Context, userID string) error {
			called = true
			if userID != "99" {
				t.Fatalf("expected userID 99, got %s", userID)
			}
			return nil
		},
	}

	uc := NewGithubAppUsecase(repo)
	if err := uc.DeleteGithubApp(context.Background(), "99"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !called {
		t.Fatal("expected delete repository call")
	}
}

func TestInstallGithubApp_DecodesTokenAndInstallationResponse(t *testing.T) {
	setGithubUsecaseConfig()

	repo := &mockGithubRepository{}
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == "https://github.com/login/oauth/access_token" {
			body := bytes.NewBufferString(`{"access_token":"token-xyz","token_type":"bearer","scope":"repo"}`)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(body), Header: make(http.Header)}, nil
		}
		if req.URL.String() == "https://api.github.com/user/installations" {
			body := bytes.NewBufferString(`{"total_count":1,"installations":[{"id":88,"app_id":1001,"account":{"login":"alice","type":"User"}}]}`)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(body), Header: make(http.Header)}, nil
		}
		return nil, errors.New("unexpected request")
	})

	uc := NewGithubAppUsecase(repo)
	if err := uc.InstallGithubApp(context.Background(), client, "code", "1"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestInstallGithubApp_DefaultsStatusToActive(t *testing.T) {
	setGithubUsecaseConfig()

	repo := &mockGithubRepository{}
	var stored *domain.GithubInstallation
	repo.storeFn = func(_ context.Context, inst *domain.GithubInstallation) error {
		stored = inst
		return nil
	}

	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "https://github.com/login/oauth/access_token":
			return jsonResponse(http.StatusOK, `{"access_token":"token-123","token_type":"bearer","scope":"repo"}`), nil
		case "https://api.github.com/user/installations":
			return jsonResponse(http.StatusOK, `{"total_count":1,"installations":[{"id":55,"app_id":1001,"account":{"login":"octocat","type":"User"}}]}`), nil
		default:
			return nil, errors.New("unexpected request")
		}
	})

	uc := NewGithubAppUsecase(repo)
	if err := uc.InstallGithubApp(context.Background(), client, "code-abc", "77"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if stored == nil {
		t.Fatal("expected stored installation")
	}
	if stored.Status != domain.GithubInstallationStatusActive {
		t.Fatalf("expected status %s, got %s", domain.GithubInstallationStatusActive, stored.Status)
	}
}

func TestUpdateConnectionStatus_ConnectedWhenActive(t *testing.T) {
	repo := &mockGithubRepository{
		getByIDFn: func(_ context.Context, id string) (*domain.GithubInstallation, error) {
			return &domain.GithubInstallation{ID: id, UserID: "user-1", Status: domain.GithubInstallationStatusActive, ConnectionStatus: domain.GithubConnectionStatusDisconnected}, nil
		},
		updateConnFn: func(_ context.Context, userID, id string, status domain.GithubConnectionStatus) error {
			if userID != "user-1" || id != "inst-1" || status != domain.GithubConnectionStatusConnected {
				t.Fatalf("unexpected update args: user=%s, id=%s, status=%s", userID, id, status)
			}
			return nil
		},
	}

	uc := NewGithubAppUsecase(repo)
	res, err := uc.UpdateConnectionStatus(context.Background(), "user-1", "inst-1", domain.GithubConnectionStatusConnected)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if res.ConnectionStatus != domain.GithubConnectionStatusConnected {
		t.Fatalf("expected status connected, got %s", res.ConnectionStatus)
	}
}

func TestUpdateConnectionStatus_ConnectedWhenSuspended(t *testing.T) {
	repo := &mockGithubRepository{
		getByIDFn: func(_ context.Context, id string) (*domain.GithubInstallation, error) {
			return &domain.GithubInstallation{ID: id, UserID: "user-1", Status: domain.GithubInstallationStatusSuspended, ConnectionStatus: domain.GithubConnectionStatusDisconnected}, nil
		},
	}

	uc := NewGithubAppUsecase(repo)
	_, err := uc.UpdateConnectionStatus(context.Background(), "user-1", "inst-1", domain.GithubConnectionStatusConnected)
	if !errors.Is(err, domain.ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestUpdateConnectionStatus_ConnectedWhenUninstalled(t *testing.T) {
	repo := &mockGithubRepository{
		getByIDFn: func(_ context.Context, id string) (*domain.GithubInstallation, error) {
			return &domain.GithubInstallation{ID: id, UserID: "user-1", Status: domain.GithubInstallationStatusUninstalled, ConnectionStatus: domain.GithubConnectionStatusDisconnected}, nil
		},
	}

	uc := NewGithubAppUsecase(repo)
	_, err := uc.UpdateConnectionStatus(context.Background(), "user-1", "inst-1", domain.GithubConnectionStatusConnected)
	if !errors.Is(err, domain.ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestUpdateConnectionStatus_DisconnectedFromAnyState(t *testing.T) {
	repo := &mockGithubRepository{
		getByIDFn: func(_ context.Context, id string) (*domain.GithubInstallation, error) {
			return &domain.GithubInstallation{ID: id, UserID: "user-1", InstallationID: 99, Status: domain.GithubInstallationStatusSuspended, ConnectionStatus: domain.GithubConnectionStatusConnected}, nil
		},
		updateConnFn: func(_ context.Context, _, _ string, _ domain.GithubConnectionStatus) error {
			return nil
		},
	}

	uc := NewGithubAppUsecase(repo)
	res, err := uc.UpdateConnectionStatus(context.Background(), "user-1", "inst-1", domain.GithubConnectionStatusDisconnected)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if res.ConnectionStatus != domain.GithubConnectionStatusDisconnected {
		t.Fatalf("expected status disconnected, got %s", res.ConnectionStatus)
	}
}

func TestUpdateConnectionStatus_InvalidStatusInput(t *testing.T) {
	repo := &mockGithubRepository{}
	uc := NewGithubAppUsecase(repo)
	_, err := uc.UpdateConnectionStatus(context.Background(), "user-1", "inst-1", "invalid_status")
	if !errors.Is(err, domain.ErrBadParamInput) {
		t.Fatalf("expected ErrBadParamInput, got %v", err)
	}
}

func TestUpdateConnectionStatus_UserMismatch(t *testing.T) {
	repo := &mockGithubRepository{
		getByIDFn: func(_ context.Context, id string) (*domain.GithubInstallation, error) {
			return &domain.GithubInstallation{ID: id, UserID: "other-user", Status: domain.GithubInstallationStatusActive}, nil
		},
	}

	uc := NewGithubAppUsecase(repo)
	_, err := uc.UpdateConnectionStatus(context.Background(), "user-1", "inst-1", domain.GithubConnectionStatusConnected)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound on user mismatch, got %v", err)
	}
}

func TestListGithubAppInstallations(t *testing.T) {
	repo := &mockGithubRepository{
		listFn: func(_ context.Context, userID string) ([]*domain.GithubInstallation, error) {
			return []*domain.GithubInstallation{
				{ID: "inst-1", UserID: userID, ConnectionStatus: domain.GithubConnectionStatusConnected},
				{ID: "inst-2", UserID: userID, ConnectionStatus: domain.GithubConnectionStatusDisconnected},
			}, nil
		},
	}

	uc := NewGithubAppUsecase(repo)
	list, err := uc.ListGithubAppInstallations(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 installations, got %d", len(list))
	}
}
