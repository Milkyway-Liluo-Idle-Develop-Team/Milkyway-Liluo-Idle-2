// Package apperror defines a small, transport-agnostic error type used across
// the application. Handlers (HTTP and WS) translate AppError into the right
// wire format. Other code returns AppError to express semantic failure modes
// without coupling to a transport.
package apperror

import (
	"errors"
	"fmt"
)

// Code is a stable, machine-friendly error identifier. Treat values like an
// API contract: changing them is a breaking change for clients.
type Code string

const (
	CodeInternal       Code = "internal"
	CodeBadRequest     Code = "bad_request"
	CodeUnauthorized   Code = "unauthorized"
	CodeForbidden      Code = "forbidden"
	CodeNotFound       Code = "not_found"
	CodeConflict       Code = "conflict"
	CodeRateLimited    Code = "rate_limited"
	CodeUnavailable    Code = "unavailable"
	CodeValidation     Code = "validation"
)

// AppError is a structured error carrying a machine code, a human-readable
// message, optional structured fields, and an optional cause for logging.
type AppError struct {
	Code    Code           `json:"code"`
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
	cause   error
}

func (e *AppError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.cause }

// WithCause attaches an underlying error for logging while keeping the
// public-facing message unchanged.
func (e *AppError) WithCause(err error) *AppError {
	clone := *e
	clone.cause = err
	return &clone
}

// WithField adds a structured field to the error payload.
func (e *AppError) WithField(key string, value any) *AppError {
	clone := *e
	if clone.Fields == nil {
		clone.Fields = map[string]any{}
	} else {
		// copy to avoid sharing the map across instances
		fields := make(map[string]any, len(clone.Fields)+1)
		for k, v := range clone.Fields {
			fields[k] = v
		}
		clone.Fields = fields
	}
	clone.Fields[key] = value
	return &clone
}

// New constructs a new AppError.
func New(code Code, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

// As is a convenience that mirrors errors.As specialised for *AppError.
func As(err error) (*AppError, bool) {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}

// Convenience constructors keep call sites short.
func Internal(msg string) *AppError     { return New(CodeInternal, msg) }
func BadRequest(msg string) *AppError   { return New(CodeBadRequest, msg) }
func Unauthorized(msg string) *AppError { return New(CodeUnauthorized, msg) }
func Forbidden(msg string) *AppError    { return New(CodeForbidden, msg) }
func NotFound(msg string) *AppError     { return New(CodeNotFound, msg) }
func Conflict(msg string) *AppError     { return New(CodeConflict, msg) }
func Validation(msg string) *AppError   { return New(CodeValidation, msg) }
func Unavailable(msg string) *AppError  { return New(CodeUnavailable, msg) }
