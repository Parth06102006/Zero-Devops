package middleware

import (
	"Zero_Devops/server/domain"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo"
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

func (m *mockUserRepository) GetProviderById(ctx context.Context, providerId int64) (domain.User, error) {
	return domain.User{}, domain.ErrNotFound
}

func (m *mockUserRepository) Store(ctx context.Context, u *domain.User) error {
	return nil
}

func (m *mockUserRepository) UpdateRefreshToken(ctx context.Context, id int64, refreshToken string) error {
	return nil
}

type middlewareTestContext struct {
	echo.Context
	rec       *httptest.ResponseRecorder
	req       *http.Request
	cookies   map[string]*http.Cookie
	pathValue string
	values    map[string]interface{}
}

func newMiddlewareTestContext() *middlewareTestContext {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	return &middlewareTestContext{
		Context:   echo.New().NewContext(req, rec),
		rec:       rec,
		req:       req,
		cookies:   make(map[string]*http.Cookie),
		pathValue: "/test",
		values:    make(map[string]interface{}),
	}
}

func (t *middlewareTestContext) Cookie(name string) (*http.Cookie, error) {
	if cookie, ok := t.cookies[name]; ok {
		return cookie, nil
	}
	return nil, http.ErrNoCookie
}

func (t *middlewareTestContext) SetCookie(cookie *http.Cookie) {
	t.cookies[cookie.Name] = cookie
}

func (t *middlewareTestContext) Path() string {
	return t.pathValue
}

func (t *middlewareTestContext) Set(key string, value interface{}) {
	t.values[key] = value
}

func (t *middlewareTestContext) Get(key string) interface{} {
	return t.values[key]
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

	for _, tt := range tests {
		ctx := newMiddlewareTestContext()
		ctx.pathValue = tt.path
		result := handler.Skipper(ctx)
		if result != tt.expected {
			t.Errorf("expected %v for path %s, got %v", tt.expected, tt.path, result)
		}
	}
}

func TestValidator_MissingSecret(t *testing.T) {
	viper.Set("JWT_SECRET", "")

	handler := &AuthMiddlewareHandler{}
	ctx := newMiddlewareTestContext()

	_, err := handler.Validator(ctx, "some-token")
	if err != domain.ErrMissingSecret {
		t.Errorf("expected ErrMissingSecret, got %v", err)
	}
}

func TestValidator_InvalidToken(t *testing.T) {
	setMiddlewareTestConfig()

	handler := &AuthMiddlewareHandler{}
	ctx := newMiddlewareTestContext()

	_, err := handler.Validator(ctx, "invalid-token")
	if err != domain.ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestValidator_ExpiredToken(t *testing.T) {
	setMiddlewareTestConfig()

	handler := &AuthMiddlewareHandler{}
	ctx := newMiddlewareTestContext()

	expiredToken := generateTestAccessToken(1, time.Now().Add(-1*time.Hour))

	_, err := handler.Validator(ctx, expiredToken)
	if err != domain.ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestValidator_ValidToken_NoUserRepo(t *testing.T) {
	setMiddlewareTestConfig()

	handler := &AuthMiddlewareHandler{userRepo: nil}
	ctx := newMiddlewareTestContext()

	validToken := generateTestAccessToken(1, time.Now().Add(15*time.Minute))

	userID, err := handler.Validator(ctx, validToken)
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
	ctx := newMiddlewareTestContext()

	validToken := generateTestAccessToken(1, time.Now().Add(15*time.Minute))

	userID, err := handler.Validator(ctx, validToken)
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
	ctx := newMiddlewareTestContext()

	validToken := generateTestAccessToken(1, time.Now().Add(15*time.Minute))

	_, err := handler.Validator(ctx, validToken)
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestAuthMiddleware_SkipsAuthPaths(t *testing.T) {
	setMiddlewareTestConfig()

	handler := &AuthMiddlewareHandler{}
	nextCalled := false

	next := func(c echo.Context) error {
		nextCalled = true
		return nil
	}

	middlewareFunc := handler.AuthMiddleware(next)

	ctx := newMiddlewareTestContext()
	ctx.pathValue = "/auth/github/login"

	err := middlewareFunc(ctx)
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

	next := func(c echo.Context) error {
		nextCalled = true
		return nil
	}

	middlewareFunc := handler.AuthMiddleware(next)

	ctx := newMiddlewareTestContext()
	ctx.pathValue = "/auth/user/me"

	err := middlewareFunc(ctx)
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

	next := func(c echo.Context) error {
		nextCalled = true
		return nil
	}

	middlewareFunc := handler.AuthMiddleware(next)

	ctx := newMiddlewareTestContext()
	ctx.pathValue = "/auth/user/me"
	ctx.cookies["access_token"] = &http.Cookie{Name: "access_token", Value: "invalid-token"}

	err := middlewareFunc(ctx)
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

	next := func(c echo.Context) error {
		nextCalled = true
		return nil
	}

	middlewareFunc := handler.AuthMiddleware(next)

	ctx := newMiddlewareTestContext()
	ctx.pathValue = "/auth/user/me"
	ctx.cookies["access_token"] = &http.Cookie{Name: "access_token", Value: generateTestAccessToken(1, time.Now().Add(15*time.Minute))}

	err := middlewareFunc(ctx)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if !nextCalled {
		t.Error("expected next handler to be called")
	}

	userID, ok := GetUserID(ctx)
	if !ok || userID != 1 {
		t.Errorf("expected userID 1 in context, got %d", userID)
	}
}

func TestGetUserID_NotSet(t *testing.T) {
	ctx := newMiddlewareTestContext()

	_, ok := GetUserID(ctx)
	if ok {
		t.Error("expected false when user ID not set")
	}
}

func TestGetUserID_Set(t *testing.T) {
	ctx := newMiddlewareTestContext()
	ctx.Set("user_id", int64(123))

	userID, ok := GetUserID(ctx)
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
