package meta

import (
	"errors"
	"fmt"
)

// DefaultError is the response body used by the default error policy.
type DefaultError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// HTTPError carries an HTTP status and response details through handler errors.
// NewHTTPError does not validate Status; use the named constructors when a
// standard status is desired.
type HTTPError struct {
	Status  int
	Code    string
	Title   string
	Message string
	Data    any
}

// NewHTTPError creates an HTTPError with caller-provided status and details.
func NewHTTPError(status int, code, title, message string, data any) *HTTPError {
	return &HTTPError{
		Status:  status,
		Code:    code,
		Title:   title,
		Message: message,
		Data:    data,
	}
}

// Error returns the message used when HTTPError is handled as a plain error.
func (e *HTTPError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Title != "" {
		return e.Title
	}
	return fmt.Sprintf("http error %d", e.Status)
}

// AsHTTPError unwraps err and reports whether it contains an HTTPError.
func AsHTTPError(err error) (*HTTPError, bool) {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr, true
	}
	return nil, false
}

// BadRequest creates a 400 HTTPError.
func BadRequest(code, message string, data any) *HTTPError {
	return NewHTTPError(400, code, "Bad Request", message, data)
}

// Unauthorized creates a 401 HTTPError.
func Unauthorized(code, message string, data any) *HTTPError {
	return NewHTTPError(401, code, "Unauthorized", message, data)
}

// Forbidden creates a 403 HTTPError.
func Forbidden(code, message string, data any) *HTTPError {
	return NewHTTPError(403, code, "Forbidden", message, data)
}

// NotFound creates a 404 HTTPError.
func NotFound(code, message string, data any) *HTTPError {
	return NewHTTPError(404, code, "Not Found", message, data)
}

// Conflict creates a 409 HTTPError.
func Conflict(code, message string, data any) *HTTPError {
	return NewHTTPError(409, code, "Conflict", message, data)
}

// Unprocessable creates a 422 HTTPError.
func Unprocessable(code, message string, data any) *HTTPError {
	return NewHTTPError(422, code, "Unprocessable Entity", message, data)
}

// Internal creates a 500 HTTPError.
func Internal(code, message string, data any) *HTTPError {
	return NewHTTPError(500, code, "Internal Server Error", message, data)
}
