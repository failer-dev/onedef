package meta

import "reflect"

// Node is the common representation for values accepted by the group DSL.
// Implementations return their canonical metadata through MetaNode before the
// app registration step interprets them.
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
	Endpoint        any
	SDKName         string
	RequiredHeaders []HeaderContract
	BeforeHandlers  []any
	AfterHandlers   []any
	Observers       []Observer
	Provides        []ProvideBinding
	ErrorPolicy     ErrorPolicyBinding
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
	Header HeaderContract
}

func (n RequireHeaderNode) MetaNode() Node { return n }

func (n RequireHeaderNode) applyEndpointOption(node *EndpointNode) {
	node.RequiredHeaders = append(node.RequiredHeaders, n.Header)
}

type OmitHeaderNode struct {
	Header HeaderContract
}

func (n OmitHeaderNode) MetaNode() Node { return n }

type GroupMeta struct {
	ID                      string
	Name                    string
	PathPrefix              string
	PathSegments            []string
	LocalRequiredHeaders    []HeaderContract
	ProviderRequiredHeaders []HeaderContract
	FinalRequiredHeaders    []HeaderContract
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

func RequireHeader(header HeaderSymbol) RequireHeaderNode {
	return RequireHeaderNode{Header: MustHeaderContract(header)}
}

func OmitHeader(header HeaderSymbol) Node {
	return OmitHeaderNode{Header: MustHeaderContract(header)}
}

func BeforeHandle(handler any) BeforeHandleNode {
	mustBeforeHandle(handler)
	return BeforeHandleNode{Handler: handler}
}

func AfterHandle(handler any) AfterHandleNode {
	mustAfterHandle(handler)
	return AfterHandleNode{Handler: handler}
}

func Observe(observer any) ObserveNode {
	return ObserveNode{Observer: mustObserver(observer)}
}

func EndpointBeforeHandle(handlers ...any) EndpointOption {
	for _, handler := range handlers {
		mustBeforeHandle(handler)
	}
	return endpointOptionFunc(func(node *EndpointNode) {
		node.BeforeHandlers = append(node.BeforeHandlers, handlers...)
	})
}

func EndpointAfterHandle(handlers ...any) EndpointOption {
	for _, handler := range handlers {
		mustAfterHandle(handler)
	}
	return endpointOptionFunc(func(node *EndpointNode) {
		node.AfterHandlers = append(node.AfterHandlers, handlers...)
	})
}

func EndpointObserve(observers ...any) EndpointOption {
	typed := make([]Observer, 0, len(observers))
	for _, observer := range observers {
		typed = append(typed, mustObserver(observer))
	}
	return endpointOptionFunc(func(node *EndpointNode) {
		node.Observers = append(node.Observers, typed...)
	})
}

func EndpointProvide(bindings ...ProvideBinding) EndpointOption {
	for _, binding := range bindings {
		mustProvideBinding(binding)
	}
	return endpointOptionFunc(func(node *EndpointNode) {
		node.Provides = append(node.Provides, bindings...)
	})
}

func SDKName(name string) EndpointOption {
	return endpointOptionFunc(func(node *EndpointNode) {
		node.SDKName = name
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

func mustProvideBinding(binding ProvideBinding) {
	if ProvideType(binding) == nil || !ProvideValue(binding).IsValid() {
		panic("onedef: provide binding must not be nil")
	}
}

func mustBeforeHandle(handler any) {
	if handler == nil {
		panic("onedef: before handler must not be nil")
	}
	handlerType := reflect.TypeFor[BeforeHandler]()
	t := reflect.TypeOf(handler)
	if t == nil || !t.Implements(handlerType) {
		panic("onedef: before handler must implement BeforeHandle(context.Context) error")
	}
}

func mustAfterHandle(handler any) {
	if handler == nil {
		panic("onedef: after handler must not be nil")
	}
	handlerType := reflect.TypeFor[AfterHandler]()
	t := reflect.TypeOf(handler)
	if t == nil || !t.Implements(handlerType) {
		panic("onedef: after handler must implement AfterHandle(context.Context) error")
	}
}

func mustObserver(observer any) Observer {
	if observer == nil {
		panic("onedef: observer must not be nil")
	}
	value := reflect.ValueOf(observer)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if value.IsNil() {
			panic("onedef: observer must not be nil")
		}
	}
	typed, ok := observer.(Observer)
	if !ok {
		panic("onedef: observer must implement Observe(context.Context, onedef.Outcome)")
	}
	return typed
}
