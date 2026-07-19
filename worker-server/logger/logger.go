// Package logger provides logging utilities for the worker server.
package logger

import "go.uber.org/zap"

// New creates a new zap.Logger based on the environment setting.
func New(env string) (*zap.Logger, error) {
	if env == "production" {
		return zap.NewProduction()
	}
	return zap.NewDevelopment()
}
