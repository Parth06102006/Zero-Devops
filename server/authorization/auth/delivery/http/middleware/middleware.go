package middleware

import (
	"Zero_Devops/server/domain"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo"
	"github.com/spf13/viper"
)

const (
	UserIDContextKey = "user_id"
)

type AuthMiddlewareHandler struct {
	userRepo domain.UserRepository
}

func NewAuthMiddlewareHandler(userRepo domain.UserRepository) *AuthMiddlewareHandler {
	return &AuthMiddlewareHandler{
		userRepo: userRepo,
	}
}

func (a *AuthMiddlewareHandler) Validator(c echo.Context, accessToken string) (int64, error) {
	secretKey := viper.GetString("JWT_SECRET")
	if secretKey == "" {
		return 0, domain.ErrMissingSecret
	}

	token, err := jwt.Parse(accessToken, func(token *jwt.Token) (any, error) {
		return []byte(secretKey), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || token == nil || !token.Valid {
		return 0, domain.ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, domain.ErrInvalidToken
	}

	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return 0, domain.ErrInvalidToken
	}
	userID := int64(userIDFloat)

	if a.userRepo != nil {
		existingUser, err := a.userRepo.GetByID(c.Request().Context(), userID)
		if err != nil {
			return 0, err
		}
		if existingUser.ID == 0 {
			return 0, domain.ErrInvalidToken
		}
	}

	return userID, nil
}

func (a *AuthMiddlewareHandler) Skipper(c echo.Context) bool {
	path := c.Path()
	return path == "/auth/github/login" || path == "/auth/refresh" || path == "/integration/scm/github/webhook"
}

func (a *AuthMiddlewareHandler) AuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if a.Skipper(c) {
			return next(c)
		}

		cookie, err := c.Cookie("access_token")
		if err != nil || cookie.Value == "" {
			return echo.NewHTTPError(http.StatusUnauthorized, "access token cookie not found")
		}

		userID, err := a.Validator(c, cookie.Value)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid access token")
		}

		c.Set(UserIDContextKey, userID)
		return next(c)
	}
}

func (a *AuthMiddlewareHandler) ToMiddleware() echo.MiddlewareFunc {
	return a.AuthMiddleware
}

func GetUserID(c echo.Context) (int64, bool) {
	userID, ok := c.Get(UserIDContextKey).(int64)
	return userID, ok
}
