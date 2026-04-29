package meta

import (
	"net/http"
	"reflect"
)

// ErrorMapper maps a handler error to an HTTP status and serializable response
// body. ErrorPolicy requires the body type T to be a non-pointer named struct
// so the SDK spec can describe it without invoking the mapper.
type ErrorMapper[T any] func(*http.Request, error) (statusCode int, body T)

// ErrorPolicyBinding is a sealed node that carries error mapping behavior and
// the mapper's response body type. Values should be created with ErrorPolicy so
// the body type invariant is checked once.
type ErrorPolicyBinding interface {
	Node
	// MapError returns the status code and response body for err.
	MapError(*http.Request, error) (int, any)
	// ErrorBodyType returns the named struct type emitted into generated specs.
	ErrorBodyType() reflect.Type
	errorPolicyBinding()
}

// errorPolicyBinding stores the mapper separately from its body type so spec
// generation can inspect the error shape without causing mapper side effects.
type errorPolicyBinding[T any] struct {
	mapper   ErrorMapper[T]
	bodyType reflect.Type
}

// MetaNode returns p as a DSL node.
func (p errorPolicyBinding[T]) MetaNode() Node { return p }

// MapError applies the mapper captured by ErrorPolicy.
func (p errorPolicyBinding[T]) MapError(r *http.Request, err error) (int, any) {
	status, body := p.mapper(r, err)
	return status, body
}

// ErrorBodyType returns the response body type captured when p was created.
func (p errorPolicyBinding[T]) ErrorBodyType() reflect.Type {
	return p.bodyType
}

func (p errorPolicyBinding[T]) errorPolicyBinding() {}

// ErrorPolicy creates a binding that overrides the default handler error
// response for a group or endpoint.
//
// ErrorPolicy panics if mapper is nil or if T is not a non-pointer named struct.
// The mapper is called only when a handler returns an error.
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

// MustErrorPolicy panics unless policy is a valid ErrorPolicyBinding.
func MustErrorPolicy(policy ErrorPolicyBinding) {
	mustErrorPolicy(policy)
}

// mustErrorPolicy centralizes policy validation so all DSL entry points enforce
// the same nil and body-type invariants before registration.
func mustErrorPolicy(policy ErrorPolicyBinding) {
	if policy == nil || policy.ErrorBodyType() == nil {
		panic("onedef: error policy must not be nil")
	}
	mustErrorBodyType(policy.ErrorBodyType())
}

// mustErrorBodyType preserves the SDK-generation invariant that error bodies
// are named structs and can be referenced by type name.
func mustErrorBodyType(t reflect.Type) {
	if t == nil || t.Kind() != reflect.Struct || t.Name() == "" {
		panic("onedef: error policy body type must be a non-pointer named struct")
	}
}
