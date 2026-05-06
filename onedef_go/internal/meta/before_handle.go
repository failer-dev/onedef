package meta

import (
	"context"
	"reflect"
	"time"
)

type BeforeHandler interface {
	BeforeHandle(context.Context) error
}

type AfterHandler interface {
	AfterHandle(context.Context) error
}

type Observer interface {
	Observe(context.Context, Outcome)
}

type ObserverFunc func(context.Context, Outcome)

func (f ObserverFunc) Observe(ctx context.Context, outcome Outcome) {
	f(ctx, outcome)
}

type Outcome struct {
	Method   string
	Path     string
	Endpoint string
	Status   int
	Duration time.Duration
	Error    error
}

type BeforeHandleNode struct {
	Handler any
}

func (n BeforeHandleNode) MetaNode() Node { return n }

func (n BeforeHandleNode) applyEndpointOption(node *EndpointNode) {
	node.BeforeHandlers = append(node.BeforeHandlers, n.Handler)
}

type AfterHandleNode struct {
	Handler any
}

func (n AfterHandleNode) MetaNode() Node { return n }

func (n AfterHandleNode) applyEndpointOption(node *EndpointNode) {
	node.AfterHandlers = append(node.AfterHandlers, n.Handler)
}

type ObserveNode struct {
	Observer Observer
}

func (n ObserveNode) MetaNode() Node { return n }

func (n ObserveNode) applyEndpointOption(node *EndpointNode) {
	node.Observers = append(node.Observers, n.Observer)
}

type BeforeHandleStruct struct {
	StructName string
	Request    RequestField
	Provide    ProvideFieldSet
	StructType reflect.Type
}

type AfterHandleStruct struct {
	StructName string
	Request    RequestField
	Provide    ProvideFieldSet
	Response   ResponseFieldSet
	StructType reflect.Type
}
