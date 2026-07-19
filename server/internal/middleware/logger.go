// Package middleware provides HTTP middleware handlers for the application
package middleware

import (
	"context"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"
)

type loggerCtxKey struct{}

// RequestLoggerMiddleware creates Echo middleware that attaches a request-scoped logger, annotated with the request ID, to each request context.
func RequestLoggerMiddleware(base *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			reqLogger := base.With(zap.String("request_id", GetRequestID(c)))
			ctx := context.WithValue(c.Request().Context(), loggerCtxKey{}, reqLogger)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

// LoggerFromContext retrieves the request-scoped logger from ctx, or the default logger when none is stored.
func LoggerFromContext(ctx context.Context) *zap.Logger {
	if l, ok := ctx.Value(loggerCtxKey{}).(*zap.Logger); ok {
		return l
	}
	return zap.L()
}
