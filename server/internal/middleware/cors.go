package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	echomw "github.com/labstack/echo/v5/middleware"
	"github.com/spf13/viper"
)

// NewCORS returns a CORS middleware configured with origins from the
// ALLOWED_ORIGINS environment variable.
func NewCORS() echo.MiddlewareFunc {
	allowedOrigins := allowedOriginsFromConfig()

	return echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins:     allowedOrigins,
		AllowCredentials: true,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodDelete},
	})
}

func allowedOriginsFromConfig() []string {
	switch origins := viper.Get("ALLOWED_ORIGINS").(type) {
	case string:
		return compactOrigins(strings.Split(origins, ","))
	case []string:
		return compactOrigins(origins)
	case []interface{}:
		values := make([]string, 0, len(origins))
		for _, origin := range origins {
			values = append(values, fmt.Sprint(origin))
		}
		return compactOrigins(values)
	default:
		return compactOrigins(viper.GetStringSlice("ALLOWED_ORIGINS"))
	}
}

func compactOrigins(origins []string) []string {
	compact := make([]string, 0, len(origins))
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			compact = append(compact, origin)
		}
	}
	return compact
}
