package http

import (
	"Zero_Devops/server/authorization/auth/delivery/http/middleware"
	"Zero_Devops/server/domain"
	"Zero_Devops/server/helper"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

type mockGithubUsecase struct {
	installFn func(ctx context.Context, client *http.Client, code string, userID int64) error
	getFn     func(ctx context.Context, userID int64) (*domain.GithubInstallation, error)
	deleteFn  func(ctx context.Context, userID int64) error
}

func (m *mockGithubUsecase) InstallGithubApp(ctx context.Context, client *http.Client, code string, userID int64) error {
	if m.installFn != nil {
		return m.installFn(ctx, client, code, userID)
	}
	return nil
}

func (m *mockGithubUsecase) GetGithubAppInstallation(ctx context.Context, userID int64) (*domain.GithubInstallation, error) {
	if m.getFn != nil {
		return m.getFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockGithubUsecase) DeleteGithubApp(ctx context.Context, userID int64) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, userID)
	}
	return nil
}

func newSCMTestContext(method, target string) (*httptest.ResponseRecorder, *echo.Context) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), method, target, http.NoBody)
	e := echo.New()
	return rec, e.NewContext(req, rec)
}

func setUserID(c *echo.Context, userID int64) {
	c.Set(middleware.UserIDContextKey, userID)
}

func TestInstallation_MissingCode(t *testing.T) {
	handler := &SCMHandler{scmUsecase: &mockGithubUsecase{}}
	rec, c := newSCMTestContext(http.MethodPost, "/integration/scm/github/install")
	setUserID(c, 1)

	if err := handler.Installation(c); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestInstallation_MissingUserID(t *testing.T) {
	handler := &SCMHandler{scmUsecase: &mockGithubUsecase{}}
	rec, c := newSCMTestContext(http.MethodPost, "/integration/scm/github/install?code=test-code")

	if err := handler.Installation(c); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestInstallation_Success(t *testing.T) {
	called := false
	handler := &SCMHandler{
		scmUsecase: &mockGithubUsecase{
			installFn: func(_ context.Context, client *http.Client, code string, userID int64) error {
				called = true
				if code != "test-code" {
					t.Fatalf("expected code test-code, got %s", code)
				}
				if userID != 42 {
					t.Fatalf("expected userID 42, got %d", userID)
				}
				if client == nil {
					t.Fatal("expected non-nil client")
				}
				return nil
			},
		},
	}

	rec, c := newSCMTestContext(http.MethodPost, "/integration/scm/github/install?code=test-code")
	setUserID(c, 42)

	if err := handler.Installation(c); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if !called {
		t.Fatal("expected install usecase to be called")
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestGetInstallation_MissingUserID(t *testing.T) {
	handler := &SCMHandler{scmUsecase: &mockGithubUsecase{}}
	rec, c := newSCMTestContext(http.MethodGet, "/integration/scm/github/")

	if err := handler.GetInstallation(c); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestGetInstallation_Success(t *testing.T) {
	expected := &domain.GithubInstallation{
		ID:             1,
		UserID:         99,
		InstallationID: 12345,
		AccountType:    "User",
		AccountLogin:   "octocat",
	}

	handler := &SCMHandler{
		scmUsecase: &mockGithubUsecase{
			getFn: func(_ context.Context, userID int64) (*domain.GithubInstallation, error) {
				if userID != 99 {
					t.Fatalf("expected userID 99, got %d", userID)
				}
				return expected, nil
			},
		},
	}

	rec, c := newSCMTestContext(http.MethodGet, "/integration/scm/github/")
	setUserID(c, 99)

	if err := handler.GetInstallation(c); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp struct {
		Success bool                      `json:"success"`
		Data    domain.GithubInstallation `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success to be true")
	}
	if resp.Data.InstallationID != expected.InstallationID {
		t.Fatalf("expected installation_id %d, got %d", expected.InstallationID, resp.Data.InstallationID)
	}
}

func TestDeleteInstallation_Success(t *testing.T) {
	called := false
	handler := &SCMHandler{
		scmUsecase: &mockGithubUsecase{
			deleteFn: func(_ context.Context, userID int64) error {
				called = true
				if userID != 7 {
					t.Fatalf("expected userID 7, got %d", userID)
				}
				return nil
			},
		},
	}

	rec, c := newSCMTestContext(http.MethodDelete, "/integration/scm/github/delete")
	setUserID(c, 7)

	if err := handler.DeleteInstallation(c); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if !called {
		t.Fatal("expected delete usecase to be called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestGetStatusCode(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		expect int
	}{
		{"nil", nil, http.StatusOK},
		{"not found", domain.ErrNotFound, http.StatusNotFound},
		{"conflict", domain.ErrConflict, http.StatusConflict},
		{"internal", domain.ErrInternalServerError, http.StatusInternalServerError},
		{"default", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := helper.GetStatusCode(tt.err); got != tt.expect {
				t.Fatalf("expected %d, got %d", tt.expect, got)
			}
		})
	}
}
