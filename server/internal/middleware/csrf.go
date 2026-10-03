package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// mutatingMethods are the HTTP methods that change server state.  All of them
// require origin validation when the app is running in production.
var mutatingMethods = map[string]bool{
	http.MethodPost:   true,
	http.MethodPut:    true,
	http.MethodPatch:  true,
	http.MethodDelete: true,
}

// NewCSRFOriginMiddleware returns a middleware that protects mutating endpoints
// against Cross-Site Request Forgery (CSRF) by validating the request Origin
// (or Referer as a fallback) against the list of allowed origins from config.
//
// Why this is needed:
//
//	Production cookies are set with SameSite=None so they travel between.SameSite=None
//	removes the browser-level cross-site cookie restriction, which means any
//	website on the internet can trigger a credentialed request to the server.
//	This middleware closes that gap by explicitly verifying where the request
//	came from before allowing state-changing operations.
//
// How browsers help:
//
//	Browsers always add the Origin header on cross-origin fetch/XHR requests
//	and on all non-GET/HEAD form submissions.  JavaScript running on a third-
//	party site cannot forge or omit this header — it is set by the browser.
//
// Skipped when:
//   - The request uses a non-mutating method (GET, HEAD, OPTIONS, TRACE).
//   - COOKIE_DOMAIN is empty (local development on localhost): in that env
//     SameSite=Lax is used and CSRF is not a concern.
//   - The Origin header is absent and no Referer is present (e.g. server-to-
//     server calls that carry no credential cookies).
func NewCSRFOriginMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			// Skip entirely in local development: cookies are SameSite=Lax
			// (host-only) and there is no cross-origin risk.
			if viper.GetString("COOKIE_DOMAIN") == "" {
				return next(c)
			}

			// Only validate mutating methods.
			if !mutatingMethods[c.Request().Method] {
				return next(c)
			}

			origin := c.Request().Header.Get("Origin")

			// If Origin is absent, fall back to the Referer header (same
			// scheme+host prefix check).
			if origin == "" {
				ref := c.Request().Header.Get("Referer")
				if ref == "" {
					// No Origin and no Referer — most likely a server-to-server
					// call without a session cookie.  Pass through; the auth
					// middleware will still enforce authentication.
					return next(c)
				}
				parsed, err := url.Parse(ref)
				if err != nil {
					return c.JSON(http.StatusForbidden, map[string]string{
						"error": "invalid Referer header",
					})
				}
				origin = parsed.Scheme + "://" + parsed.Host
			}

			if !isAllowedOrigin(origin) {
				log := LoggerFromContext(c.Request().Context())
				log.Warn("CSRF origin check failed",
					zap.String("method", c.Request().Method),
					zap.String("path", c.Request().URL.Path),
					zap.String("origin", origin),
				)
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": "request origin not allowed",
				})
			}

			return next(c)
		}
	}
}

// isAllowedOrigin checks whether the given origin string is present in the
// ALLOWED_ORIGINS config value (comma-separated list).
func isAllowedOrigin(origin string) bool {
	origin = strings.TrimSpace(origin)
	for _, allowed := range allowedOriginsFromConfig() {
		if strings.EqualFold(strings.TrimSpace(allowed), origin) {
			return true
		}
	}
	return false
}
