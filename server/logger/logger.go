// Package logger provides logging utilities for the application
package logger

import "go.uber.org/zap"

// New creates a new zap.Logger based on the environment
func New(env string) *zap.Logger {
	if env == "production" {
		l, _ := zap.NewProduction()
		return l
	}
	l, _ := zap.NewDevelopment()
	return l
}
