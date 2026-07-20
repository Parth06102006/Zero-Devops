package middleware

import (
	"Zero_Devops/server/internal/domain"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
	"github.com/spf13/viper"
)

type mockUserRepository struct {
	user domain.User
	err  error
}

func (m *mockUserRepository) GetByID(_ context.Context, _ string) (domain.User, error) {
	if m.err != nil {
		return domain.User{}, m.err
	}
	return m.user, nil
}

func (m *mockUserRepository) GetByUsername(_ context.Context, _ string) (domain.User, error) {
	return domain.User{}, domain.ErrNotFound
}

func (m *mockUserRepository) GetProviderByID(_ context.Context, _ int64) (domain.User, error) {
	return domain.User{}, domain.ErrNotFound
}

func (m *mockUserRepository) Store(_ context.Context, _ *domain.User) error {
	return nil
}

func (m *mockUserRepository) UpdateRefreshToken(_ context.Context, _, _ string) error {
	return nil
}

func setMiddlewareTestConfig() {
	viper.Set("JWT_SECRET", "test-secret-key-for-middleware")
}

func generateTestAccessToken(exp time.Time) string {
	secretKey := []byte(viper.GetString("JWT_SECRET"))
	claims := jwt.MapClaims{
		"user_id": "1",
		"email":   "test@example.com",
		"exp":     exp.Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, _ := token.SignedString(secretKey)
	return signedToken
}

func TestSkipper(t *testing.T) {
	setMiddlewareTestConfig()

	handler := &AuthMiddlewareHandler{}

	tests := []struct {
		path     string
		expected bool
	}{
		{"/auth/github/login", true},
		{"/auth/refresh", true},
		{"/auth/user/me", false},
		{"/other/path", false},
	}

	e := echo.New()
	for _, tt := range tests {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tt.path, http.NoBody)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPath(tt.path)

		result := handler.Skipper(c)
		if result != tt.expected {
			t.Errorf("expected %v for path %s, got %v", tt.expected, tt.path, result)
		}
	}
}

func TestValidator_MissingSecret(t *testing.T) {
	viper.Set("JWT_SECRET", "")

	handler := &AuthMiddlewareHandler{}
	e := echo.New()
	c := e.NewContext(httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody), httptest.NewRecorder())

	_, err := handler.Validator(c, "some-token")
	if err != domain.ErrMissingSecret {
		t.Errorf("expected ErrMissingSecret, got %v", err)
	}
}

func TestValidator_InvalidToken(t *testing.T) {
	setMiddlewareTestConfig()

	handler := &AuthMiddlewareHandler{}
	e := echo.New()
	c := e.NewContext(httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody), httptest.NewRecorder())

	_, err := handler.Validator(c, "invalid-token")
	if err != domain.ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestValidator_ExpiredToken(t *testing.T) {
	setMiddlewareTestConfig()

	handler := &AuthMiddlewareHandler{}
	e := echo.New()
	c := e.NewContext(httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody), httptest.NewRecorder())

	expiredToken := generateTestAccessToken(time.Now().Add(-1 * time.Hour))

	_, err := handler.Validator(c, expiredToken)
	if err != domain.ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestValidator_ValidToken_NoUserRepo(t *testing.T) {
	setMiddlewareTestConfig()

	handler := &AuthMiddlewareHandler{userRepo: nil}
	e := echo.New()
	c := e.NewContext(httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody), httptest.NewRecorder())

	validToken := generateTestAccessToken(time.Now().Add(15 * time.Minute))

	userID, err := handler.Validator(c, validToken)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if userID != "1" {
		t.Errorf("expected userID 1, got %s", userID)
	}
}

func TestValidator_ValidToken_WithUserRepo(t *testing.T) {
	setMiddlewareTestConfig()

	handler := &AuthMiddlewareHandler{
		userRepo: &mockUserRepository{
			user: domain.User{
				ID:       "1",
				Username: "testuser",
			},
		},
	}
	e := echo.New()
	c := e.NewContext(httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody), httptest.NewRecorder())

	validToken := generateTestAccessToken(time.Now().Add(15 * time.Minute))

	userID, err := handler.Validator(c, validToken)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if userID != "1" {
		t.Errorf("expected userID 1, got %s", userID)
	}
}

func TestValidator_ValidToken_UserNotFound(t *testing.T) {
	setMiddlewareTestConfig()

	handler := &AuthMiddlewareHandler{
		userRepo: &mockUserRepository{
			err: domain.ErrNotFound,
		},
	}
	e := echo.New()
	c := e.NewContext(httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody), httptest.NewRecorder())

	validToken := generateTestAccessToken(time.Now().Add(15 * time.Minute))

	_, err := handler.Validator(c, validToken)
	if err != domain.ErrUserLookupFailed {
		t.Errorf("expected ErrUserLookupFailed, got %v", err)
	}
}

func TestAuthMiddleware_SkipsAuthPaths(t *testing.T) {
	setMiddlewareTestConfig()

	handler := &AuthMiddlewareHandler{}
	nextCalled := false

	next := func(_ *echo.Context) error {
		nextCalled = true
		return nil
	}

	middlewareFunc := handler.AuthMiddleware(next)

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/github/login", http.NoBody)
	c := e.NewContext(req, httptest.NewRecorder())
	c.SetPath("/auth/github/login")

	err := middlewareFunc(c)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !nextCalled {
		t.Error("expected next handler to be called for skipped path")
	}
}

func TestAuthMiddleware_MissingCookie(t *testing.T) {
	setMiddlewareTestConfig()

	handler := &AuthMiddlewareHandler{}
	nextCalled := false

	next := func(_ *echo.Context) error {
		nextCalled = true
		return nil
	}

	middlewareFunc := handler.AuthMiddleware(next)

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/user/me", http.NoBody)
	c := e.NewContext(req, httptest.NewRecorder())

	err := middlewareFunc(c)
	if err == nil {
		t.Error("expected error for missing cookie")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok || httpErr.Code != http.StatusUnauthorized {
		t.Errorf("expected unauthorized error, got %v", err)
	}

	if nextCalled {
		t.Error("expected next handler to not be called")
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	setMiddlewareTestConfig()

	handler := &AuthMiddlewareHandler{}
	nextCalled := false

	next := func(_ *echo.Context) error {
		nextCalled = true
		return nil
	}

	middlewareFunc := handler.AuthMiddleware(next)

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/user/me", http.NoBody)
	//nolint:gosec
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "invalid-token"})
	c := e.NewContext(req, httptest.NewRecorder())

	err := middlewareFunc(c)
	if err == nil {
		t.Error("expected error for invalid token")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok || httpErr.Code != http.StatusUnauthorized {
		t.Errorf("expected unauthorized error, got %v", err)
	}

	if nextCalled {
		t.Error("expected next handler to not be called")
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	setMiddlewareTestConfig()

	handler := &AuthMiddlewareHandler{}
	nextCalled := false

	next := func(_ *echo.Context) error {
		nextCalled = true
		return nil
	}

	middlewareFunc := handler.AuthMiddleware(next)

	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/user/me", http.NoBody)
	//nolint:gosec
	req.AddCookie(&http.Cookie{Name: "access_token", Value: generateTestAccessToken(time.Now().Add(15 * time.Minute))})
	c := e.NewContext(req, httptest.NewRecorder())

	err := middlewareFunc(c)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if !nextCalled {
		t.Error("expected next handler to be called")
	}

	userID, ok := GetUserID(c)
	if !ok || userID != "1" {
		t.Errorf("expected userID 1 in context, got %s", userID)
	}
}

func TestGetUserID_NotSet(t *testing.T) {
	e := echo.New()
	c := e.NewContext(httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody), httptest.NewRecorder())

	_, ok := GetUserID(c)
	if ok {
		t.Error("expected false when user ID not set")
	}
}

func TestGetUserID_Set(t *testing.T) {
	e := echo.New()
	c := e.NewContext(httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody), httptest.NewRecorder())
	c.Set("user_id", "123")

	userID, ok := GetUserID(c)
	if !ok {
		t.Error("expected true when user ID is set")
	}
	if userID != "123" {
		t.Errorf("expected userID 123, got %s", userID)
	}
}

func TestToMiddleware(t *testing.T) {
	handler := &AuthMiddlewareHandler{}

	middlewareFunc := handler.ToMiddleware()
	if middlewareFunc == nil {
		t.Error("expected non-nil middleware function")
	}
}
