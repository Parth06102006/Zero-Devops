package http

import (
	"Zero_Devops/server/domain"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo"
	"github.com/spf13/viper"
)

type mockAuthUsecase struct {
	tokens   *domain.TokenResponse
	userResp *domain.UserResponse
	err      error
}

func (m *mockAuthUsecase) HandleOAuthCallback(ctx context.Context, code string, provider string) (*domain.TokenResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tokens, nil
}

func (m *mockAuthUsecase) RefreshToken(ctx context.Context, refreshToken string) (*domain.TokenResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tokens, nil
}

func (m *mockAuthUsecase) GetCurrentUser(ctx context.Context, accessToken string) (domain.UserResponse, error) {
	if m.err != nil {
		return domain.UserResponse{}, m.err
	}
	return *m.userResp, nil
}

func (m *mockAuthUsecase) Logout(ctx context.Context, accessToken string) error {
	return m.err
}

type testContext struct {
	echo.Context
	req        *http.Request
	rec        *httptest.ResponseRecorder
	cookies    map[string]*http.Cookie
	queryParams map[string]string
}

func newTestContext() *testContext {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	return &testContext{
		Context:    echo.New().NewContext(req, rec),
		req:        req,
		rec:        rec,
		cookies:    make(map[string]*http.Cookie),
		queryParams: make(map[string]string),
	}
}

func (t *testContext) Cookie(name string) (*http.Cookie, error) {
	if cookie, ok := t.cookies[name]; ok {
		return cookie, nil
	}
	return nil, http.ErrNoCookie
}

func (t *testContext) SetCookie(cookie *http.Cookie) {
	t.cookies[cookie.Name] = cookie
}

func (t *testContext) QueryParam(name string) string {
	return t.queryParams[name]
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

	c := newTestContext()
	c.queryParams["code"] = ""

	err := handler.Login(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if c.rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, c.rec.Code)
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

	c := newTestContext()
	c.queryParams["code"] = "test-code"

	err := handler.Login(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if c.rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, c.rec.Code)
	}

	if len(c.cookies) != 2 {
		t.Errorf("expected 2 cookies, got %d", len(c.cookies))
	}
}

func TestLogin_UsecaseError(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{
			err: domain.ErrInternalServerError,
		},
	}

	c := newTestContext()
	c.queryParams["code"] = "test-code"

	err := handler.Login(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if c.rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, c.rec.Code)
	}
}

func TestRefresh_MissingCookie(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{},
	}

	c := newTestContext()

	err := handler.Refresh(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if c.rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, c.rec.Code)
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

	c := newTestContext()
	c.cookies["refresh_token"] = &http.Cookie{Name: "refresh_token", Value: "test-refresh-token"}

	err := handler.Refresh(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if c.rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, c.rec.Code)
	}
}

func TestRefresh_UsecaseError(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{
			err: domain.ErrInvalidToken,
		},
	}

	c := newTestContext()
	c.cookies["refresh_token"] = &http.Cookie{Name: "refresh_token", Value: "invalid-token"}

	err := handler.Refresh(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if c.rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, c.rec.Code)
	}
}

func TestLogout_MissingCookie(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{},
	}

	c := newTestContext()

	err := handler.Logout(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if c.rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, c.rec.Code)
	}
}

func TestLogout_Success(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{},
	}

	c := newTestContext()
	c.cookies["access_token"] = &http.Cookie{Name: "access_token", Value: "test-access-token"}

	err := handler.Logout(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if c.rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, c.rec.Code)
	}

	if accessCookie, ok := c.cookies["access_token"]; !ok || accessCookie.MaxAge != -1 {
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

	c := newTestContext()
	c.cookies["access_token"] = &http.Cookie{Name: "access_token", Value: "test-access-token"}

	err := handler.Logout(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if c.rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, c.rec.Code)
	}
}

func TestGetUser_MissingCookie(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{},
	}

	c := newTestContext()

	err := handler.GetUser(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if c.rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, c.rec.Code)
	}
}

func TestGetUser_Success(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{
			userResp: &domain.UserResponse{
				ID:       1,
				Provider: "github",
				Username: "testuser",
				Email:    "test@example.com",
				AvatarURL: "https://example.com/avatar.png",
			},
		},
	}

	c := newTestContext()
	c.cookies["access_token"] = &http.Cookie{Name: "access_token", Value: "test-access-token"}

	err := handler.GetUser(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if c.rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, c.rec.Code)
	}

	if !strings.Contains(c.rec.Body.String(), "testuser") {
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

	c := newTestContext()
	c.cookies["access_token"] = &http.Cookie{Name: "access_token", Value: "test-access-token"}

	err := handler.GetUser(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if c.rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, c.rec.Code)
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
		result := getStatusCode(tt.err)
		if result != tt.expect {
			t.Errorf("expected %d, got %d for error %v", tt.expect, result, tt.err)
		}
	}
}