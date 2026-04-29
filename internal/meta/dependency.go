package meta

import (
	"fmt"
	"reflect"
)

type DependencyBinding interface {
	Node
	isDependencyBinding()
}

type dependencyBinding struct {
	dependencyType  reflect.Type
	dependencyValue reflect.Value
}

func (dependencyBinding) isDependencyBinding() {}

func (b dependencyBinding) MetaNode() Node { return b }

func NewDependencyBinding[T any](value T) DependencyBinding {
	bindingType := reflect.TypeFor[T]()
	if isNilDependencyValue(value) {
		panic(fmt.Sprintf("onedef: cannot bind nil dependency for %s", bindingType))
	}

	return dependencyBinding{
		dependencyType:  bindingType,
		dependencyValue: reflect.ValueOf(value),
	}
}

func DependencyType(binding DependencyBinding) reflect.Type {
	concrete, ok := binding.(dependencyBinding)
	if !ok {
		return nil
	}
	return concrete.dependencyType
}

func DependencyValue(binding DependencyBinding) reflect.Value {
	concrete, ok := binding.(dependencyBinding)
	if !ok {
		return reflect.Value{}
	}
	return concrete.dependencyValue
}

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

type DependencyField struct {
	FieldName  string
	FieldIndex int
	FieldType  reflect.Type
}

type DependenciesField struct {
	Exists      bool
	StructIndex int
	Fields      []DependencyField
}
