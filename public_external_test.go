package onedef_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/failer-dev/onedef"
	helloapi "github.com/failer-dev/onedef/example/hello_world/api"
)

type sdkNameEndpoint struct {
	onedef.GET `path:"/items"`
	Request    struct{}
	Response   struct{}
}

func (h *sdkNameEndpoint) Handle(context.Context) error {
	return nil
}

func TestEndpointSDKNameOptionCompilesExternally(t *testing.T) {
	api := onedef.Group(
		"/api/v1",
		onedef.Dependency("root-value"),
		onedef.Endpoint(&sdkNameEndpoint{}, onedef.SDKName("get")),
		onedef.Endpoint(
			&sdkNameEndpoint{},
			onedef.EndpointDependency(onedef.Dependency("endpoint-value")),
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
			onedef.Endpoint(&helloapi.GetUser{}),
		),
	)

	specJSON, err := api.GenerateIRJSON(onedef.GenerateIROptions{
		Initialisms: []string{"ID"},
	})
	if err != nil {
		t.Fatalf("GenerateIRJSON() error = %v", err)
	}
	var spec struct {
		Naming *struct {
			Initialisms []string `json:"initialisms"`
		} `json:"naming"`
	}
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if spec.Naming == nil || strings.Join(spec.Naming.Initialisms, ",") != "ID" {
		t.Fatalf("Naming = %#v, want ID", spec.Naming)
	}
}
