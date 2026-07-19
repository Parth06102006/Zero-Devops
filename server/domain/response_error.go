package domain

type DebugInfo struct {
    RawError string `json:"raw_error"`
    Stack    string `json:"stack,omitempty"`
    Reason   string `json:"reason,omitempty"`
    Query    string `json:"query,omitempty"`
}

type ErrorBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Debug   *DebugInfo `json:"debug,omitempty"`
}

type ErrorResponse struct {
	Success bool       `json:"success"`
	Error   ErrorBody  `json:"error"`
	RequestID   string `json:"reqId"`
}

