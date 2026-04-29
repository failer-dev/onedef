package onedef

import (
	"net/http"

	"github.com/failer-dev/onedef/internal/meta"
)

type DefaultError = meta.DefaultError

type ErrorMapper[T any] func(*http.Request, error) (statusCode int, body T)

func ErrorPolicy[T any](mapper ErrorMapper[T]) Node {
	if mapper == nil {
		panic("onedef: error policy mapper must not be nil")
	}
	return meta.ErrorPolicy(func(r *http.Request, err error) (int, T) {
		return mapper(r, err)
	})
}

func EndpointErrorPolicy[T any](mapper ErrorMapper[T]) EndpointOption {
	if mapper == nil {
		panic("onedef: error policy mapper must not be nil")
	}
	return EndpointOption{option: meta.EndpointErrorPolicy(meta.ErrorPolicy(func(r *http.Request, err error) (int, T) {
		return mapper(r, err)
	}))}
}
