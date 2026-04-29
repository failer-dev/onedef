package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/failer-dev/onedef/internal/app/testpkgone"
	"github.com/failer-dev/onedef/internal/app/testpkgtwo"
	"github.com/failer-dev/onedef/internal/dartgen"
	"github.com/failer-dev/onedef/internal/meta"
)

type testUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type createUserEndpoint struct {
	meta.POST `path:"/users" status:"201"`
	Request   struct {
		Name string `json:"name"`
	}
	Response testUser
}

func (h *createUserEndpoint) Handle(ctx context.Context) error {
	h.Response = testUser{
		ID:   "new-id",
		Name: h.Request.Name,
	}
	return nil
}

type deleteUserEndpoint struct {
	meta.DELETE `path:"/users/{id}" status:"204"`
	Request     struct{ ID string }
	Response    struct{}
}

func (h *deleteUserEndpoint) Handle(ctx context.Context) error {
	return nil
}

type invalidNoContentEndpoint struct {
	meta.DELETE `path:"/users/{id}" status:"204"`
	Request     struct{ ID string }
	Response    testUser
}

func (h *invalidNoContentEndpoint) Handle(ctx context.Context) error {
	return nil
}

type getUserEndpoint struct {
	meta.GET `path:"/users/{id}"`
	Request  struct{ ID int }
	Response testUser
}

func (h *getUserEndpoint) Handle(ctx context.Context) error {
	h.Response = testUser{
		ID:   "ok",
		Name: "Alice",
	}
	return nil
}

type listUsersEndpoint struct {
	meta.GET `path:"/users"`
	Request  struct{ Page int }
	Response []testUser
}

func (h *listUsersEndpoint) Handle(ctx context.Context) error {
	h.Response = []testUser{}
	return nil
}

type conflictEndpoint struct {
	meta.POST `path:"/conflicts"`
	Request   struct {
		Name string `json:"name"`
	}
	Response testUser
}

func (h *conflictEndpoint) Handle(ctx context.Context) error {
	return meta.Conflict(
		"email_taken",
		"email already in use",
		map[string]any{"field": "email"},
	)
}

type internalFailureEndpoint struct {
	meta.GET `path:"/broken"`
	Request  struct{}
	Response struct{}
}

func (h *internalFailureEndpoint) Handle(ctx context.Context) error {
	return errors.New("boom")
}

type dependencyUserRepo interface {
	FindUser(ctx context.Context, id string) (testUser, error)
}

type dependencyLogger struct {
	calls []string
}

func (l *dependencyLogger) Record(msg string) {
	l.calls = append(l.calls, msg)
}

type staticDependencyRepo struct{}

func (r *staticDependencyRepo) FindUser(_ context.Context, id string) (testUser, error) {
	return testUser{
		ID:   id,
		Name: "Injected",
	}, nil
}

type namedDependencyRepo struct {
	name string
}

func (r *namedDependencyRepo) FindUser(_ context.Context, id string) (testUser, error) {
	return testUser{
		ID:   id,
		Name: r.name,
	}, nil
}

type dependencyEndpoint struct {
	meta.GET `path:"/dependencies/{id}"`
	Request  struct{ ID string }
	Response testUser
	Deps     struct {
		UsersPrimary   dependencyUserRepo
		UsersSecondary dependencyUserRepo
		Logger         *dependencyLogger
	}
}

func (h *dependencyEndpoint) Handle(ctx context.Context) error {
	if h.Deps.UsersPrimary != h.Deps.UsersSecondary {
		return errors.New("expected the same dependency binding for both repo fields")
	}

	h.Deps.Logger.Record("lookup:" + h.Request.ID)
	user, err := h.Deps.UsersPrimary.FindUser(ctx, h.Request.ID)
	if err != nil {
		return err
	}

	h.Response = user
	return nil
}

func TestAppRegister_PanicsForNoContentWithNonEmptyResponse(t *testing.T) {
	t.Parallel()

	app := New()

	defer func() {
		msg, ok := panicMessage(recover())
		if !ok {
			t.Fatal("expected panic, got nil")
		}
		if !strings.Contains(msg, `status "204"`) {
			t.Fatalf("panic = %q, want status 204 message", msg)
		}
	}()

	app.Register(&invalidNoContentEndpoint{})
}

func TestAppGroup_PanicsForDuplicateDependencyBindingInSameScope(t *testing.T) {
	t.Parallel()

	app := New()
	repo := &staticDependencyRepo{}

	defer func() {
		msg, ok := panicMessage(recover())
		if !ok {
			t.Fatal("expected panic, got nil")
		}
		if !strings.Contains(msg, "already bound in this scope") {
			t.Fatalf("panic = %q, want duplicate binding message", msg)
		}
	}()

	app.Group(
		"/api/v1",
		meta.NewDependencyBinding(repo),
		meta.NewDependencyBinding(repo),
		meta.Endpoint(&dependencyEndpoint{}),
	)
}

func TestAppGroup_PanicsForDuplicateErrorPolicyInSameScope(t *testing.T) {
	t.Parallel()

	app := New()
	policy := meta.ErrorPolicy(func(_ *http.Request, _ error) (int, GroupedSpecTestError) {
		return http.StatusBadRequest, GroupedSpecTestError{}
	})

	defer func() {
		msg, ok := panicMessage(recover())
		if !ok {
			t.Fatal("expected panic, got nil")
		}
		if !strings.Contains(msg, "error policy already declared") {
			t.Fatalf("panic = %q, want duplicate error policy message", msg)
		}
	}()

	app.Group(
		"/api/v1",
		policy,
		policy,
		meta.Endpoint(&GroupedSpecTestEndpoint{}),
	)
}

func TestAppGroup_PanicsForDuplicateEndpointErrorPolicy(t *testing.T) {
	t.Parallel()

	app := New()
	policy := meta.ErrorPolicy(func(_ *http.Request, _ error) (int, GroupedSpecTestError) {
		return http.StatusBadRequest, GroupedSpecTestError{}
	})

	defer func() {
		msg, ok := panicMessage(recover())
		if !ok {
			t.Fatal("expected panic, got nil")
		}
		if !strings.Contains(msg, "endpoint error policy already declared") {
			t.Fatalf("panic = %q, want duplicate endpoint error policy message", msg)
		}
	}()

	app.Group(
		"/api/v1",
		meta.Endpoint(
			&GroupedSpecTestEndpoint{},
			meta.EndpointErrorPolicy(policy),
			meta.EndpointErrorPolicy(policy),
		),
	)
}

func TestAppRun_ErrorsForMissingDependency(t *testing.T) {
	t.Parallel()

	app := New()
	app.Register(&dependencyEndpoint{})

	err := app.Run("localhost:0")
	if err == nil {
		t.Fatal("expected error for missing dependency, got nil")
	}
	if !strings.Contains(err.Error(), "not bound") {
		t.Fatalf("error = %q, want missing dependency message", err)
	}
}

func TestAppHandler_WritesCreatedResponse(t *testing.T) {
	t.Parallel()

	app := newTestApp(t, &createUserEndpoint{})

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"Bob"}`))
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusCreated)
	}

	got := assertSuccessResponse(t, res, http.StatusCreated, "created", "Created", "success")
	data := resSuccessData(t, got)
	if data["id"] != "new-id" || data["name"] != "Bob" {
		t.Fatalf("response data = %+v, want created user", data)
	}
}

func TestAppHandler_WritesOKSuccessEnvelope(t *testing.T) {
	t.Parallel()

	app := newTestApp(t, &getUserEndpoint{})

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	got := assertSuccessResponse(t, res, http.StatusOK, "ok", "OK", "success")
	data := resSuccessData(t, got)
	if data["id"] != "ok" || data["name"] != "Alice" {
		t.Fatalf("response data = %+v, want fetched user", data)
	}
}

func TestAppGroup_RegistersGroupedEndpoint(t *testing.T) {
	t.Parallel()

	app := New()
	app.Group("/api/v1", meta.Group("/users", meta.Endpoint(&GroupedSpecTestEndpoint{})))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/abc", nil)
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	got := assertSuccessResponse(t, res, http.StatusOK, "ok", "OK", "success")
	data := resSuccessData(t, got)
	if data["id"] != "abc" || data["name"] != "Grouped" {
		t.Fatalf("response data = %+v, want grouped user", data)
	}
}

func TestAppGroup_EndpointsNode(t *testing.T) {
	t.Parallel()

	app := New()
	app.Group("/api/v1", meta.Group("/users", meta.Endpoints(&GroupedSpecTestEndpoint{})))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/abc", nil)
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	got := assertSuccessResponse(t, res, http.StatusOK, "ok", "OK", "success")
	data := resSuccessData(t, got)
	if data["id"] != "abc" || data["name"] != "Grouped" {
		t.Fatalf("response data = %+v, want grouped user", data)
	}
}

func TestAppRegister_EndpointsNode(t *testing.T) {
	t.Parallel()

	app := New()
	app.Register(meta.Endpoints(&createUserEndpoint{}))

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"Alice"}`))
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusCreated)
	}
	got := assertSuccessResponse(t, res, http.StatusCreated, "created", "Created", "success")
	data := resSuccessData(t, got)
	if data["id"] != "new-id" || data["name"] != "Alice" {
		t.Fatalf("response data = %+v, want created user", data)
	}
}

func TestAppBuildGroupedSDKSpecJSON_IncludesGroupTreeAndHeaders(t *testing.T) {
	t.Parallel()

	app := New()
	app.Group(
		"/api/v1",
		meta.Group(
			"/users",
			meta.RequireHeader("Authorization"),
			meta.Endpoint(&GroupedSpecTestEndpoint{}),
		),
	)

	specJSON, err := app.buildGroupedSDKSpecJSON()
	if err != nil {
		t.Fatalf("buildGroupedSDKSpecJSON() error = %v", err)
	}

	spec := string(specJSON)
	if !strings.Contains(spec, `"name": "users"`) {
		t.Fatalf("spec = %s, want users group", spec)
	}
	if !strings.Contains(spec, `"requiredHeaders": [`) || !strings.Contains(spec, `"Authorization"`) {
		t.Fatalf("spec = %s, want Authorization header", spec)
	}
	if !strings.Contains(spec, `"path": "/api/v1/users/{id}"`) {
		t.Fatalf("spec = %s, want full grouped path", spec)
	}
}

func TestAppBuildGroupedSDKSpecJSON_IncludesEndpointSDKName(t *testing.T) {
	t.Parallel()

	app := New()
	app.Group(
		"/api/v1",
		meta.Group(
			"/users",
			meta.Endpoint(&GroupedSpecTestEndpoint{}, meta.SDKName("get")),
		),
		meta.Group(
			"/orders",
			meta.Endpoint(&CreateOrderSpecTestEndpoint{}),
		),
	)

	specJSON, err := app.buildGroupedSDKSpecJSON()
	if err != nil {
		t.Fatalf("buildGroupedSDKSpecJSON() error = %v", err)
	}

	spec := decodeGroupedSDKSpec(t, specJSON)
	users := findSDKGroupByID(t, spec.Groups, "users")
	if got := users.Endpoints[0].Name; got != "GroupedSpecTestEndpoint" {
		t.Fatalf("endpoint name = %q, want %q", got, "GroupedSpecTestEndpoint")
	}
	if got := users.Endpoints[0].SDKName; got != "get" {
		t.Fatalf("endpoint sdk name = %q, want %q", got, "get")
	}

	orders := findSDKGroupByID(t, spec.Groups, "orders")
	if got := orders.Endpoints[0].SDKName; got != "" {
		t.Fatalf("endpoint sdk name = %q, want empty", got)
	}
	if strings.Contains(string(specJSON), `"sdkName": ""`) {
		t.Fatalf("spec = %s, should omit empty sdkName", specJSON)
	}
}

func TestAppErrorPolicy_GroupAndEndpointOverride(t *testing.T) {
	t.Parallel()

	app := New()
	app.Group(
		"/api",
		meta.ErrorPolicy(func(_ *http.Request, _ error) (int, GroupedSpecTestError) {
			return http.StatusBadRequest, GroupedSpecTestError{Source: "group"}
		}),
		meta.Group(
			"/group",
			meta.Endpoint(&middlewareErrorEndpoint{}),
		),
		meta.Group(
			"/endpoint",
			meta.Endpoint(
				&middlewareErrorEndpoint{},
				meta.EndpointErrorPolicy(meta.ErrorPolicy(func(_ *http.Request, _ error) (int, GroupedSpecEndpointError) {
					return http.StatusConflict, GroupedSpecEndpointError{Source: "endpoint"}
				})),
			),
		),
	)

	groupReq := httptest.NewRequest(http.MethodGet, "/api/group", nil)
	groupRes := httptest.NewRecorder()
	app.mux.ServeHTTP(groupRes, groupReq)
	if groupRes.Code != http.StatusBadRequest {
		t.Fatalf("group status = %d, want %d", groupRes.Code, http.StatusBadRequest)
	}
	var groupBody GroupedSpecTestError
	decodeJSONBody(t, bytes.NewReader(groupRes.Body.Bytes()), &groupBody)
	if groupBody.Source != "group" {
		t.Fatalf("group body = %#v, want group", groupBody)
	}

	endpointReq := httptest.NewRequest(http.MethodGet, "/api/endpoint", nil)
	endpointRes := httptest.NewRecorder()
	app.mux.ServeHTTP(endpointRes, endpointReq)
	if endpointRes.Code != http.StatusConflict {
		t.Fatalf("endpoint status = %d, want %d", endpointRes.Code, http.StatusConflict)
	}
	var endpointBody GroupedSpecEndpointError
	decodeJSONBody(t, bytes.NewReader(endpointRes.Body.Bytes()), &endpointBody)
	if endpointBody.Source != "endpoint" {
		t.Fatalf("endpoint body = %#v, want endpoint", endpointBody)
	}
}

func TestAppErrorPolicy_SubgroupOverrideReflectedInIR(t *testing.T) {
	t.Parallel()

	app := New()
	app.Group(
		"/api/v1",
		meta.ErrorPolicy(func(_ *http.Request, _ error) (int, GroupedSpecTestError) {
			return http.StatusBadRequest, GroupedSpecTestError{}
		}),
		meta.Group(
			"/users",
			meta.Endpoint(&GroupedSpecTestEndpoint{}),
		),
		meta.Group(
			"/orders",
			meta.ErrorPolicy(func(_ *http.Request, _ error) (int, GroupedSpecEndpointError) {
				return http.StatusConflict, GroupedSpecEndpointError{}
			}),
			meta.Endpoint(&CreateOrderSpecTestEndpoint{}),
		),
	)

	specJSON, err := app.buildGroupedSDKSpecJSON()
	if err != nil {
		t.Fatalf("buildGroupedSDKSpecJSON() error = %v", err)
	}

	spec := decodeGroupedSDKSpec(t, specJSON)
	users := findSDKGroupByID(t, spec.Groups, "users")
	if got := users.Endpoints[0].Error.Body.Name; got != "GroupedSpecTestError" {
		t.Fatalf("users error body = %q, want GroupedSpecTestError", got)
	}
	orders := findSDKGroupByID(t, spec.Groups, "orders")
	if got := orders.Endpoints[0].Error.Body.Name; got != "GroupedSpecEndpointError" {
		t.Fatalf("orders error body = %q, want GroupedSpecEndpointError", got)
	}
	if !sdkSpecHasType(spec, "GroupedSpecEndpointError") {
		t.Fatalf("spec types = %#v, want GroupedSpecEndpointError", spec.Types)
	}
}

func TestAppBuildGroupedSDKSpecJSON_AllowsDuplicateLeafGroupsUnderDifferentParents(t *testing.T) {
	t.Parallel()

	app := New()
	app.Group(
		"/api/v1",
		meta.Group(
			"/branch",
			meta.RequireHeader("Authorization"),
			meta.Group(
				"/booking",
				meta.RequireHeader("X-Branch-Id"),
				meta.Endpoint(&GroupedSpecTestEndpoint{}),
			),
		),
		meta.Group(
			"/customer",
			meta.RequireHeader("Authorization"),
			meta.Group(
				"/booking",
				meta.Endpoint(&GroupedSpecTestEndpoint{}),
			),
		),
	)

	specJSON, err := app.buildGroupedSDKSpecJSON()
	if err != nil {
		t.Fatalf("buildGroupedSDKSpecJSON() error = %v", err)
	}

	spec := decodeGroupedSDKSpec(t, specJSON)
	branchBooking := findSDKGroupByID(t, spec.Groups, "branch.booking")
	customerBooking := findSDKGroupByID(t, spec.Groups, "customer.booking")

	if got := strings.Join(branchBooking.PathSegments, "."); got != "branch.booking" {
		t.Fatalf("branch booking path segments = %q, want %q", got, "branch.booking")
	}
	if got := strings.Join(customerBooking.PathSegments, "."); got != "customer.booking" {
		t.Fatalf("customer booking path segments = %q, want %q", got, "customer.booking")
	}
}

func TestAppHandler_OptionalHeaderParameter(t *testing.T) {
	t.Parallel()

	app := New()
	app.Register(&CreateOrderSpecTestEndpoint{})

	req := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(`{"name":"A"}`))
	req.Header.Set("Idempotency-Key", "req-1")
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	got := assertSuccessResponse(t, res, http.StatusCreated, "created", "Created", "success")
	data := resSuccessData(t, got)
	if data["id"] != "req-1" {
		t.Fatalf("response data = %+v, want req-1", data)
	}
}

func TestAppBuildGroupedSDKSpecJSON_OptionalHeaderParamMarkedOptional(t *testing.T) {
	t.Parallel()

	app := New()
	app.Group(
		"/api/v1",
		meta.Group(
			"/orders",
			meta.Endpoint(&CreateOrderSpecTestEndpoint{}),
		),
	)

	specJSON, err := app.buildGroupedSDKSpecJSON()
	if err != nil {
		t.Fatalf("buildGroupedSDKSpecJSON() error = %v", err)
	}

	spec := string(specJSON)
	if !strings.Contains(spec, `"wireName": "X-Request-Id"`) {
		t.Fatalf("spec = %s, want optional request header", spec)
	}
	if !strings.Contains(spec, `"required": false`) {
		t.Fatalf("spec = %s, want optional header required=false", spec)
	}
}

func TestAppBuildGroupedSDKSpecJSON_UsesQualifiedEndpointIdentity(t *testing.T) {
	t.Parallel()

	app := New()
	app.Group(
		"/api/v1",
		meta.Group(
			"/one",
			meta.Endpoint(&testpkgone.Lookup{}),
		),
		meta.Group(
			"/two",
			meta.Endpoint(&testpkgtwo.Lookup{}),
		),
	)

	specJSON, err := app.buildGroupedSDKSpecJSON()
	if err != nil {
		t.Fatalf("buildGroupedSDKSpecJSON() error = %v", err)
	}

	spec := decodeGroupedSDKSpec(t, specJSON)
	groupOne := findSDKGroupByID(t, spec.Groups, "one")
	groupTwo := findSDKGroupByID(t, spec.Groups, "two")

	if got := groupOne.Endpoints[0].Request.PathParams[0].WireName; got != "id" {
		t.Fatalf("group one path param = %q, want %q", got, "id")
	}
	if got := groupTwo.Endpoints[0].Request.PathParams[0].WireName; got != "slug" {
		t.Fatalf("group two path param = %q, want %q", got, "slug")
	}
}

func TestAppGenerateIRJSON_BuildsGroupedSpecWithInitialisms(t *testing.T) {
	t.Parallel()

	app := New()
	app.Group(
		"/api/v1",
		meta.Group(
			"/users",
			meta.RequireHeader("Authorization"),
			meta.Endpoint(&GroupedSpecTestEndpoint{}),
		),
	)

	specJSON, err := app.GenerateIRJSON(GenerateIROptions{
		Initialisms: []string{"ID"},
	})
	if err != nil {
		t.Fatalf("GenerateIRJSON() error = %v", err)
	}

	spec := decodeGroupedSDKSpec(t, specJSON)
	if spec.Naming == nil || strings.Join(spec.Naming.Initialisms, ",") != "ID" {
		t.Fatalf("Naming = %#v, want ID", spec.Naming)
	}
	users := findSDKGroupByID(t, spec.Groups, "users")
	if len(users.Endpoints) != 1 {
		t.Fatalf("len(users.Endpoints) = %d, want 1", len(users.Endpoints))
	}
	if got := users.Endpoints[0].Path; got != "/api/v1/users/{id}" {
		t.Fatalf("endpoint path = %q, want grouped full path", got)
	}
	if !containsStringFold(users.Endpoints[0].RequiredHeaders, "Authorization") {
		t.Fatalf("required headers = %#v, want Authorization", users.Endpoints[0].RequiredHeaders)
	}
}

func TestAppGenerateSDK_EmbedsInitialismsInSpec(t *testing.T) {
	original := generateDartPackage
	t.Cleanup(func() {
		generateDartPackage = original
	})

	var gotSpec []byte
	generateDartPackage = func(opts dartgen.GenerateOptions) error {
		gotSpec = append([]byte(nil), opts.SpecJSON...)
		return nil
	}

	app := New()
	app.Register(&getUserEndpoint{})

	err := app.GenerateSDK(GenerateSDKOptions{
		OutDir:      t.TempDir(),
		PackageName: "api",
		Initialisms: []string{"OAuth", "JWT"},
	})
	if err != nil {
		t.Fatalf("GenerateSDK() error = %v", err)
	}

	var spec sdkSpec
	if err := json.Unmarshal(gotSpec, &spec); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if spec.Naming == nil || strings.Join(spec.Naming.Initialisms, ",") != "OAuth,JWT" {
		t.Fatalf("Naming = %#v, want OAuth/JWT", spec.Naming)
	}
}

func TestPathLeafWarning_DetectsRepeatedLeafWithoutRejecting(t *testing.T) {
	t.Parallel()

	warning, ok := pathLeafWarning(meta.EndpointStruct{
		StructName: "ListProducts",
		GroupPath:  []string{"products"},
		LeafPath:   "/products",
		Path:       "/api/v1/products/products",
	})
	if !ok {
		t.Fatal("pathLeafWarning() ok = false, want true")
	}
	if !strings.Contains(warning, `path:""`) || !strings.Contains(warning, "/api/v1/products/products") {
		t.Fatalf("warning = %q, want path hint and full path", warning)
	}

	_, ok = pathLeafWarning(meta.EndpointStruct{
		StructName: "CreateBooking",
		GroupPath:  []string{"branch", "booking"},
		LeafPath:   "/bookings",
		Path:       "/api/v1/branch/booking/bookings",
	})
	if !ok {
		t.Fatal("pathLeafWarning() ok = false for singular/plural match, want true")
	}
}

func TestPathLeafWarning_IgnoresRootEmptyAndPathParamLeaves(t *testing.T) {
	t.Parallel()

	cases := []meta.EndpointStruct{
		{StructName: "Create", GroupPath: []string{"salons"}, LeafPath: "/", Path: "/api/v1/salons"},
		{StructName: "Create", GroupPath: []string{"salons"}, LeafPath: "", Path: "/api/v1/salons"},
		{StructName: "Find", GroupPath: []string{"salons"}, LeafPath: "/{id}", Path: "/api/v1/salons/{id}"},
	}
	for _, tc := range cases {
		if warning, ok := pathLeafWarning(tc); ok {
			t.Fatalf("pathLeafWarning(%q) = %q, want no warning", tc.LeafPath, warning)
		}
	}
}

func TestAppGroup_OmitHeaderRemovesInheritedAuth(t *testing.T) {
	t.Parallel()

	app := New()
	app.Group(
		"/api/v1",
		meta.Group(
			"/users",
			meta.RequireHeader("Authorization"),
			meta.Group(
				"/public",
				meta.OmitHeader("Authorization"),
				meta.Endpoint(&GroupedSpecTestEndpoint{}),
			),
		),
	)

	specJSON, err := app.buildGroupedSDKSpecJSON()
	if err != nil {
		t.Fatalf("buildGroupedSDKSpecJSON() error = %v", err)
	}

	spec := decodeGroupedSDKSpec(t, specJSON)
	public := findSDKGroupByID(t, spec.Groups, "users.public")
	if containsStringFold(public.Endpoints[0].RequiredHeaders, "Authorization") {
		t.Fatalf("public endpoint headers = %#v, should not contain Authorization", public.Endpoints[0].RequiredHeaders)
	}
}

func TestAppGroup_OmitHeaderMatchesAuthorizationExactlyAndCaseInsensitively(t *testing.T) {
	t.Parallel()

	app := New()
	app.Group(
		"/api/v1",
		meta.Group(
			"/users",
			meta.RequireHeader("authorization"),
			meta.RequireHeader("X-Authorization-Token"),
			meta.Group(
				"/public",
				meta.OmitHeader("Authorization"),
				meta.Endpoint(&GroupedSpecTestEndpoint{}),
			),
		),
	)

	specJSON, err := app.buildGroupedSDKSpecJSON()
	if err != nil {
		t.Fatalf("buildGroupedSDKSpecJSON() error = %v", err)
	}

	spec := decodeGroupedSDKSpec(t, specJSON)
	public := findSDKGroupByID(t, spec.Groups, "users.public")
	headers := public.Endpoints[0].RequiredHeaders
	if containsStringFold(headers, "Authorization") {
		t.Fatalf("public endpoint headers = %#v, should not contain Authorization", headers)
	}
	if !containsStringFold(headers, "X-Authorization-Token") {
		t.Fatalf("public endpoint headers = %#v, want X-Authorization-Token", headers)
	}
}

func TestAppGroup_PanicsForDuplicateEndpointHeaderInheritedFromGroup(t *testing.T) {
	t.Parallel()

	app := New()
	defer func() {
		msg, ok := panicMessage(recover())
		if !ok {
			t.Fatal("expected panic, got nil")
		}
		if !strings.Contains(msg, "duplicate required header") {
			t.Fatalf("panic = %q, want duplicate required header message", msg)
		}
	}()

	app.Group(
		"/api/v1",
		meta.Group(
			"/secure",
			meta.RequireHeader("Authorization"),
			meta.Endpoint(&DuplicateAuthorizationHeaderSpecTestEndpoint{}),
		),
	)
}

func TestAppGroup_OmitHeaderPanicsForNonAuth(t *testing.T) {
	t.Parallel()

	app := New()
	defer func() {
		msg, ok := panicMessage(recover())
		if !ok {
			t.Fatal("expected panic, got nil")
		}
		if !strings.Contains(msg, "not supported") {
			t.Fatalf("panic = %q, want not supported message", msg)
		}
	}()

	app.Group(
		"/api/v1",
		meta.Group(
			"/users",
			meta.RequireHeader("Authorization"),
			meta.Group(
				"/public",
				meta.OmitHeader("X-Custom-Id"),
				meta.Endpoint(&GroupedSpecTestEndpoint{}),
			),
		),
	)
}

func TestAppGroup_OmitHeaderPanicsWithoutInheritedAuth(t *testing.T) {
	t.Parallel()

	app := New()
	defer func() {
		msg, ok := panicMessage(recover())
		if !ok {
			t.Fatal("expected panic, got nil")
		}
		if !strings.Contains(msg, "not required by any parent group") {
			t.Fatalf("panic = %q, want not required message", msg)
		}
	}()

	app.Group(
		"/api/v1",
		meta.Group(
			"/public",
			meta.OmitHeader("Authorization"),
			meta.Endpoint(&GroupedSpecTestEndpoint{}),
		),
	)
}

func TestAppHandler_WritesNoContent(t *testing.T) {
	t.Parallel()

	app := newTestApp(t, &deleteUserEndpoint{})

	req := httptest.NewRequest(http.MethodDelete, "/users/123", nil)
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
	if strings.TrimSpace(res.Body.String()) != "" {
		t.Fatalf("body = %q, want empty", res.Body.String())
	}
}

func TestAppHandler_InvalidBody(t *testing.T) {
	t.Parallel()

	app := newTestApp(t, &createUserEndpoint{})

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":`))
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	assertErrorResponse(t, res, http.StatusBadRequest, "invalid_body", "Bad Request", "request body is invalid JSON")

	data := resErrorData(t, res)
	if _, ok := data["error"].(string); !ok {
		t.Fatalf("data.error = %#v, want string", data["error"])
	}
}

func TestAppHandler_InvalidPathParameter(t *testing.T) {
	t.Parallel()

	app := newTestApp(t, &getUserEndpoint{})

	req := httptest.NewRequest(http.MethodGet, "/users/not-int", nil)
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	assertErrorResponse(t, res, http.StatusBadRequest, "invalid_path_parameter", "Bad Request", `cannot convert path value "not-int"`)

	data := resErrorData(t, res)
	if got := data["parameter"]; got != "id" {
		t.Fatalf("data.parameter = %#v, want %q", got, "id")
	}
}

func TestAppHandler_InvalidQueryParameter(t *testing.T) {
	t.Parallel()

	app := newTestApp(t, &listUsersEndpoint{})

	req := httptest.NewRequest(http.MethodGet, "/users?page=abc", nil)
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	assertErrorResponse(t, res, http.StatusBadRequest, "invalid_query_parameter", "Bad Request", `cannot convert path value "abc"`)

	data := resErrorData(t, res)
	if got := data["parameter"]; got != "page" {
		t.Fatalf("data.parameter = %#v, want %q", got, "page")
	}
}

func TestAppHandler_HTTPErrorPassThrough(t *testing.T) {
	t.Parallel()

	app := newTestApp(t, &conflictEndpoint{})

	req := httptest.NewRequest(http.MethodPost, "/conflicts", strings.NewReader(`{"name":"taken"}`))
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	assertErrorResponse(t, res, http.StatusConflict, "email_taken", "Conflict", "email already in use")

	data := resErrorData(t, res)
	if got := data["field"]; got != "email" {
		t.Fatalf("data.field = %#v, want %q", got, "email")
	}
}

func TestAppHandler_InternalErrorMasked(t *testing.T) {
	t.Parallel()

	app := newTestApp(t, &internalFailureEndpoint{})

	req := httptest.NewRequest(http.MethodGet, "/broken", nil)
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	assertErrorResponse(t, res, http.StatusInternalServerError, "internal_error", "Internal Server Error", "internal server error")

	if data := resErrorData(t, res); data != nil {
		t.Fatalf("data = %#v, want nil", data)
	}
}

func TestAppHandler_InjectsGroupScopedDependencies(t *testing.T) {
	t.Parallel()

	app := New()
	repo := &staticDependencyRepo{}
	logger := &dependencyLogger{}

	app.Group(
		"/api",
		meta.NewDependencyBinding(repo),
		meta.NewDependencyBinding[dependencyUserRepo](repo),
		meta.NewDependencyBinding(logger),
		meta.Endpoint(&dependencyEndpoint{}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/dependencies/abc", nil)
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	got := assertSuccessResponse(t, res, http.StatusOK, "ok", "OK", "success")
	data := resSuccessData(t, got)
	if data["id"] != "abc" || data["name"] != "Injected" {
		t.Fatalf("response data = %+v, want injected user", data)
	}
	if len(logger.calls) != 1 || logger.calls[0] != "lookup:abc" {
		t.Fatalf("logger calls = %#v, want lookup:abc", logger.calls)
	}
}

func TestAppHandler_InheritsParentScopedDependencies(t *testing.T) {
	t.Parallel()

	app := New()
	repo := &namedDependencyRepo{name: "Parent"}
	logger := &dependencyLogger{}

	app.Group(
		"/api",
		meta.NewDependencyBinding[dependencyUserRepo](repo),
		meta.NewDependencyBinding(logger),
		meta.Group(
			"/child",
			meta.Endpoint(&dependencyEndpoint{}),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/child/dependencies/abc", nil)
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	got := assertSuccessResponse(t, res, http.StatusOK, "ok", "OK", "success")
	data := resSuccessData(t, got)
	if data["id"] != "abc" || data["name"] != "Parent" {
		t.Fatalf("response data = %+v, want parent dependency", data)
	}
}

func TestAppHandler_ChildDependencyShadowsParent(t *testing.T) {
	t.Parallel()

	app := New()
	parentRepo := &namedDependencyRepo{name: "Parent"}
	childRepo := &namedDependencyRepo{name: "Child"}
	logger := &dependencyLogger{}

	app.Group(
		"/api",
		meta.NewDependencyBinding[dependencyUserRepo](parentRepo),
		meta.NewDependencyBinding(logger),
		meta.Group(
			"/child",
			meta.NewDependencyBinding[dependencyUserRepo](childRepo),
			meta.Endpoint(&dependencyEndpoint{}),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/child/dependencies/abc", nil)
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	got := assertSuccessResponse(t, res, http.StatusOK, "ok", "OK", "success")
	data := resSuccessData(t, got)
	if data["id"] != "abc" || data["name"] != "Child" {
		t.Fatalf("response data = %+v, want child dependency", data)
	}
}

func TestAppHandler_EndpointDependencyShadowsGroup(t *testing.T) {
	t.Parallel()

	app := New()
	parentRepo := &namedDependencyRepo{name: "Parent"}
	endpointRepo := &namedDependencyRepo{name: "Endpoint"}
	logger := &dependencyLogger{}

	app.Group(
		"/api",
		meta.NewDependencyBinding[dependencyUserRepo](parentRepo),
		meta.NewDependencyBinding(logger),
		meta.Endpoint(
			&dependencyEndpoint{},
			meta.EndpointDependency(meta.NewDependencyBinding[dependencyUserRepo](endpointRepo)),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/dependencies/abc", nil)
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	got := assertSuccessResponse(t, res, http.StatusOK, "ok", "OK", "success")
	data := resSuccessData(t, got)
	if data["id"] != "abc" || data["name"] != "Endpoint" {
		t.Fatalf("response data = %+v, want endpoint dependency", data)
	}
}

func TestAppNewWithDefinitionUsesScopedDependencies(t *testing.T) {
	t.Parallel()

	repo := &staticDependencyRepo{}
	logger := &dependencyLogger{}
	app := New(meta.GroupNode{
		Path: "/api/v1",
		Children: []meta.Node{
			meta.NewDependencyBinding(repo),
			meta.NewDependencyBinding[dependencyUserRepo](repo),
			meta.NewDependencyBinding(logger),
			meta.Group("/deps", meta.Endpoint(&dependencyEndpoint{})),
		},
	})
	app.finalizeDefinitions()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/deps/dependencies/abc", nil)
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	got := assertSuccessResponse(t, res, http.StatusOK, "ok", "OK", "success")
	data := resSuccessData(t, got)
	if data["id"] != "abc" || data["name"] != "Injected" {
		t.Fatalf("response data = %+v, want injected user", data)
	}
	if len(logger.calls) != 1 || logger.calls[0] != "lookup:abc" {
		t.Fatalf("logger calls = %#v, want lookup:abc", logger.calls)
	}
}

func newTestApp(t *testing.T, endpoints ...any) *App {
	t.Helper()

	app := New()
	app.Register(endpoints...)
	return app
}

func decodeJSONBody(t *testing.T, body io.Reader, target any) {
	t.Helper()

	if err := json.NewDecoder(body).Decode(target); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
}

func assertErrorResponse(t *testing.T, res *httptest.ResponseRecorder, wantStatus int, wantCode, _ string, wantMessageSubstring string) {
	t.Helper()

	if res.Code != wantStatus {
		t.Fatalf("status = %d, want %d", res.Code, wantStatus)
	}

	var got meta.DefaultError
	decodeJSONBody(t, bytes.NewReader(res.Body.Bytes()), &got)

	if got.Code != wantCode {
		t.Fatalf("code = %q, want %q", got.Code, wantCode)
	}
	if !strings.Contains(got.Message, wantMessageSubstring) {
		t.Fatalf("message = %q, want substring %q", got.Message, wantMessageSubstring)
	}
}

func assertSuccessResponse(t *testing.T, res *httptest.ResponseRecorder, wantStatus int, wantCode, wantTitle, wantMessage string) responseEnvelope {
	t.Helper()

	if res.Code != wantStatus {
		t.Fatalf("status = %d, want %d", res.Code, wantStatus)
	}

	var got responseEnvelope
	decodeJSONBody(t, bytes.NewReader(res.Body.Bytes()), &got)

	if got.Code != wantCode {
		t.Fatalf("code = %q, want %q", got.Code, wantCode)
	}
	if got.Title != wantTitle {
		t.Fatalf("title = %q, want %q", got.Title, wantTitle)
	}
	if got.Message != wantMessage {
		t.Fatalf("message = %q, want %q", got.Message, wantMessage)
	}

	return got
}

func resErrorData(t *testing.T, res *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var got meta.DefaultError
	decodeJSONBody(t, bytes.NewReader(res.Body.Bytes()), &got)

	if got.Details == nil {
		return nil
	}

	data, ok := got.Details.(map[string]any)
	if !ok {
		t.Fatalf("details = %#v, want map[string]any", got.Details)
	}
	return data
}

func resSuccessData(t *testing.T, got responseEnvelope) map[string]any {
	t.Helper()

	data, ok := got.Data.(map[string]any)
	if !ok {
		t.Fatalf("data = %#v, want map[string]any", got.Data)
	}
	return data
}

func decodeGroupedSDKSpec(t *testing.T, specJSON []byte) sdkSpec {
	t.Helper()

	var spec sdkSpec
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return spec
}

func findSDKGroupByID(t *testing.T, groups []sdkGroup, id string) sdkGroup {
	t.Helper()

	for _, group := range groups {
		if group.ID == id {
			return group
		}
		if found := findSDKGroupByIDOrEmpty(group.Groups, id); found != nil {
			return *found
		}
	}

	t.Fatalf("group %q not found", id)
	return sdkGroup{}
}

func findSDKGroupByIDOrEmpty(groups []sdkGroup, id string) *sdkGroup {
	for _, group := range groups {
		if group.ID == id {
			return &group
		}
		if found := findSDKGroupByIDOrEmpty(group.Groups, id); found != nil {
			return found
		}
	}
	return nil
}

func sdkSpecHasType(spec sdkSpec, name string) bool {
	for _, typeDef := range spec.Types {
		if typeDef.Name == name {
			return true
		}
	}
	return false
}

func containsStringFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

func panicMessage(v any) (string, bool) {
	if v == nil {
		return "", false
	}
	if err, ok := v.(error); ok {
		return err.Error(), true
	}
	if msg, ok := v.(string); ok {
		return msg, true
	}
	return "", false
}
