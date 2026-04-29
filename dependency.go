package onedef

import "github.com/failer-dev/onedef/internal/meta"

type DependencyBinding = meta.DependencyBinding

func Dependency[T any](value T) DependencyBinding {
	return meta.NewDependencyBinding[T](value)
}

func EndpointDependency(bindings ...DependencyBinding) EndpointOption {
	return EndpointOption{option: meta.EndpointDependency(bindings...)}
}
