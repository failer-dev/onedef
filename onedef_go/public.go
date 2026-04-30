package onedef

import (
	"fmt"
	"net/http"
	"time"

	"github.com/failer-dev/onedef/onedef_go/internal/app"
	"github.com/failer-dev/onedef/onedef_go/internal/meta"
)

type GET = meta.GET
type POST = meta.POST
type PUT = meta.PUT
type PATCH = meta.PATCH
type DELETE = meta.DELETE
type HEAD = meta.HEAD
type OPTIONS = meta.OPTIONS

type App = app.App
type DefaultError = meta.DefaultError
type GenerateIROptions = app.GenerateIROptions
type HeaderOption = meta.HeaderOption
type HeaderSymbol = meta.HeaderSymbol
type HTTPError = meta.HTTPError
type Node = meta.Node
type ObserverFunc = meta.ObserverFunc
type Outcome = meta.Outcome
type ProvideBinding = meta.ProvideBinding
type RunOption = app.RunOption

type ErrorMapper[T any] func(*http.Request, error) (statusCode int, body T)

type EndpointOption interface {
	endpointOption() meta.EndpointOption
}

type endpointOption struct {
	option meta.EndpointOption
}

func (o endpointOption) endpointOption() meta.EndpointOption {
	return o.option
}

type RequiredHeader struct {
	node meta.RequireHeaderNode
}

func (h RequiredHeader) MetaNode() meta.Node {
	return h.node
}

func (h RequiredHeader) endpointOption() meta.EndpointOption {
	return h.node
}

type BeforeHandleNode struct {
	node meta.BeforeHandleNode
}

func (n BeforeHandleNode) MetaNode() meta.Node {
	return n.node
}

func (n BeforeHandleNode) endpointOption() meta.EndpointOption {
	return n.node
}

type AfterHandleNode struct {
	node meta.AfterHandleNode
}

func (n AfterHandleNode) MetaNode() meta.Node {
	return n.node
}

func (n AfterHandleNode) endpointOption() meta.EndpointOption {
	return n.node
}

type ObserveNode struct {
	node meta.ObserveNode
}

func (n ObserveNode) MetaNode() meta.Node {
	return n.node
}

func (n ObserveNode) endpointOption() meta.EndpointOption {
	return n.node
}

type provideNode struct {
	binding meta.ProvideBinding
}

func (n provideNode) MetaNode() meta.Node {
	return n.binding
}

func (n provideNode) endpointOption() meta.EndpointOption {
	return meta.EndpointProvide(n.binding)
}

type Spec struct {
	root meta.GroupNode
}

func Header[T any](wireName string, opts ...HeaderOption) HeaderSymbol {
	return meta.NewHeader[T](wireName, opts...)
}

func ParseHeader[T any](parse func(string) (T, error)) HeaderOption {
	return meta.HeaderParse(parse)
}

func Name(name string) HeaderOption {
	return meta.HeaderName(name)
}

func Description(description string) HeaderOption {
	return meta.HeaderDescription(description)
}

func Example(example string) HeaderOption {
	return meta.HeaderExample(example)
}

func Group(path string, children ...Node) *Spec {
	return &Spec{
		root: meta.GroupNode{
			Path:     path,
			Children: append([]meta.Node(nil), children...),
			Exposed:  true,
		},
	}
}

func (s *Spec) MetaNode() meta.Node {
	if s == nil {
		panic("onedef: spec must not be nil")
	}
	node := s.root
	node.Exposed = true
	return node
}

func (s *Spec) GenerateIRJSON(opts GenerateIROptions) ([]byte, error) {
	if s == nil {
		return nil, fmt.Errorf("onedef: spec must not be nil")
	}
	return app.New(s.rootNode()).GenerateIRJSON(opts)
}

func (s *Spec) rootNode() meta.GroupNode {
	if s == nil {
		panic("onedef: spec must not be nil")
	}
	node := s.root
	node.Exposed = false
	return node
}

func Endpoint(endpoint any, opts ...EndpointOption) Node {
	metaOpts := make([]meta.EndpointOption, 0, len(opts))
	for _, opt := range opts {
		if opt != nil {
			metaOpts = append(metaOpts, opt.endpointOption())
		}
	}
	return meta.Endpoint(endpoint, metaOpts...)
}

func Endpoints(endpoints ...any) Node {
	return meta.Endpoints(endpoints...)
}

func RequireHeader(header HeaderSymbol) RequiredHeader {
	return RequiredHeader{node: meta.RequireHeader(header)}
}

func OmitHeader(header HeaderSymbol) Node {
	return meta.OmitHeader(header)
}

func Provide[T any](value T) provideNode {
	return provideNode{binding: meta.NewProvideBinding[T](value)}
}

func BeforeHandle(handler any) BeforeHandleNode {
	return BeforeHandleNode{node: meta.BeforeHandle(handler)}
}

func AfterHandle(handler any) AfterHandleNode {
	return AfterHandleNode{node: meta.AfterHandle(handler)}
}

func Observe(observer any) ObserveNode {
	return ObserveNode{node: meta.Observe(observer)}
}

func SDKName(name string) EndpointOption {
	return endpointOption{option: meta.SDKName(name)}
}

func ErrorPolicy[T any](mapper ErrorMapper[T]) Node {
	if mapper == nil {
		panic("onedef: error policy mapper must not be nil")
	}
	return meta.ErrorPolicy(func(r *http.Request, err error) (int, T) {
		return mapper(r, err)
	})
}

func EndpointErrorPolicy[T any](mapper ErrorMapper[T]) EndpointOption {
	if mapper == nil {
		panic("onedef: error policy mapper must not be nil")
	}
	return endpointOption{option: meta.EndpointErrorPolicy(meta.ErrorPolicy(func(r *http.Request, err error) (int, T) {
		return mapper(r, err)
	}))}
}

func AsHTTPError(err error) (*HTTPError, bool) {
	return meta.AsHTTPError(err)
}

func NewHTTPError(status int, code, title, message string, data any) *HTTPError {
	return meta.NewHTTPError(status, code, title, message, data)
}

func BadRequest(code, message string, data any) *HTTPError {
	return meta.BadRequest(code, message, data)
}

func Unauthorized(code, message string, data any) *HTTPError {
	return meta.Unauthorized(code, message, data)
}

func Forbidden(code, message string, data any) *HTTPError {
	return meta.Forbidden(code, message, data)
}

func NotFound(code, message string, data any) *HTTPError {
	return meta.NotFound(code, message, data)
}

func Conflict(code, message string, data any) *HTTPError {
	return meta.Conflict(code, message, data)
}

func Unprocessable(code, message string, data any) *HTTPError {
	return meta.Unprocessable(code, message, data)
}

func Internal(code, message string, data any) *HTTPError {
	return meta.Internal(code, message, data)
}

func WithReadHeaderTimeout(d time.Duration) RunOption {
	return app.WithReadHeaderTimeout(d)
}

func WithReadTimeout(d time.Duration) RunOption {
	return app.WithReadTimeout(d)
}

func WithWriteTimeout(d time.Duration) RunOption {
	return app.WithWriteTimeout(d)
}

func WithIdleTimeout(d time.Duration) RunOption {
	return app.WithIdleTimeout(d)
}

func WithMaxHeaderBytes(n int) RunOption {
	return app.WithMaxHeaderBytes(n)
}

func WithMaxBodyBytes(n int64) RunOption {
	return app.WithMaxBodyBytes(n)
}

func New(spec *Spec, specs ...*Spec) *App {
	all := append([]*Spec{spec}, specs...)
	return app.New(specRootNodes(all)...)
}

func specRootNodes(specs []*Spec) []meta.GroupNode {
	roots := make([]meta.GroupNode, 0, len(specs))
	for _, spec := range specs {
		roots = append(roots, spec.rootNode())
	}
	return roots
}
