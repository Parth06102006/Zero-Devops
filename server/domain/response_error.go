package domain

// DebugInfo contains additional error debug details for non-production environments
type DebugInfo struct {
	RawError string `json:"raw_error"`
	Stack    string `json:"stack,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Query    string `json:"query,omitempty"`
}

// ErrorBody represents the structured error payload
type ErrorBody struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Debug   *DebugInfo `json:"debug,omitempty"`
}

// ErrorResponse represents the standard error API response
type ErrorResponse struct {
	Success   bool      `json:"success"`
	Error     ErrorBody `json:"error"`
	RequestID string    `json:"reqId"`
}
