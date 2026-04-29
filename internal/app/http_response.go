package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode"

	"github.com/failer-dev/onedef/internal/meta"
)

type responseEnvelope struct {
	Code    string `json:"code"`
	Title   string `json:"title"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeHTTPSuccess(w http.ResponseWriter, status int, data any) {
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}

	writeJSON(w, status, responseEnvelope{
		Code:    successCode(status),
		Title:   http.StatusText(status),
		Message: "success",
		Data:    data,
	})
}

func defaultErrorPolicy() meta.ErrorPolicyBinding {
	return meta.ErrorPolicy(defaultErrorMapper)
}

func defaultErrorMapper(_ *http.Request, err error) (int, meta.DefaultError) {
	if httpErr, ok := meta.AsHTTPError(err); ok {
		return httpErr.Status, meta.DefaultError{
			Code:    httpErr.Code,
			Message: httpErr.Message,
			Details: httpErr.Data,
		}
	}
	return http.StatusInternalServerError, meta.DefaultError{
		Code:    "internal_error",
		Message: "internal server error",
	}
}

func successCode(status int) string {
	title := http.StatusText(status)
	if title == "" {
		return "success"
	}

	var sb strings.Builder
	lastWasUnderscore := false

	for _, r := range title {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			sb.WriteRune(unicode.ToLower(r))
			lastWasUnderscore = false
		case !lastWasUnderscore && sb.Len() > 0:
			sb.WriteByte('_')
			lastWasUnderscore = true
		}
	}

	code := strings.Trim(sb.String(), "_")
	if code == "" {
		return "success"
	}
	return code
}
