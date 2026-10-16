package middleware

import (
	"path"
	"strings"

	"github.com/labstack/echo/v5"
)

// CleanPathMiddleware normalizes request URL paths by collapsing consecutive slashes (e.g., //webhooks/github -> /webhooks/github).
// It should be registered with e.Pre() so path normalization happens before route matching.
func CleanPathMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		req := c.Request()
		if req != nil && req.URL != nil && req.URL.Path != "" {
			cleaned := path.Clean(req.URL.Path)
			if strings.HasSuffix(req.URL.Path, "/") && cleaned != "/" {
				cleaned += "/"
			}
			req.URL.Path = cleaned
			if req.URL.RawPath != "" {
				rawCleaned := path.Clean(req.URL.RawPath)
				if strings.HasSuffix(req.URL.RawPath, "/") && rawCleaned != "/" {
					rawCleaned += "/"
				}
				req.URL.RawPath = rawCleaned
			}
		}
		return next(c)
	}
}
