package helper

import "Zero_Devops/server/internal/domain"

// BuildSuccessResponse creates a standardized successful API response and applies the provided options.
// It returns the response containing the supplied data and request ID.
func BuildSuccessResponse(data interface{}, _, reqID string, opts ...SuccessOption) domain.ResponseSuccess {
	resp := domain.ResponseSuccess{
		Success:   true,
		Data:      data,
		RequestID: reqID,
	}
	for _, opt := range opts {
		opt(&resp)
	}
	return resp
}

// SuccessOption configures the success response
type SuccessOption func(*domain.ResponseSuccess)

// WithMessage sets the message on a success response
func WithMessage(msg string) SuccessOption {
	return func(r *domain.ResponseSuccess) {
		r.Message = msg
	}
}
