package onedef_test

import (
	"net/http"
	"testing"

	"github.com/failer-dev/onedef"
)

func TestMiddlewarePublicAPISurfaceCompilesExternally(t *testing.T) {
	auth := onedef.NamedMiddleware("auth", onedef.MiddlewareFunc(func(next onedef.HandlerFunc) onedef.HandlerFunc {
		return next
	}))
	std := onedef.HTTPMiddleware(func(next http.Handler) http.Handler {
		return next
	})

	api := onedef.Group(
		"/api",
		onedef.ErrorPolicy(func(_ *http.Request, err error) (int, externalError) {
			if httpErr, ok := onedef.AsHTTPError(err); ok {
				return httpErr.Status, externalError{Message: httpErr.Message}
			}
			return http.StatusInternalServerError, externalError{Message: err.Error()}
		}),
		onedef.Use(auth),
		onedef.Group(
			"/public",
			onedef.SkipMiddleware("auth"),
			onedef.Endpoint(&sdkNameEndpoint{}),
		),
		onedef.Group(
			"/items",
			onedef.Endpoint(
				&sdkNameEndpoint{},
				onedef.EndpointMiddleware(std),
				onedef.SkipEndpointMiddleware("auth"),
			),
		),
	)

	app := onedef.New(api)
	if app == nil {
		t.Fatal("New(api) = nil")
	}
}

type externalError struct {
	Message string `json:"message"`
}
