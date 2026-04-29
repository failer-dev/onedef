package onedef

import "github.com/failer-dev/onedef/internal/meta"

type HTTPError = meta.HTTPError

func AsHTTPError(err error) (*HTTPError, bool) {
	return meta.AsHTTPError(err)
}

func NewHTTPError(status int, code, title, message string, data any) *HTTPError {
	return meta.NewHTTPError(status, code, title, message, data)
}

func BadRequest(code, message string, data any) *HTTPError {
	return meta.BadRequest(code, message, data)
}

func Unauthorized(code, message string, data any) *HTTPError {
	return meta.Unauthorized(code, message, data)
}

func Forbidden(code, message string, data any) *HTTPError {
	return meta.Forbidden(code, message, data)
}

func NotFound(code, message string, data any) *HTTPError {
	return meta.NotFound(code, message, data)
}

func Conflict(code, message string, data any) *HTTPError {
	return meta.Conflict(code, message, data)
}

func Unprocessable(code, message string, data any) *HTTPError {
	return meta.Unprocessable(code, message, data)
}

func Internal(code, message string, data any) *HTTPError {
	return meta.Internal(code, message, data)
}
