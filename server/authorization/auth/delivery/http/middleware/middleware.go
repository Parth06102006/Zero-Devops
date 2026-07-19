// Package middleware provides authentication middleware for HTTP handlers
package middleware

import (
	"Zero_Devops/server/domain"
	"context"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
	"github.com/spf13/viper"
)

// UserIDContextKey is the context key used to store the authenticated user ID
const UserIDContextKey = "user_id"

// SkipperFunc defines a function that skips authentication for a given context
type SkipperFunc func(*echo.Context) bool

// AuthMiddlewareHandler provides JWT-based authentication middleware
type AuthMiddlewareHandler struct {
	userRepo domain.UserRepository
}

// NewAuthMiddlewareHandler creates a new AuthMiddlewareHandler
func NewAuthMiddlewareHandler(repo domain.UserRepository) *AuthMiddlewareHandler {
	return &AuthMiddlewareHandler{
		userRepo: repo,
	}
}

// Validator validates a JWT token and returns the user ID
func (a *AuthMiddlewareHandler) Validator(c *echo.Context, token string) (int64, error) {
	secretKey := viper.GetString("JWT_SECRET")
	if secretKey == "" {
		return 0, domain.ErrMissingSecret
	}

	if token == "" {
		return 0, domain.ErrInvalidToken
	}

	parsedToken, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, domain.ErrInvalidToken
		}
		return []byte(secretKey), nil
	})

	if err != nil || !parsedToken.Valid {
		return 0, domain.ErrInvalidToken
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return 0, domain.ErrInvalidToken
	}

	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return 0, domain.ErrInvalidToken
	}
	userID := int64(userIDFloat)

	if a.userRepo != nil {
		_, err := a.userRepo.GetByID(c.Request().Context(), userID)
		if err != nil {
			return 0, domain.ErrUserLookupFailed
		}
	}

	return userID, nil
}

func isPublicPath(c *echo.Context) bool {
	switch c.Path() {
	case "/auth/github/login",
		"/auth/refresh":
		return true
	}
	return false
}

// Skipper determines if a request should skip authentication
func (a *AuthMiddlewareHandler) Skipper(c *echo.Context) bool {
	return isPublicPath(c)
}

// AuthMiddleware returns an HTTP middleware that enforces JWT authentication
func (a *AuthMiddlewareHandler) AuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if a.Skipper(c) {
			return next(c)
		}

		accessToken := ""
		if cookie, err := c.Cookie("access_token"); err == nil {
			accessToken = cookie.Value
		}

		userID, err := a.Validator(c, accessToken)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
		}

		c.Set(UserIDContextKey, userID)
		return next(c)
	}
}

// ToMiddleware converts the AuthMiddlewareHandler into an Echo middleware function
func (a *AuthMiddlewareHandler) ToMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return a.AuthMiddleware(next)
	}
}

// GetUserID retrieves the authenticated user ID from the echo context
func GetUserID(c *echo.Context) (int64, bool) {
	userID, ok := c.Get(UserIDContextKey).(int64)
	return userID, ok
}

// GetUserIDFromContext retrieves the authenticated user ID from a standard context
func GetUserIDFromContext(ctx context.Context) (int64, bool) {
	userID, ok := ctx.Value(UserIDContextKey).(int64)
	return userID, ok
}
