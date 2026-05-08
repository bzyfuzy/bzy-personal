package client

import (
	"fmt"
	"net/http"
)

// APIError is returned when the server responds with a non-2xx status.
type APIError struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Detail     string `json:"detail,omitempty"`
}

func (e *APIError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("[%d %s] %s: %s", e.StatusCode, e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("[%d %s] %s", e.StatusCode, e.Code, e.Message)
}

func (e *APIError) IsNotFound() bool     { return e.StatusCode == http.StatusNotFound }
func (e *APIError) IsUnauthorized() bool { return e.StatusCode == http.StatusUnauthorized }
func (e *APIError) IsForbidden() bool    { return e.StatusCode == http.StatusForbidden }
func (e *APIError) IsRateLimited() bool  { return e.StatusCode == http.StatusTooManyRequests }
func (e *APIError) IsServerError() bool  { return e.StatusCode >= 500 }

// RetryableError wraps a transient error that can be retried.
type RetryableError struct{ Cause error }

func (e *RetryableError) Error() string  { return "retryable: " + e.Cause.Error() }
func (e *RetryableError) Unwrap() error  { return e.Cause }

func isRetryable(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == http.StatusTooManyRequests ||
			apiErr.StatusCode >= http.StatusInternalServerError
	}
	return false
}
