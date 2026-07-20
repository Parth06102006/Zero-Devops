package http

import (
	"Zero_Devops/server/internal/domain"
	"Zero_Devops/server/internal/helper"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/spf13/viper"
)

type mockAuthUsecase struct {
	tokens   *domain.TokenResponse
	userResp *domain.UserResponse
	err      error
}

func (m *mockAuthUsecase) HandleOAuthCallback(_ context.Context, _, _ string) (*domain.TokenResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tokens, nil
}

func (m *mockAuthUsecase) RefreshToken(_ context.Context, _ string) (*domain.TokenResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tokens, nil
}

func (m *mockAuthUsecase) GetCurrentUser(_ context.Context, _ string) (domain.UserResponse, error) {
	if m.err != nil {
		return domain.UserResponse{}, m.err
	}
	return *m.userResp, nil
}

func (m *mockAuthUsecase) Logout(_ context.Context, _ string) error {
	return m.err
}

func setTestConfig() {
	viper.Set("JWT_SECRET", "test-secret-key")
	viper.Set("IS_PRODUCTION_ENV", false)
	viper.Set("ACCESS_TOKEN_EXPIRY", "1")
	viper.Set("REFRESH_TOKEN_EXPIRY", "720")
}

func TestLogin_MissingCode(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{},
	}

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?code=", http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Login(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestLogin_Success(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{
			tokens: &domain.TokenResponse{
				AccessToken:  "test-access-token",
				RefreshToken: "test-refresh-token",
			},
		},
	}

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?code=test-code", http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Login(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if len(rec.Result().Cookies()) != 2 {
		t.Errorf("expected 2 cookies, got %d", len(rec.Result().Cookies()))
	}
}

func TestLogin_UsecaseError(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{
			err: domain.ErrInternalServerError,
		},
	}

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?code=test-code", http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Login(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestRefresh_MissingCookie(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{},
	}

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Refresh(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestRefresh_Success(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{
			tokens: &domain.TokenResponse{
				AccessToken:  "new-access-token",
				RefreshToken: "new-refresh-token",
			},
		},
	}

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", http.NoBody)
	//nolint:gosec
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "test-refresh-token"})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Refresh(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestRefresh_UsecaseError(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{
			err: domain.ErrInvalidToken,
		},
	}

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", http.NoBody)
	//nolint:gosec
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "invalid-token"})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Refresh(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestLogout_MissingCookie(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{},
	}

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Logout(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestLogout_Success(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{},
	}

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", http.NoBody)
	//nolint:gosec
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "test-access-token"})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Logout(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var accessCookie *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "access_token" {
			accessCookie = cookie
			break
		}
	}
	if accessCookie == nil || accessCookie.MaxAge != -1 {
		t.Error("expected access_token cookie to be cleared")
	}
}

func TestLogout_UsecaseError(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{
			err: domain.ErrInvalidToken,
		},
	}

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", http.NoBody)
	//nolint:gosec
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "test-access-token"})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Logout(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestGetUser_MissingCookie(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{},
	}

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.GetUser(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestGetUser_Success(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{
			userResp: &domain.UserResponse{
				ID:        "1",
				Provider:  "github",
				Username:  "testuser",
				Email:     "test@example.com",
				AvatarURL: "https://example.com/avatar.png",
			},
		},
	}

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	//nolint:gosec
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "test-access-token"})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.GetUser(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "testuser") {
		t.Error("expected response to contain username")
	}
}

func TestGetUser_UsecaseError(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{
			err: domain.ErrNotFound,
		},
	}

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	//nolint:gosec
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "test-access-token"})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.GetUser(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestGetStatusCode(t *testing.T) {
	tests := []struct {
		err    error
		expect int
	}{
		{nil, http.StatusOK},
		{domain.ErrInternalServerError, http.StatusInternalServerError},
		{domain.ErrNotFound, http.StatusNotFound},
		{domain.ErrConflict, http.StatusConflict},
		{domain.ErrInvalidToken, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		result := helper.GetStatusCode(tt.err)
		if result != tt.expect {
			t.Errorf("expected %d, got %d for error %v", tt.expect, result, tt.err)
		}
	}
}
