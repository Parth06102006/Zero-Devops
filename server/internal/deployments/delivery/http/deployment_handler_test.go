package http

import (
	middleware "Zero_Devops/server/internal/auth/delivery/http/middleware"
	"Zero_Devops/server/internal/domain"
	"Zero_Devops/server/internal/helper"
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

type mockDeploymentUsecase struct {
	createFn             func(ctx context.Context, userID string, repoID int64, reqID string) (*domain.Deployment, error)
	createProjectBuildFn func(ctx context.Context, userID string, params domain.CreateProjectBuildParams) (*domain.Deployment, error)
}

func (m *mockDeploymentUsecase) CreateDeployment(ctx context.Context, userID string, repoID int64, reqID string) (*domain.Deployment, error) {
	if m.createFn != nil {
		return m.createFn(ctx, userID, repoID, reqID)
	}
	return nil, nil
}

func (m *mockDeploymentUsecase) CreateProjectBuild(ctx context.Context, userID string, params domain.CreateProjectBuildParams) (*domain.Deployment, error) {
	if m.createProjectBuildFn != nil {
		return m.createProjectBuildFn(ctx, userID, params)
	}
	return nil, nil
}

func (m *mockDeploymentUsecase) GetDeployments(_ context.Context, _ string) ([]domain.Deployment, error) {
	return nil, nil
}

func (m *mockDeploymentUsecase) GetDeploymentByID(_ context.Context, _, _ string) (*domain.Deployment, error) {
	return nil, nil
}

func TestCreateProjectBuild_Unauthorized(t *testing.T) {
	e := echo.New()
	h := &DeploymentHandler{dUsecase: &mockDeploymentUsecase{}}
	e.POST("/projects/:id/builds", h.CreateProjectBuild)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/projects/p1/builds", http.NoBody)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestCreateProjectBuild_ValidatesBody(t *testing.T) {
	e := echo.New()
	h := &DeploymentHandler{dUsecase: &mockDeploymentUsecase{}}
	e.POST("/projects/:id/builds", func(c *echo.Context) error {
		c.Set(middleware.UserIDContextKey, "11")
		return h.CreateProjectBuild(c)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/projects/p1/builds", bytes.NewBufferString(`{"sha_or_ref":"main"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestCreateProjectBuild_PassesParams(t *testing.T) {
	e := echo.New()
	body := `{"sha_or_ref":"refs/heads/main","idempotency_key":"11111111-1111-1111-1111-111111111111"}`
	h := &DeploymentHandler{dUsecase: &mockDeploymentUsecase{
		createProjectBuildFn: func(_ context.Context, userID string, params domain.CreateProjectBuildParams) (*domain.Deployment, error) {
			if userID != "11" || params.ProjectID != "p1" || params.ShaOrRef != "refs/heads/main" || params.IdempotencyKey != "11111111-1111-1111-1111-111111111111" {
				t.Fatalf("unexpected params userID=%s params=%+v", userID, params)
			}
			return &domain.Deployment{ID: "d1"}, nil
		},
	}}
	e.POST("/projects/:id/builds", func(c *echo.Context) error {
		c.Set(middleware.UserIDContextKey, "11")
		return h.CreateProjectBuild(c)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/projects/p1/builds", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}
}

func TestCreateDeployment_Unauthorized(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/deployments", http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := &DeploymentHandler{dUsecase: &mockDeploymentUsecase{}}
	if err := h.CreateDeployment(c); err != nil {
		t.Fatalf("expected nil echo error, got %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestCreateDeployment_InvalidBody(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/deployments", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.UserIDContextKey, "11")

	h := &DeploymentHandler{dUsecase: &mockDeploymentUsecase{}}
	if err := h.CreateDeployment(c); err != nil {
		t.Fatalf("expected nil echo error, got %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestCreateDeployment_LegacyRequestFailsClosed(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/deployments", bytes.NewBufferString(`{"repo_id":42}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.UserIDContextKey, "11")

	h := &DeploymentHandler{dUsecase: &mockDeploymentUsecase{
		createFn: func(_ context.Context, _ string, _ int64, _ string) (*domain.Deployment, error) {
			t.Fatal("legacy handler must not call the use case")
			return nil, nil
		},
	}}

	if err := h.CreateDeployment(c); err != nil {
		t.Fatalf("expected nil echo error, got %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("complete V1 build request")) {
		t.Fatalf("missing actionable error: %s", rec.Body.String())
	}
}

func TestCreateDeployment_UsecaseError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/deployments", bytes.NewBufferString(`{"repo_id":42}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.UserIDContextKey, "11")

	h := &DeploymentHandler{
		dUsecase: &mockDeploymentUsecase{
			createFn: func(_ context.Context, _ string, _ int64, _ string) (*domain.Deployment, error) {
				return nil, domain.ErrConflict
			},
		},
	}

	if err := h.CreateDeployment(c); err != nil {
		t.Fatalf("expected nil echo error, got %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
}

func TestGetStatusCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, http.StatusOK},
		{"not found", domain.ErrNotFound, http.StatusNotFound},
		{"conflict", domain.ErrConflict, http.StatusConflict},
		{"internal", domain.ErrInternalServerError, http.StatusInternalServerError},
		{"other", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := helper.GetStatusCode(tc.err); got != tc.want {
				t.Fatalf("expected %d, got %d", tc.want, got)
			}
		})
	}
}
