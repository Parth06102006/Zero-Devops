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
	listProjectBuildsFn  func(ctx context.Context, userID, projectID string) ([]domain.Deployment, error)
	getBuildFn           func(ctx context.Context, userID, buildID string) (*domain.Deployment, error)
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

func (m *mockDeploymentUsecase) ListProjectBuilds(ctx context.Context, userID, projectID string) ([]domain.Deployment, error) {
	if m.listProjectBuildsFn != nil {
		return m.listProjectBuildsFn(ctx, userID, projectID)
	}
	return nil, nil
}

func (m *mockDeploymentUsecase) GetBuild(ctx context.Context, userID, buildID string) (*domain.Deployment, error) {
	if m.getBuildFn != nil {
		return m.getBuildFn(ctx, userID, buildID)
	}
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

func TestListProjectBuilds_Unauthorized(t *testing.T) {
	e := echo.New()
	h := &DeploymentHandler{dUsecase: &mockDeploymentUsecase{}}
	e.GET("/projects/:id/builds", h.ListProjectBuilds)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/projects/p1/builds", http.NoBody)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestListProjectBuilds_MissingProjectID(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/projects//builds", http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.UserIDContextKey, "11")
	c.SetPathValues(echo.PathValues{{Name: "id", Value: ""}})

	h := &DeploymentHandler{dUsecase: &mockDeploymentUsecase{}}
	if err := h.ListProjectBuilds(c); err != nil {
		t.Fatalf("expected nil echo error, got %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestListProjectBuilds_PassesParams(t *testing.T) {
	e := echo.New()
	h := &DeploymentHandler{dUsecase: &mockDeploymentUsecase{
		listProjectBuildsFn: func(_ context.Context, userID, projectID string) ([]domain.Deployment, error) {
			if userID != "11" || projectID != "p1" {
				t.Fatalf("unexpected args userID=%s projectID=%s", userID, projectID)
			}
			return []domain.Deployment{{ID: "b1"}}, nil
		},
	}}
	e.GET("/projects/:id/builds", func(c *echo.Context) error {
		c.Set(middleware.UserIDContextKey, "11")
		return h.ListProjectBuilds(c)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/projects/p1/builds", http.NoBody)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestListProjectBuilds_NotFound(t *testing.T) {
	e := echo.New()
	h := &DeploymentHandler{dUsecase: &mockDeploymentUsecase{
		listProjectBuildsFn: func(_ context.Context, _, _ string) ([]domain.Deployment, error) {
			return nil, domain.ErrNotFound
		},
	}}
	e.GET("/projects/:id/builds", func(c *echo.Context) error {
		c.Set(middleware.UserIDContextKey, "11")
		return h.ListProjectBuilds(c)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/projects/p1/builds", http.NoBody)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("project not found or has no builds")) {
		t.Fatalf("missing expected error message: %s", rec.Body.String())
	}
}

func TestGetBuild_Unauthorized(t *testing.T) {
	e := echo.New()
	h := &DeploymentHandler{dUsecase: &mockDeploymentUsecase{}}
	e.GET("/builds/:id", h.GetBuild)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/builds/b1", http.NoBody)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestGetBuild_MissingBuildID(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/builds/", http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.UserIDContextKey, "11")
	c.SetPathValues(echo.PathValues{{Name: "id", Value: ""}})

	h := &DeploymentHandler{dUsecase: &mockDeploymentUsecase{}}
	if err := h.GetBuild(c); err != nil {
		t.Fatalf("expected nil echo error, got %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestGetBuild_PassesParams(t *testing.T) {
	e := echo.New()
	h := &DeploymentHandler{dUsecase: &mockDeploymentUsecase{
		getBuildFn: func(_ context.Context, userID, buildID string) (*domain.Deployment, error) {
			if userID != "11" || buildID != "b1" {
				t.Fatalf("unexpected args userID=%s buildID=%s", userID, buildID)
			}
			return &domain.Deployment{ID: "b1"}, nil
		},
	}}
	e.GET("/builds/:id", func(c *echo.Context) error {
		c.Set(middleware.UserIDContextKey, "11")
		return h.GetBuild(c)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/builds/b1", http.NoBody)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestGetBuild_NotFound(t *testing.T) {
	e := echo.New()
	h := &DeploymentHandler{dUsecase: &mockDeploymentUsecase{
		getBuildFn: func(_ context.Context, _, _ string) (*domain.Deployment, error) {
			return nil, domain.ErrNotFound
		},
	}}
	e.GET("/builds/:id", func(c *echo.Context) error {
		c.Set(middleware.UserIDContextKey, "11")
		return h.GetBuild(c)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/builds/b1", http.NoBody)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("build not found")) {
		t.Fatalf("missing expected error message: %s", rec.Body.String())
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
