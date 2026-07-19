package domain

// ResponseSuccess represents the successful API response structure
type ResponseSuccess struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data"`
	RequestID string      `json:"reqId"`
}


