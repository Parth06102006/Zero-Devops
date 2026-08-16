// Package middleware provides HTTP middleware handlers for the application
package middleware

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"
)

type loggerCtxKey struct{}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// RequestLoggerMiddleware creates Echo middleware that attaches a request-scoped logger, annotated with the request ID, to each request context.
func RequestLoggerMiddleware(base *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			start := time.Now()
			reqLogger := base.With(zap.String("request_id", GetRequestID(c)))
			ctx := context.WithValue(c.Request().Context(), loggerCtxKey{}, reqLogger)
			c.SetRequest(c.Request().WithContext(ctx))
			recorder := &statusRecorder{ResponseWriter: c.Response(), status: http.StatusOK}
			c.SetResponse(recorder)

			err := next(c)
			status := recorder.status
			var httpErr *echo.HTTPError
			if err != nil && errors.As(err, &httpErr) {
				status = httpErr.Code
			}
			fields := []zap.Field{
				zap.String("method", c.Request().Method),
				zap.String("path", c.Request().URL.Path),
				zap.Int("status", status),
				zap.Duration("latency", time.Since(start)),
				zap.String("remote_addr", c.Request().RemoteAddr),
			}
			if err != nil {
				fields = append(fields, zap.Error(err))
				reqLogger.Error("http request failed", fields...)
			} else {
				reqLogger.Info("http request completed", fields...)
			}

			return err
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
