package meta

import "strings"

type Node interface {
	MetaNode() Node
}

type GroupNode struct {
	Path     string
	Children []Node
	Exposed  bool
}

func (n GroupNode) MetaNode() Node { return n }

type EndpointNode struct {
	Endpoint            any
	SDKName             string
	Middlewares         []Middleware
	SkipMiddlewareNames []string
	Dependencies        []DependencyBinding
	ErrorPolicy         ErrorPolicyBinding
}

func (n EndpointNode) MetaNode() Node { return n }

type EndpointOption interface {
	applyEndpointOption(*EndpointNode)
}

type endpointOptionFunc func(*EndpointNode)

func (f endpointOptionFunc) applyEndpointOption(node *EndpointNode) {
	f(node)
}

type EndpointsNode struct {
	Endpoints []any
}

func (n EndpointsNode) MetaNode() Node { return n }

type RequireHeaderNode struct {
	Name string
}

func (n RequireHeaderNode) MetaNode() Node { return n }

type OmitHeaderNode struct {
	Name string
}

func (n OmitHeaderNode) MetaNode() Node { return n }

type UseNode struct {
	Middlewares []Middleware
}

func (n UseNode) MetaNode() Node { return n }

type SkipMiddlewareNode struct {
	Names []string
}

func (n SkipMiddlewareNode) MetaNode() Node { return n }

type GroupRef struct {
	Path string
}

type GroupMeta struct {
	ID                      string
	Name                    string
	PathPrefix              string
	PathSegments            []string
	LocalRequiredHeaders    []string
	ProviderRequiredHeaders []string
	FinalRequiredHeaders    []string
	Children                []*GroupMeta
	Endpoints               []EndpointStruct
}

func Group(path string, children ...Node) Node {
	return GroupNode{Path: path, Children: children, Exposed: true}
}

func Endpoint(endpoint any, opts ...EndpointOption) Node {
	node := EndpointNode{Endpoint: endpoint}
	for _, opt := range opts {
		if opt != nil {
			opt.applyEndpointOption(&node)
		}
	}
	return node
}

func Endpoints(endpoints ...any) Node {
	return EndpointsNode{Endpoints: endpoints}
}

func RequireHeader(name string) Node {
	return RequireHeaderNode{Name: name}
}

func OmitHeader(name string) Node {
	return OmitHeaderNode{Name: name}
}

func Use(middlewares ...Middleware) Node {
	for _, middleware := range middlewares {
		mustMiddleware(middleware)
	}
	return UseNode{Middlewares: append([]Middleware(nil), middlewares...)}
}

func SkipMiddleware(names ...string) Node {
	return SkipMiddlewareNode{Names: normalizeMiddlewareNames(names)}
}

func SDKName(name string) EndpointOption {
	return endpointOptionFunc(func(node *EndpointNode) {
		node.SDKName = name
	})
}

func EndpointMiddleware(middlewares ...Middleware) EndpointOption {
	for _, middleware := range middlewares {
		mustMiddleware(middleware)
	}
	return endpointOptionFunc(func(node *EndpointNode) {
		node.Middlewares = append(node.Middlewares, middlewares...)
	})
}

func SkipEndpointMiddleware(names ...string) EndpointOption {
	normalized := normalizeMiddlewareNames(names)
	return endpointOptionFunc(func(node *EndpointNode) {
		node.SkipMiddlewareNames = append(node.SkipMiddlewareNames, normalized...)
	})
}

func EndpointDependency(bindings ...DependencyBinding) EndpointOption {
	for _, binding := range bindings {
		mustDependencyBinding(binding)
	}
	return endpointOptionFunc(func(node *EndpointNode) {
		node.Dependencies = append(node.Dependencies, bindings...)
	})
}

func EndpointErrorPolicy(policy ErrorPolicyBinding) EndpointOption {
	mustErrorPolicy(policy)
	return endpointOptionFunc(func(node *EndpointNode) {
		if node.ErrorPolicy != nil {
			panic("onedef: endpoint error policy already declared")
		}
		node.ErrorPolicy = policy
	})
}

func normalizeMiddlewareNames(names []string) []string {
	normalized := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			panic("onedef: middleware name must not be empty")
		}
		normalized = append(normalized, name)
	}
	return normalized
}

func mustDependencyBinding(binding DependencyBinding) {
	if DependencyType(binding) == nil || !DependencyValue(binding).IsValid() {
		panic("onedef: dependency binding must not be nil")
	}
}
