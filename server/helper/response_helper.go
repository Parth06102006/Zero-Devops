package helper

import "Zero_Devops/server/domain"

func BuildSuccessResponse(data interface{}, message string, reqID string , opts ...SuccessOption) domain.ResponseSuccess {
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

type SuccessOption func(* domain.ResponseSuccess)

func WithMessage(msg string) SuccessOption {
    return func(r *domain.ResponseSuccess) {
        r.Message = msg
    }
}