package meta

import (
	"errors"
	"fmt"
)

type DefaultError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type HTTPError struct {
	Status  int
	Code    string
	Title   string
	Message string
	Data    any
}

func NewHTTPError(status int, code, title, message string, data any) *HTTPError {
	return &HTTPError{
		Status:  status,
		Code:    code,
		Title:   title,
		Message: message,
		Data:    data,
	}
}

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

func AsHTTPError(err error) (*HTTPError, bool) {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr, true
	}
	return nil, false
}

func BadRequest(code, message string, data any) *HTTPError {
	return NewHTTPError(400, code, "Bad Request", message, data)
}

func Unauthorized(code, message string, data any) *HTTPError {
	return NewHTTPError(401, code, "Unauthorized", message, data)
}

func Forbidden(code, message string, data any) *HTTPError {
	return NewHTTPError(403, code, "Forbidden", message, data)
}

func NotFound(code, message string, data any) *HTTPError {
	return NewHTTPError(404, code, "Not Found", message, data)
}

func Conflict(code, message string, data any) *HTTPError {
	return NewHTTPError(409, code, "Conflict", message, data)
}

func Unprocessable(code, message string, data any) *HTTPError {
	return NewHTTPError(422, code, "Unprocessable Entity", message, data)
}

func Internal(code, message string, data any) *HTTPError {
	return NewHTTPError(500, code, "Internal Server Error", message, data)
}
