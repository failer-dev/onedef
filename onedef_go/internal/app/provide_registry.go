package app

import (
	"fmt"
	"reflect"

	"github.com/failer-dev/onedef/onedef_go/internal/meta"
)

type provideRegistry map[reflect.Type]reflect.Value

type provideKey struct {
	name string
	typ  reflect.Type
}

type provideScope map[provideKey]reflect.Value

func (r provideRegistry) clone() provideRegistry {
	if len(r) == 0 {
		return provideRegistry{}
	}
	result := make(provideRegistry, len(r))
	for key, value := range r {
		result[key] = value
	}
	return result
}

func (r provideRegistry) addScopedBinding(binding meta.ProvideBinding) error {
	bindingType := meta.ProvideType(binding)
	if bindingType == nil {
		return fmt.Errorf("onedef: invalid provide binding")
	}

	bindingValue := meta.ProvideValue(binding)
	if !bindingValue.IsValid() {
		return fmt.Errorf("onedef: invalid provide binding for %s", bindingType)
	}

	r[bindingType] = bindingValue
	return nil
}

func newProvideScope() provideScope {
	return make(provideScope)
}

func (s provideScope) set(field meta.ProvideField, value reflect.Value) {
	s[provideKey{name: field.FieldName, typ: field.FieldType}] = value
}

func (s provideScope) get(field meta.ProvideField) (reflect.Value, bool) {
	value, ok := s[provideKey{name: field.FieldName, typ: field.FieldType}]
	return value, ok
}

func fillProvideFieldSet(target reflect.Value, fields meta.ProvideFieldSet, registry provideRegistry, scope provideScope, strict bool, owner string) error {
	if !fields.Exists {
		return nil
	}

	provideField := target.Field(fields.StructIndex)
	for _, field := range fields.Fields {
		value, ok := scope.get(field)
		if !ok {
			value, ok = registry[field.FieldType]
		}
		if !ok {
			if !strict && !isNilCapable(field.FieldType) {
				continue
			}
			return fmt.Errorf("onedef: provide %s required by %s.Provide.%s not bound", field.FieldType, owner, field.FieldName)
		}
		if !value.Type().AssignableTo(field.FieldType) {
			if value.Type().ConvertibleTo(field.FieldType) {
				value = value.Convert(field.FieldType)
			} else {
				return fmt.Errorf("onedef: cannot assign provide %s to %s.Provide.%s (%s)", value.Type(), owner, field.FieldName, field.FieldType)
			}
		}
		provideField.Field(field.FieldIndex).Set(value)
	}
	return nil
}

func mergeProvideFieldSet(source reflect.Value, fields meta.ProvideFieldSet, scope provideScope) {
	if !fields.Exists {
		return
	}
	provideField := source.Field(fields.StructIndex)
	for _, field := range fields.Fields {
		scope.set(field, provideField.Field(field.FieldIndex))
	}
}

func isNilCapable(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return true
	default:
		return false
	}
}
