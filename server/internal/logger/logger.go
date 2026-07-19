// Package logger provides logging utilities for the application
package logger

import "go.uber.org/zap"

// New creates a zap.Logger configured for production when env is "production" and for
// development otherwise.
func New(env string) *zap.Logger {
	if env == "production" {
		l, _ := zap.NewProduction()
		return l
	}
	l, _ := zap.NewDevelopment()
	return l
}
