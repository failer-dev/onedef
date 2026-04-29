package app

import (
	"fmt"
	"reflect"

	"github.com/failer-dev/onedef/internal/meta"
)

type dependencyRegistry map[reflect.Type]reflect.Value

type resolvedDependency struct {
	FieldIndex int
	Value      reflect.Value
}

func (r dependencyRegistry) clone() dependencyRegistry {
	if len(r) == 0 {
		return dependencyRegistry{}
	}
	result := make(dependencyRegistry, len(r))
	for key, value := range r {
		result[key] = value
	}
	return result
}

func (r dependencyRegistry) addScopedBinding(binding meta.DependencyBinding, local map[reflect.Type]struct{}) error {
	bindingType := meta.DependencyType(binding)
	if bindingType == nil {
		return fmt.Errorf("onedef: invalid dependency binding")
	}
	if _, exists := local[bindingType]; exists {
		return fmt.Errorf("onedef: dependency %s already bound in this scope", bindingType)
	}

	bindingValue := meta.DependencyValue(binding)
	if !bindingValue.IsValid() {
		return fmt.Errorf("onedef: invalid dependency binding for %s", bindingType)
	}

	r[bindingType] = bindingValue
	local[bindingType] = struct{}{}
	return nil
}

func (r dependencyRegistry) snapshotDependencies(es meta.EndpointStruct) ([]resolvedDependency, error) {
	if !es.Dependencies.Exists {
		return nil, nil
	}

	resolved := make([]resolvedDependency, 0, len(es.Dependencies.Fields))
	for _, field := range es.Dependencies.Fields {
		value, exists := r[field.FieldType]
		if !exists {
			return nil, fmt.Errorf(
				"onedef: dependency %s required by %s.Deps.%s not bound (did you forget onedef.Dependency(...) in the group tree?)",
				field.FieldType,
				es.StructName,
				field.FieldName,
			)
		}
		resolved = append(resolved, resolvedDependency{
			FieldIndex: field.FieldIndex,
			Value:      value,
		})
	}
	return resolved, nil
}
