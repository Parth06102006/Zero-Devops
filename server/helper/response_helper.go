package helper

import "Zero_Devops/server/domain"

// BuildSuccessResponse creates a standardized success API response
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
