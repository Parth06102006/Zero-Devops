package http

import (
	middleware "Zero_Devops/server/authorization/auth/delivery/http/middleware"
	"Zero_Devops/server/domain"
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo"
)

type mockDeploymentUsecase struct {
	createFn func(ctx context.Context, userID int64, repoID int64) (*domain.Deployment, error)
}

func (m *mockDeploymentUsecase) CreateDeployment(ctx context.Context, userID int64, repoID int64) (*domain.Deployment, error) {
	if m.createFn != nil {
		return m.createFn(ctx, userID, repoID)
	}
	return nil, nil
}

func (m *mockDeploymentUsecase) GetDeployments(ctx context.Context, userID int64) ([]domain.Deployment, error) {
	return nil, nil
}

func (m *mockDeploymentUsecase) GetDeploymentByID(ctx context.Context, userID, deploymentID int64) (*domain.Deployment, error) {
	return nil, nil
}

func TestCreateDeployment_Unauthorized(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/deployments", nil)
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
	req := httptest.NewRequest(http.MethodPost, "/deployments", bytes.NewBufferString("{"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.UserIDContextKey, int64(11))

	h := &DeploymentHandler{dUsecase: &mockDeploymentUsecase{}}
	if err := h.CreateDeployment(c); err != nil {
		t.Fatalf("expected nil echo error, got %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestCreateDeployment_Success(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/deployments", bytes.NewBufferString(`{"repo_id":42}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.UserIDContextKey, int64(11))

	want := &domain.Deployment{ID: 9, UserID: 11, RepoID: 42, Status: domain.DeploymentStatusPending}
	h := &DeploymentHandler{
		dUsecase: &mockDeploymentUsecase{
			createFn: func(ctx context.Context, userID int64, repoID int64) (*domain.Deployment, error) {
				if userID != 11 || repoID != 42 {
					t.Fatalf("unexpected args userID=%d repoID=%d", userID, repoID)
				}
				return want, nil
			},
		},
	}

	if err := h.CreateDeployment(c); err != nil {
		t.Fatalf("expected nil echo error, got %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}
}

func TestCreateDeployment_UsecaseError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/deployments", bytes.NewBufferString(`{"repo_id":42}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.UserIDContextKey, int64(11))

	h := &DeploymentHandler{
		dUsecase: &mockDeploymentUsecase{
			createFn: func(ctx context.Context, userID int64, repoID int64) (*domain.Deployment, error) {
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
			if got := getStatusCode(tc.err); got != tc.want {
				t.Fatalf("expected %d, got %d", tc.want, got)
			}
		})
	}
}
