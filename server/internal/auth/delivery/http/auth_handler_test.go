package http

import (
	"Zero_Devops/server/internal/domain"
	"Zero_Devops/server/internal/helper"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/spf13/viper"
)

type mockAuthUsecase struct {
	tokens         *domain.TokenResponse
	userResp       *domain.UserResponse
	err            error
	githubOAuthURL string
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

func (m *mockAuthUsecase) GithubOauthURL(_ context.Context, _ string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.githubOAuthURL, nil
}

func setTestConfig() {
	viper.Set("JWT_SECRET", "test-secret-key")
	viper.Set("APP_ENV", "development")
	viper.Set("ACCESS_TOKEN_EXPIRY", "1")
	viper.Set("REFRESH_TOKEN_EXPIRY", "720")
	viper.Set("FRONTEND_URL", "http://localhost:3000")
	viper.Set("COOKIE_DOMAIN", "")
}

func setTestConfigWithCookieDomain(t *testing.T, cookieDomain string) {
	t.Helper()
	setTestConfig()
	viper.Set("COOKIE_DOMAIN", cookieDomain)
	t.Cleanup(func() { viper.Set("COOKIE_DOMAIN", "") })
}

func TestLogin_DefaultReturnTo(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{
			githubOAuthURL: "https://github.com/login/oauth/authorize?state=test-state",
		},
	}

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/github/login", http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Login(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("expected status %d, got %d", http.StatusTemporaryRedirect, rec.Code)
	}

	location := rec.Header().Get("Location")
	if location != "https://github.com/login/oauth/authorize?state=test-state" {
		t.Errorf("expected redirect to GitHub OAuth URL, got %s", location)
	}
}

func TestLogin_Success(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{
			githubOAuthURL: "https://github.com/login/oauth/authorize?state=test-state",
		},
	}

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/github/login?return_to=/dashboard", http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Login(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("expected status %d, got %d", http.StatusTemporaryRedirect, rec.Code)
	}

	location := rec.Header().Get("Location")
	if location != "https://github.com/login/oauth/authorize?state=test-state" {
		t.Errorf("expected redirect to GitHub OAuth URL, got %s", location)
	}

	cookies := rec.Result().Cookies()
	stateCookie := findCookie(cookies, "gh_oauth_state")
	if stateCookie == nil {
		t.Fatal("expected gh_oauth_state cookie to be set")
	}
	if stateCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("expected gh_oauth_state cookie SameSite=Lax, got %v", stateCookie.SameSite)
	}
	if !stateCookie.Secure {
		t.Error("expected gh_oauth_state cookie to be Secure")
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
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/github/login", http.NoBody)
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

	accessCookie := findCookie(rec.Result().Cookies(), "access_token")
	if accessCookie == nil {
		t.Fatal("expected access_token cookie to be set")
	}
	if accessCookie.SameSite != http.SameSiteNoneMode {
		t.Errorf("expected access_token cookie SameSite=None, got %v", accessCookie.SameSite)
	}
	if !accessCookie.Secure {
		t.Error("expected access_token cookie to be Secure")
	}
	if accessCookie.Domain != "" {
		t.Errorf("expected host-only access_token cookie when COOKIE_DOMAIN is unset, got Domain=%q", accessCookie.Domain)
	}

	refreshCookie := findCookie(rec.Result().Cookies(), "refresh_token")
	if refreshCookie == nil {
		t.Fatal("expected refresh_token cookie to be set")
	}
	if refreshCookie.SameSite != http.SameSiteNoneMode {
		t.Errorf("expected refresh_token cookie SameSite=None, got %v", refreshCookie.SameSite)
	}
	if !refreshCookie.Secure {
		t.Error("expected refresh_token cookie to be Secure")
	}
	if refreshCookie.Domain != "" {
		t.Errorf("expected host-only refresh_token cookie when COOKIE_DOMAIN is unset, got Domain=%q", refreshCookie.Domain)
	}
}

func TestRefresh_WithCookieDomain(t *testing.T) {
	setTestConfigWithCookieDomain(t, ".parthgarg.me")

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

	accessCookie := findCookie(rec.Result().Cookies(), "access_token")
	if accessCookie == nil {
		t.Fatal("expected access_token cookie to be set")
	}
	// Go strips the leading dot when serializing Set-Cookie (cookie.go's
	// Cookie.String), so the parsed value reflects the wire format. A non-empty
	// Domain makes the cookie a domain cookie shared across subdomains.
	if accessCookie.Domain != "parthgarg.me" {
		t.Errorf("expected access_token cookie Domain=parthgarg.me, got %q", accessCookie.Domain)
	}

	refreshCookie := findCookie(rec.Result().Cookies(), "refresh_token")
	if refreshCookie == nil {
		t.Fatal("expected refresh_token cookie to be set")
	}
	if refreshCookie.Domain != "parthgarg.me" {
		t.Errorf("expected refresh_token cookie Domain=parthgarg.me, got %q", refreshCookie.Domain)
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

	refreshCookie := findCookie(rec.Result().Cookies(), "refresh_token")
	if refreshCookie == nil || refreshCookie.MaxAge != -1 {
		t.Error("expected refresh_token cookie to be cleared")
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

func validStateCookie(returnTo string) *http.Cookie {
	payload := oauthStatePayload{State: "test-state", ReturnTo: returnTo}
	b, _ := json.Marshal(payload)
	//nolint:gosec
	return &http.Cookie{Name: "gh_oauth_state", Value: base64.URLEncoding.EncodeToString(b), HttpOnly: true, SameSite: http.SameSiteLaxMode}
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func TestLoginCallback_MissingStateCookie(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{},
	}

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/github/login/callback?state=test-state&code=test-code", http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.LoginCallback(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestLoginCallback_InvalidBase64Cookie(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{},
	}

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/github/login/callback?state=test-state&code=test-code", http.NoBody)
	//nolint:gosec
	req.AddCookie(&http.Cookie{Name: "gh_oauth_state", Value: "!!!invalid-base64!!!"})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.LoginCallback(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestLoginCallback_InvalidJSONCookie(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{},
	}

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/github/login/callback?state=test-state&code=test-code", http.NoBody)
	//nolint:gosec
	req.AddCookie(&http.Cookie{Name: "gh_oauth_state", Value: base64.URLEncoding.EncodeToString([]byte("not-json"))})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.LoginCallback(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestLoginCallback_StateMismatch(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{},
	}

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/github/login/callback?state=wrong-state&code=test-code", http.NoBody)
	req.AddCookie(validStateCookie("/"))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.LoginCallback(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestLoginCallback_MissingCode(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{},
	}

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/github/login/callback?state=test-state", http.NoBody)
	req.AddCookie(validStateCookie("/"))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.LoginCallback(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestLoginCallback_Success(t *testing.T) {
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
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/github/login/callback?state=test-state&code=test-code", http.NoBody)
	req.AddCookie(validStateCookie("/dashboard"))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.LoginCallback(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("expected status %d, got %d", http.StatusTemporaryRedirect, rec.Code)
	}

	location := rec.Header().Get("Location")
	if location != "http://localhost:3000/dashboard" {
		t.Errorf("expected redirect to http://localhost:3000/dashboard, got %s", location)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 3 {
		t.Fatalf("expected 3 cookies (state cleared + access + refresh), got %d", len(cookies))
	}

	stateCookie := findCookie(cookies, "gh_oauth_state")
	if stateCookie == nil || stateCookie.MaxAge != -1 {
		t.Error("expected gh_oauth_state cookie to be cleared")
	}

	accessCookie := findCookie(cookies, "access_token")
	if accessCookie == nil {
		t.Fatal("expected access_token cookie to be set")
	}
	if accessCookie.SameSite != http.SameSiteNoneMode {
		t.Errorf("expected access_token cookie SameSite=None, got %v", accessCookie.SameSite)
	}
	if !accessCookie.Secure {
		t.Error("expected access_token cookie to be Secure")
	}

	refreshCookie := findCookie(cookies, "refresh_token")
	if refreshCookie == nil {
		t.Fatal("expected refresh_token cookie to be set")
	}
	if refreshCookie.SameSite != http.SameSiteNoneMode {
		t.Errorf("expected refresh_token cookie SameSite=None, got %v", refreshCookie.SameSite)
	}
	if !refreshCookie.Secure {
		t.Error("expected refresh_token cookie to be Secure")
	}
}

func TestLoginCallback_OpenRedirectBlocked(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{
			tokens: &domain.TokenResponse{
				AccessToken:  "test-access-token",
				RefreshToken: "test-refresh-token",
			},
		},
	}

	tests := []struct {
		name     string
		returnTo string
	}{
		{name: "absolute URL", returnTo: "https://evil.example"},
		{name: "network-path reference", returnTo: "//evil.example"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/github/login/callback?state=test-state&code=test-code", http.NoBody)
			req.AddCookie(validStateCookie(tt.returnTo))
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			err := handler.LoginCallback(c)
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}

			if rec.Code != http.StatusTemporaryRedirect {
				t.Errorf("expected status %d, got %d", http.StatusTemporaryRedirect, rec.Code)
			}

			location := rec.Header().Get("Location")
			if location != "http://localhost:3000/" {
				t.Errorf("expected redirect to http://localhost:3000/, got %s", location)
			}
		})
	}
}

func TestLoginCallback_UsecaseError(t *testing.T) {
	setTestConfig()

	handler := &AuthHandler{
		AUsecase: &mockAuthUsecase{
			err: domain.ErrInternalServerError,
		},
	}

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/github/login/callback?state=test-state&code=test-code", http.NoBody)
	req.AddCookie(validStateCookie("/"))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.LoginCallback(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
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
