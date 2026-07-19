package middleware

import (
	"Zero_Devops/server/domain"
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

func (m *mockUserRepository) GetByID(ctx context.Context, id int64) (domain.User, error) {
	if m.err != nil {
		return domain.User{}, m.err
	}
	return m.user, nil
}

func (m *mockUserRepository) GetByUsername(ctx context.Context, username string) (domain.User, error) {
	return domain.User{}, domain.ErrNotFound
}

func (m *mockUserRepository) GetProviderByID(ctx context.Context, providerID int64) (domain.User, error) {
	return domain.User{}, domain.ErrNotFound
}

func (m *mockUserRepository) Store(ctx context.Context, u *domain.User) error {
	return nil
}

func (m *mockUserRepository) UpdateRefreshToken(ctx context.Context, id int64, refreshToken string) error {
	return nil
}

func setMiddlewareTestConfig() {
	viper.Set("JWT_SECRET", "test-secret-key-for-middleware")
}

func generateTestAccessToken(userID int64, exp time.Time) string {
	secretKey := []byte(viper.GetString("JWT_SECRET"))
	claims := jwt.MapClaims{
		"user_id": userID,
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
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
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
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())

	_, err := handler.Validator(c, "some-token")
	if err != domain.ErrMissingSecret {
		t.Errorf("expected ErrMissingSecret, got %v", err)
	}
}

func TestValidator_InvalidToken(t *testing.T) {
	setMiddlewareTestConfig()

	handler := &AuthMiddlewareHandler{}
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())

	_, err := handler.Validator(c, "invalid-token")
	if err != domain.ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestValidator_ExpiredToken(t *testing.T) {
	setMiddlewareTestConfig()

	handler := &AuthMiddlewareHandler{}
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())

	expiredToken := generateTestAccessToken(1, time.Now().Add(-1*time.Hour))

	_, err := handler.Validator(c, expiredToken)
	if err != domain.ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestValidator_ValidToken_NoUserRepo(t *testing.T) {
	setMiddlewareTestConfig()

	handler := &AuthMiddlewareHandler{userRepo: nil}
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())

	validToken := generateTestAccessToken(1, time.Now().Add(15*time.Minute))

	userID, err := handler.Validator(c, validToken)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if userID != 1 {
		t.Errorf("expected userID 1, got %d", userID)
	}
}

func TestValidator_ValidToken_WithUserRepo(t *testing.T) {
	setMiddlewareTestConfig()

	handler := &AuthMiddlewareHandler{
		userRepo: &mockUserRepository{
			user: domain.User{
				ID:       1,
				Username: "testuser",
			},
		},
	}
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())

	validToken := generateTestAccessToken(1, time.Now().Add(15*time.Minute))

	userID, err := handler.Validator(c, validToken)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if userID != 1 {
		t.Errorf("expected userID 1, got %d", userID)
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
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())

	validToken := generateTestAccessToken(1, time.Now().Add(15*time.Minute))

	_, err := handler.Validator(c, validToken)
	if err != domain.ErrUserLookupFailed {
		t.Errorf("expected ErrUserLookupFailed, got %v", err)
	}
}

func TestAuthMiddleware_SkipsAuthPaths(t *testing.T) {
	setMiddlewareTestConfig()

	handler := &AuthMiddlewareHandler{}
	nextCalled := false

	next := func(c *echo.Context) error {
		nextCalled = true
		return nil
	}

	middlewareFunc := handler.AuthMiddleware(next)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/auth/github/login", nil)
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

	next := func(c *echo.Context) error {
		nextCalled = true
		return nil
	}

	middlewareFunc := handler.AuthMiddleware(next)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/auth/user/me", nil)
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

	next := func(c *echo.Context) error {
		nextCalled = true
		return nil
	}

	middlewareFunc := handler.AuthMiddleware(next)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/auth/user/me", nil)
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

	next := func(c *echo.Context) error {
		nextCalled = true
		return nil
	}

	middlewareFunc := handler.AuthMiddleware(next)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/auth/user/me", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: generateTestAccessToken(1, time.Now().Add(15*time.Minute))})
	c := e.NewContext(req, httptest.NewRecorder())

	err := middlewareFunc(c)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if !nextCalled {
		t.Error("expected next handler to be called")
	}

	userID, ok := GetUserID(c)
	if !ok || userID != 1 {
		t.Errorf("expected userID 1 in context, got %d", userID)
	}
}

func TestGetUserID_NotSet(t *testing.T) {
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())

	_, ok := GetUserID(c)
	if ok {
		t.Error("expected false when user ID not set")
	}
}

func TestGetUserID_Set(t *testing.T) {
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
	c.Set("user_id", int64(123))

	userID, ok := GetUserID(c)
	if !ok {
		t.Error("expected true when user ID is set")
	}
	if userID != 123 {
		t.Errorf("expected userID 123, got %d", userID)
	}
}

func TestToMiddleware(t *testing.T) {
	handler := &AuthMiddlewareHandler{}

	middlewareFunc := handler.ToMiddleware()
	if middlewareFunc == nil {
		t.Error("expected non-nil middleware function")
	}
}
