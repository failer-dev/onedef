package meta

import (
	"net/http"
	"reflect"
)

type ErrorMapper[T any] func(*http.Request, error) (statusCode int, body T)

type ErrorPolicyBinding interface {
	Node
	MapError(*http.Request, error) (int, any)
	ErrorBodyType() reflect.Type
	errorPolicyBinding()
}

type errorPolicyBinding[T any] struct {
	mapper   ErrorMapper[T]
	bodyType reflect.Type
}

func (p errorPolicyBinding[T]) MetaNode() Node { return p }

func (p errorPolicyBinding[T]) MapError(r *http.Request, err error) (int, any) {
	status, body := p.mapper(r, err)
	return status, body
}

func (p errorPolicyBinding[T]) ErrorBodyType() reflect.Type {
	return p.bodyType
}

func (p errorPolicyBinding[T]) errorPolicyBinding() {}

func ErrorPolicy[T any](mapper ErrorMapper[T]) ErrorPolicyBinding {
	if mapper == nil {
		panic("onedef: error policy mapper must not be nil")
	}
	bodyType := reflect.TypeFor[T]()
	mustErrorBodyType(bodyType)
	return errorPolicyBinding[T]{
		mapper:   mapper,
		bodyType: bodyType,
	}
}

func MustErrorPolicy(policy ErrorPolicyBinding) {
	mustErrorPolicy(policy)
}

func mustErrorPolicy(policy ErrorPolicyBinding) {
	if policy == nil || policy.ErrorBodyType() == nil {
		panic("onedef: error policy must not be nil")
	}
	mustErrorBodyType(policy.ErrorBodyType())
}

func mustErrorBodyType(t reflect.Type) {
	if t == nil || t.Kind() != reflect.Struct || t.Name() == "" {
		panic("onedef: error policy body type must be a non-pointer named struct")
	}
}
