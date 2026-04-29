package meta

import (
	"net/http"
	"reflect"
	"strings"
)

type HandlerFunc func(http.ResponseWriter, *http.Request) error

type Middleware interface {
	Wrap(HandlerFunc) HandlerFunc
}

type MiddlewareFunc func(HandlerFunc) HandlerFunc

func (f MiddlewareFunc) Wrap(next HandlerFunc) HandlerFunc {
	if f == nil {
		panic("onedef: middleware func must not be nil")
	}
	return f(next)
}

type namedMiddleware struct {
	name       string
	middleware Middleware
}

func (m namedMiddleware) Wrap(next HandlerFunc) HandlerFunc {
	return m.middleware.Wrap(next)
}

func (m namedMiddleware) middlewareName() string {
	return m.name
}

func NamedMiddleware(name string, middleware Middleware) Middleware {
	name = strings.TrimSpace(name)
	if name == "" {
		panic("onedef: middleware name must not be empty")
	}
	mustMiddleware(middleware)
	return namedMiddleware{name: name, middleware: middleware}
}

func MiddlewareName(middleware Middleware) (string, bool) {
	named, ok := middleware.(interface {
		middlewareName() string
	})
	if !ok {
		return "", false
	}
	return named.middlewareName(), true
}

func HTTPMiddleware(middleware func(http.Handler) http.Handler) Middleware {
	if middleware == nil {
		panic("onedef: http middleware must not be nil")
	}
	return MiddlewareFunc(func(next HandlerFunc) HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) error {
			var err error
			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				err = next(w, r)
			}))
			if handler == nil {
				panic("onedef: http middleware returned nil handler")
			}
			handler.ServeHTTP(w, r)
			return err
		}
	})
}

func Recover() Middleware {
	return MiddlewareFunc(func(next HandlerFunc) HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) (err error) {
			defer func() {
				if recovered := recover(); recovered != nil {
					err = Internal("internal_error", "internal server error", nil)
				}
			}()
			return next(w, r)
		}
	})
}

func MustMiddleware(middleware Middleware) {
	mustMiddleware(middleware)
}

func mustMiddleware(middleware Middleware) {
	if isNilMiddleware(middleware) {
		panic("onedef: middleware must not be nil")
	}
}

func isNilMiddleware(middleware Middleware) bool {
	if middleware == nil {
		return true
	}
	value := reflect.ValueOf(middleware)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
