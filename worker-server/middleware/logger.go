// Package middleware provides middleware utilities for the worker server.
package middleware

import (
	"context"

	"go.uber.org/zap"
)

type loggerCtxKey struct{}

// WithLogger stores a zap.Logger in the context.
func WithLogger(ctx context.Context, l *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey{}, l)
}

// LoggerFromContext retrieves a zap.Logger from the context, or returns the global logger.
func LoggerFromContext(ctx context.Context) *zap.Logger {
	if l, ok := ctx.Value(loggerCtxKey{}).(*zap.Logger); ok {
		return l
	}
	return zap.L()
}
