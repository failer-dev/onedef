package onedef

import (
	"fmt"

	"github.com/failer-dev/onedef/internal/app"
	"github.com/failer-dev/onedef/internal/meta"
)

type GET = meta.GET
type POST = meta.POST
type PUT = meta.PUT
type PATCH = meta.PATCH
type DELETE = meta.DELETE
type HEAD = meta.HEAD
type OPTIONS = meta.OPTIONS

type Node = meta.Node
type GroupRef = meta.GroupRef

type EndpointOption struct {
	option meta.EndpointOption
}

type Spec struct {
	root meta.GroupNode
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

func (s *Spec) GenerateSDK(opts GenerateSDKOptions) error {
	if s == nil {
		return fmt.Errorf("onedef: spec must not be nil")
	}
	return app.New(s.rootNode()).GenerateSDK(opts)
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
		if opt.option != nil {
			metaOpts = append(metaOpts, opt.option)
		}
	}
	return meta.Endpoint(endpoint, metaOpts...)
}

func Endpoints(endpoints ...any) Node {
	return meta.Endpoints(endpoints...)
}

func RequireHeader(name string) Node {
	return meta.RequireHeader(name)
}

func OmitHeader(name string) Node {
	return meta.OmitHeader(name)
}

func SDKName(name string) EndpointOption {
	return EndpointOption{option: meta.SDKName(name)}
}

type App = app.App
type GenerateSDKOptions = app.GenerateSDKOptions
type GenerateIROptions = app.GenerateIROptions

func New(specs ...*Spec) *App {
	return app.New(specRootNodes(specs)...)
}

func specRootNodes(specs []*Spec) []meta.GroupNode {
	roots := make([]meta.GroupNode, 0, len(specs))
	for _, spec := range specs {
		roots = append(roots, spec.rootNode())
	}
	return roots
}
