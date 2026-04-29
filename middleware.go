package onedef

import (
	"net/http"

	"github.com/failer-dev/onedef/internal/meta"
)

type HandlerFunc = meta.HandlerFunc
type Middleware = meta.Middleware
type MiddlewareFunc = meta.MiddlewareFunc

func Use(middlewares ...Middleware) Node {
	return meta.Use(middlewares...)
}

func SkipMiddleware(names ...string) Node {
	return meta.SkipMiddleware(names...)
}

func EndpointMiddleware(middlewares ...Middleware) EndpointOption {
	return EndpointOption{option: meta.EndpointMiddleware(middlewares...)}
}

func SkipEndpointMiddleware(names ...string) EndpointOption {
	return EndpointOption{option: meta.SkipEndpointMiddleware(names...)}
}

func NamedMiddleware(name string, middleware Middleware) Middleware {
	return meta.NamedMiddleware(name, middleware)
}

func HTTPMiddleware(middleware func(http.Handler) http.Handler) Middleware {
	return meta.HTTPMiddleware(middleware)
}

func Recover() Middleware {
	return meta.Recover()
}
