package meta

import (
	"fmt"
	"reflect"
)

// ProvideBinding records one value that can be injected into a unit's Provide
// field. Implementations are sealed to this package so callers
// cannot construct unchecked bindings.
type ProvideBinding interface {
	Node
	isProvideBinding()
}

// provideBinding keeps both the generic binding type and the concrete value.
// The type is the lookup key used later, so callers should bind interface
// providers with an explicit type parameter.
type provideBinding struct {
	provideType  reflect.Type
	provideValue reflect.Value
}

func (provideBinding) isProvideBinding() {}

func (b provideBinding) MetaNode() Node { return b }

// NewProvideBinding creates a provided value binding for value.
//
// NewProvideBinding panics when value is nil, including typed nil values for
// nil-capable kinds. The generic type T is captured as the dependency key; use
// an explicit type argument when the endpoint field is an interface.
func NewProvideBinding[T any](value T) ProvideBinding {
	bindingType := reflect.TypeFor[T]()
	if isNilDependencyValue(value) {
		panic(fmt.Sprintf("onedef: cannot provide nil value for %s", bindingType))
	}

	return provideBinding{
		provideType:  bindingType,
		provideValue: reflect.ValueOf(value),
	}
}

// ProvideType returns the lookup type captured by a valid binding.
//
// ProvideType returns nil for bindings that were not created by this
// package, allowing internal validators to reject unchecked implementations.
func ProvideType(binding ProvideBinding) reflect.Type {
	concrete, ok := binding.(provideBinding)
	if !ok {
		return nil
	}
	return concrete.provideType
}

// ProvideValue returns the concrete value captured by a valid binding.
//
// ProvideValue returns an invalid reflect.Value for bindings that were not
// created by this package.
func ProvideValue(binding ProvideBinding) reflect.Value {
	concrete, ok := binding.(provideBinding)
	if !ok {
		return reflect.Value{}
	}
	return concrete.provideValue
}

// isNilDependencyValue detects typed nil values before they are stored in a
// binding. A binding with a nil runtime value would pass type lookup but panic
// later during reflection-based injection.
func isNilDependencyValue[T any](value T) bool {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return true
	}

	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

// ProvideField describes a single field inside a unit's Provide struct.
// FieldIndex is resolved against that Provide struct, not the unit itself.
type ProvideField struct {
	FieldName  string
	FieldIndex int
	FieldType  reflect.Type
}

// ProvideFieldSet records the Provide field discovered on a unit struct.
// When Exists is false, StructIndex and Fields are intentionally ignored.
type ProvideFieldSet struct {
	Exists      bool
	StructIndex int
	Fields      []ProvideField
}
