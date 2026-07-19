// Package helper provides utility functions for building API responses
package helper

import (
	"Zero_Devops/server/domain"
	"net/http"
	rtdebug "runtime/debug"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// BuildErrorResponse creates a standardized error API response
func BuildErrorResponse(message string, err error, reqID string, opts ...DebugOption) domain.ErrorResponse {
	resp := domain.ErrorResponse{
		Success: false,
		Error: domain.ErrorBody{
			Code:    GetStatusCode(err),
			Message: message,
		},
		RequestID: reqID,
	}

	if viper.GetString("APP_ENV") != "production" {
		debug := &domain.DebugInfo{
			RawError: err.Error(),
			Stack:    string(rtdebug.Stack()),
		}
		for _, opt := range opts {
			opt(debug)
		}
		resp.Error.Debug = debug
	}

	return resp
}

// DebugOption configures the debug info in error responses
type DebugOption func(*domain.DebugInfo)

// WithReason adds a reason to the debug info
func WithReason(r string) DebugOption {
	return func(d *domain.DebugInfo) { d.Reason = r }
}

// WithQuery adds a query to the debug info
func WithQuery(q string) DebugOption {
	return func(d *domain.DebugInfo) { d.Query = q }
}

// GetStatusCode maps an error to an HTTP status code
func GetStatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}

	zap.L().Error("An error occurred", zap.Error(err))
	switch err {
	case domain.ErrInternalServerError:
		return http.StatusInternalServerError
	case domain.ErrNotFound:
		return http.StatusNotFound
	case domain.ErrConflict:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
