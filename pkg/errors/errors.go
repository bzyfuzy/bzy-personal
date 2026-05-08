// Package errors provides typed domain errors for the platform.
package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// Code represents a machine-readable error classification.
type Code string

const (
	CodeNotFound          Code = "NOT_FOUND"
	CodeAlreadyExists     Code = "ALREADY_EXISTS"
	CodeInvalidArgument   Code = "INVALID_ARGUMENT"
	CodeUnauthorized      Code = "UNAUTHORIZED"
	CodeForbidden         Code = "FORBIDDEN"
	CodeInternal          Code = "INTERNAL"
	CodeUnavailable       Code = "UNAVAILABLE"
	CodeTimeout           Code = "TIMEOUT"
	CodeRateLimited       Code = "RATE_LIMITED"
	CodeConflict          Code = "CONFLICT"
	CodeUnprocessable     Code = "UNPROCESSABLE"
)

// DomainError is the standard platform error type.
type DomainError struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
	Cause   error  `json:"-"`
}

func (e *DomainError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *DomainError) Unwrap() error { return e.Cause }

func (e *DomainError) HTTPStatus() int {
	switch e.Code {
	case CodeNotFound:
		return http.StatusNotFound
	case CodeAlreadyExists, CodeConflict:
		return http.StatusConflict
	case CodeInvalidArgument, CodeUnprocessable:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeRateLimited:
		return http.StatusTooManyRequests
	case CodeTimeout:
		return http.StatusGatewayTimeout
	case CodeUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// ─── Constructors ──────────────────────────────────────────────────────────────

func NotFound(msg string, cause ...error) *DomainError {
	return newErr(CodeNotFound, msg, cause...)
}
func AlreadyExists(msg string, cause ...error) *DomainError {
	return newErr(CodeAlreadyExists, msg, cause...)
}
func InvalidArgument(msg string, cause ...error) *DomainError {
	return newErr(CodeInvalidArgument, msg, cause...)
}
func Unauthorized(msg string, cause ...error) *DomainError {
	return newErr(CodeUnauthorized, msg, cause...)
}
func Forbidden(msg string, cause ...error) *DomainError {
	return newErr(CodeForbidden, msg, cause...)
}
func Internal(msg string, cause ...error) *DomainError {
	return newErr(CodeInternal, msg, cause...)
}
func Unavailable(msg string, cause ...error) *DomainError {
	return newErr(CodeUnavailable, msg, cause...)
}
func RateLimited(msg string, cause ...error) *DomainError {
	return newErr(CodeRateLimited, msg, cause...)
}

func newErr(code Code, msg string, cause ...error) *DomainError {
	e := &DomainError{Code: code, Message: msg}
	if len(cause) > 0 {
		e.Cause = cause[0]
	}
	return e
}

// ─── Helpers ───────────────────────────────────────────────────────────────────

func Is(err error, code Code) bool {
	var de *DomainError
	if errors.As(err, &de) {
		return de.Code == code
	}
	return false
}

func As(err error) (*DomainError, bool) {
	var de *DomainError
	return de, errors.As(err, &de)
}
