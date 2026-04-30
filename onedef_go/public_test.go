package onedef_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/failer-dev/onedef/onedef_go"
)

type publicCompileEndpoint struct {
	onedef.GET `path:"/items/{id}"`
	Request    struct{ ID string }
	Response   struct{}
}

func (h *publicCompileEndpoint) Handle(context.Context) error {
	return nil
}

type sdkNameEndpoint struct {
	onedef.GET `path:"/items"`
	Request    struct{}
	Response   struct{}
}

func (h *sdkNameEndpoint) Handle(context.Context) error {
	return nil
}

type externalError struct {
	Message string `json:"message"`
}

type externalBeforeHandle struct {
	Request struct {
		Authorization string `header:"Authorization"`
	}
	Provide struct {
		Value string
	}
}

func (h *externalBeforeHandle) BeforeHandle(context.Context) error {
	h.Provide.Value = h.Request.Authorization
	return nil
}

type externalAfterHandle struct{}

func (h *externalAfterHandle) AfterHandle(context.Context) error {
	return nil
}

type externalObserver struct{}

func (externalObserver) Observe(context.Context, onedef.Outcome) {}

func TestPublicAPISurface(t *testing.T) {
	api := onedef.Group(
		"/api/v1",
		onedef.Group(
			"/items",
			onedef.Endpoint(&publicCompileEndpoint{}, onedef.SDKName("get")),
		),
	)

	app := onedef.New(api)
	if app == nil {
		t.Fatal("New(spec) = nil")
	}
	_ = api.GenerateIRJSON
}

func TestEndpointOptionsCompileExternally(t *testing.T) {
	authorization := onedef.Header[string]("Authorization")
	idempotencyKey := onedef.Header[int](
		"Idempotency-Key",
		onedef.Name("IdempotencyKey"),
		onedef.Description("Idempotency token."),
		onedef.Example("123"),
	)

	api := onedef.Group(
		"/api/v1",
		onedef.RequireHeader(authorization),
		onedef.Provide("root-value"),
		onedef.BeforeHandle(&externalBeforeHandle{}),
		onedef.AfterHandle(&externalAfterHandle{}),
		onedef.Observe(onedef.ObserverFunc(func(context.Context, onedef.Outcome) {})),
		onedef.Endpoint(&sdkNameEndpoint{}, onedef.SDKName("get")),
		onedef.Endpoint(
			&sdkNameEndpoint{},
			onedef.RequireHeader(idempotencyKey),
			onedef.Provide("endpoint-value"),
			onedef.AfterHandle(&externalAfterHandle{}),
			onedef.Observe(externalObserver{}),
		),
	)

	if app := onedef.New(api); app == nil {
		t.Fatal("New(spec) = nil")
	}
}

func TestSpecGenerateIRJSONCompilesExternally(t *testing.T) {
	api := onedef.Group(
		"/api/v1",
		onedef.Group(
			"/users",
			onedef.Endpoint(&publicCompileEndpoint{}),
		),
	)

	specJSON, err := api.GenerateIRJSON(onedef.GenerateIROptions{
		Initialisms: []string{"ID"},
	})
	if err != nil {
		t.Fatalf("GenerateIRJSON() error = %v", err)
	}
	var spec struct {
		Initialisms []string        `json:"initialisms"`
		Models      json.RawMessage `json:"models"`
		Naming      json.RawMessage `json:"naming"`
		Types       json.RawMessage `json:"types"`
	}
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if strings.Join(spec.Initialisms, ",") != "ID" {
		t.Fatalf("Initialisms = %#v, want ID", spec.Initialisms)
	}
	if spec.Models == nil {
		t.Fatal("Models = nil, want emitted")
	}
	if spec.Naming != nil {
		t.Fatalf("Naming = %s, want omitted", spec.Naming)
	}
	if spec.Types != nil {
		t.Fatalf("Types = %s, want omitted", spec.Types)
	}
}

func TestBeforeHandlePublicAPISurfaceCompilesExternally(t *testing.T) {
	authorization := onedef.Header[string]("Authorization")

	api := onedef.Group(
		"/api",
		onedef.ErrorPolicy(func(_ *http.Request, err error) (int, externalError) {
			if httpErr, ok := onedef.AsHTTPError(err); ok {
				return httpErr.Status, externalError{Message: httpErr.Message}
			}
			return http.StatusInternalServerError, externalError{Message: err.Error()}
		}),
		onedef.RequireHeader(authorization),
		onedef.BeforeHandle(&externalBeforeHandle{}),
		onedef.Group(
			"/public",
			onedef.OmitHeader(authorization),
			onedef.Endpoint(&sdkNameEndpoint{}),
		),
		onedef.Group(
			"/items",
			onedef.Endpoint(&sdkNameEndpoint{}),
		),
	)

	app := onedef.New(api)
	if app == nil {
		t.Fatal("New(api) = nil")
	}
}
