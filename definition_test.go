package onedef

import (
	"context"
	"testing"
)

type SpecCompileEndpoint struct {
	GET      `path:"/items/{id}"`
	Request  struct{ ID string }
	Response struct{}
}

func (h *SpecCompileEndpoint) Handle(context.Context) error {
	return nil
}

func TestSpecAPISurface(t *testing.T) {
	api := Group(
		"/api/v1",
		Group(
			"/items",
			Endpoint(&SpecCompileEndpoint{}, SDKName("get")),
		),
	)

	app := New(api)
	if app == nil {
		t.Fatal("New(spec) = nil")
	}
	_ = api.GenerateSDK
	_ = api.GenerateIRJSON

	legacy := New()
	legacy.Group(
		"/api/v1",
		Group(
			"/items",
			Endpoint(&SpecCompileEndpoint{}),
		),
	)
}
