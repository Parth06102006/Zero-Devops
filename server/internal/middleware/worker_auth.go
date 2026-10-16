package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/spf13/viper"
)

// WorkerAuthMiddleware protects service-to-service endpoints. It is separate
// from browser JWT authentication because workers have no user session.
func WorkerAuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		expected := viper.GetString("WORKER_INTERNAL_AUTH_TOKEN")
		provided := strings.TrimPrefix(c.Request().Header.Get("Authorization"), "Bearer ")
		if expected == "" || provided == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
			return c.JSON(http.StatusUnauthorized, map[string]string{errResponseKey: "worker authorization required"})
		}
		return next(c)
	}
}
