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

	"github.com/failer-dev/onedef/onedef_go/internal/app/testpkgone"
	"github.com/failer-dev/onedef/onedef_go/internal/app/testpkgtwo"
	ir "github.com/failer-dev/onedef/onedef_go/internal/irspec"
	"github.com/failer-dev/onedef/onedef_go/internal/meta"
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
	Provide  struct {
		UsersPrimary   dependencyUserRepo
		UsersSecondary dependencyUserRepo
		Logger         *dependencyLogger
	}
}

func (h *dependencyEndpoint) Handle(ctx context.Context) error {
	if h.Provide.UsersPrimary != h.Provide.UsersSecondary {
		return errors.New("expected the same dependency binding for both repo fields")
	}

	h.Provide.Logger.Record("lookup:" + h.Request.ID)
	user, err := h.Provide.UsersPrimary.FindUser(ctx, h.Request.ID)
	if err != nil {
		return err
	}

	h.Response = user
	return nil
}

type policyErrorEndpoint struct {
	meta.GET `path:""`
	Request  struct{}
	Response struct{}
}

func (h *policyErrorEndpoint) Handle(context.Context) error {
	return errors.New("policy boom")
}

var (
	authorizationHeader  = meta.NewHeader[string]("Authorization")
	branchIDHeader       = meta.NewHeader[string]("X-Branch-Id")
	authorizationToken   = meta.NewHeader[string]("X-Authorization-Token")
	idempotencyHeader    = meta.NewHeader[string]("Idempotency-Key")
	intIdempotencyHeader = meta.NewHeader[int]("Idempotency-Key")
	requestIDHeader      = meta.NewHeader[string]("X-Request-Id")
	bookingScopeHeader   = meta.NewHeader[string]("X-Booking-Scope")
	customIDHeader       = meta.NewHeader[string]("X-Custom-Id")
)

func TestAppDefinition_PanicsForNoContentWithNonEmptyResponse(t *testing.T) {
	t.Parallel()

	defer func() {
		msg, ok := panicMessage(recover())
		if !ok {
			t.Fatal("expected panic, got nil")
		}
		if !strings.Contains(msg, `status "204"`) {
			t.Fatalf("panic = %q, want status 204 message", msg)
		}
	}()

	newTestApp(t, &invalidNoContentEndpoint{})
}

func TestAppGroup_ProvideBindingLastWriterWinsInSameScope(t *testing.T) {
	t.Parallel()

	first := &namedDependencyRepo{name: "First"}
	second := &namedDependencyRepo{name: "Second"}
	logger := &dependencyLogger{}

	app := newGroupedTestApp(
		"/api",
		meta.NewProvideBinding[dependencyUserRepo](first),
		meta.NewProvideBinding[dependencyUserRepo](second),
		meta.NewProvideBinding(logger),
		meta.Endpoint(&dependencyEndpoint{}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/dependencies/abc", nil)
	res := httptest.NewRecorder()
	app.mux.ServeHTTP(res, req)

	got := assertSuccessResponse(t, res, http.StatusOK, "ok", "OK", "success")
	data := resSuccessData(t, got)
	if data["name"] != "Second" {
		t.Fatalf("response data = %+v, want second provider", data)
	}
}

func TestAppGroup_PanicsForDuplicateErrorPolicyInSameScope(t *testing.T) {
	t.Parallel()

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

	newGroupedTestApp(
		"/api/v1",
		policy,
		policy,
		meta.Endpoint(&GroupedSpecTestEndpoint{}),
	)
}

func TestAppGroup_PanicsForDuplicateEndpointErrorPolicy(t *testing.T) {
	t.Parallel()

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

	newGroupedTestApp(
		"/api/v1",
		meta.Endpoint(
			&GroupedSpecTestEndpoint{},
			meta.EndpointErrorPolicy(policy),
			meta.EndpointErrorPolicy(policy),
		),
	)
}

func TestAppHandler_ErrorsForMissingProvide(t *testing.T) {
	t.Parallel()

	app := newTestApp(t, &dependencyEndpoint{})

	req := httptest.NewRequest(http.MethodGet, "/dependencies/abc", nil)
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	assertErrorResponse(t, res, http.StatusInternalServerError, "internal_error", "Internal Server Error", "internal server error")
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

	app := newGroupedTestApp("/api/v1", meta.Group("/users", meta.Endpoint(&GroupedSpecTestEndpoint{})))

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

	app := newGroupedTestApp("/api/v1", meta.Group("/users", meta.Endpoints(&GroupedSpecTestEndpoint{})))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/abc", nil)
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	got := assertSuccessResponse(t, res, http.StatusOK, "ok", "OK", "success")
	data := resSuccessData(t, got)
	if data["id"] != "abc" || data["name"] != "Grouped" {
		t.Fatalf("response data = %+v, want grouped user", data)
	}
}

func TestAppDefinition_EndpointsNode(t *testing.T) {
	t.Parallel()

	app := newTestApp(t, &createUserEndpoint{})

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

	app := newGroupedTestApp(
		"/api/v1",
		meta.Group(
			"/users",
			meta.RequireHeader(authorizationHeader),
			meta.Endpoint(&GroupedSpecTestEndpoint{}),
		),
	)

	specJSON, err := app.buildIRJSON(nil)
	if err != nil {
		t.Fatalf("buildIRJSON() error = %v", err)
	}

	spec := string(specJSON)
	if !strings.Contains(spec, `"name": "users"`) {
		t.Fatalf("spec = %s, want users group", spec)
	}
	if !strings.Contains(spec, `"headers": [`) || !strings.Contains(spec, `"Authorization"`) {
		t.Fatalf("spec = %s, want Authorization header", spec)
	}
	if strings.Contains(spec, `"header": [`) {
		t.Fatalf("spec = %s, should use headers not header", spec)
	}
	if !strings.Contains(spec, `"path": "/api/v1/users/{id}"`) {
		t.Fatalf("spec = %s, want full grouped path", spec)
	}
}

func TestAppBuildGroupedSDKSpecJSON_IncludesEndpointSDKName(t *testing.T) {
	t.Parallel()

	app := newGroupedTestApp(
		"/api/v1",
		meta.Group(
			"/users",
			meta.Endpoint(&GroupedSpecTestEndpoint{}, meta.SDKName("get")),
		),
		meta.Group(
			"/orders",
			createOrderEndpointNode(),
		),
	)

	specJSON, err := app.buildIRJSON(nil)
	if err != nil {
		t.Fatalf("buildIRJSON() error = %v", err)
	}

	spec := decodeIRDocument(t, specJSON)
	users := findIRGroupByPathString(t, spec.Routes.Groups, "users")
	if got := users.Endpoints[0].Name; got != "GroupedSpecTestEndpoint" {
		t.Fatalf("endpoint name = %q, want %q", got, "GroupedSpecTestEndpoint")
	}
	if got := users.Endpoints[0].SDKName; got != "get" {
		t.Fatalf("endpoint sdk name = %q, want %q", got, "get")
	}

	orders := findIRGroupByPathString(t, spec.Routes.Groups, "orders")
	if got := orders.Endpoints[0].SDKName; got != "" {
		t.Fatalf("endpoint sdk name = %q, want empty", got)
	}
	if strings.Contains(string(specJSON), `"sdkName": ""`) {
		t.Fatalf("spec = %s, should omit empty sdkName", specJSON)
	}
}

func TestAppErrorPolicy_GroupAndEndpointOverride(t *testing.T) {
	t.Parallel()

	app := newGroupedTestApp(
		"/api",
		meta.ErrorPolicy(func(_ *http.Request, _ error) (int, GroupedSpecTestError) {
			return http.StatusBadRequest, GroupedSpecTestError{Source: "group"}
		}),
		meta.Group(
			"/group",
			meta.Endpoint(&policyErrorEndpoint{}),
		),
		meta.Group(
			"/endpoint",
			meta.Endpoint(
				&policyErrorEndpoint{},
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

	app := newGroupedTestApp(
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
			createOrderEndpointNode(),
		),
	)

	specJSON, err := app.buildIRJSON(nil)
	if err != nil {
		t.Fatalf("buildIRJSON() error = %v", err)
	}

	spec := decodeIRDocument(t, specJSON)
	users := findIRGroupByPathString(t, spec.Routes.Groups, "users")
	if got := users.Endpoints[0].Error.Body.Name; got != "GroupedSpecTestError" {
		t.Fatalf("users error body = %q, want GroupedSpecTestError", got)
	}
	orders := findIRGroupByPathString(t, spec.Routes.Groups, "orders")
	if got := orders.Endpoints[0].Error.Body.Name; got != "GroupedSpecEndpointError" {
		t.Fatalf("orders error body = %q, want GroupedSpecEndpointError", got)
	}
	if !irDocumentHasModel(spec, "GroupedSpecEndpointError") {
		t.Fatalf("spec models = %#v, want GroupedSpecEndpointError", spec.Models)
	}
}

func TestAppBuildGroupedSDKSpecJSON_AllowsDuplicateLeafGroupsUnderDifferentParents(t *testing.T) {
	t.Parallel()

	app := newGroupedTestApp(
		"/api/v1",
		meta.Group(
			"/branch",
			meta.RequireHeader(authorizationHeader),
			meta.Group(
				"/booking",
				meta.RequireHeader(branchIDHeader),
				meta.Endpoint(&GroupedSpecTestEndpoint{}),
			),
		),
		meta.Group(
			"/customer",
			meta.RequireHeader(authorizationHeader),
			meta.Group(
				"/booking",
				meta.Endpoint(&GroupedSpecTestEndpoint{}),
			),
		),
	)

	specJSON, err := app.buildIRJSON(nil)
	if err != nil {
		t.Fatalf("buildIRJSON() error = %v", err)
	}

	spec := decodeIRDocument(t, specJSON)
	branchBooking := findIRGroupByPathString(t, spec.Routes.Groups, "branch.booking")
	customerBooking := findIRGroupByPathString(t, spec.Routes.Groups, "customer.booking")

	if branchBooking.Name != "booking" {
		t.Fatalf("branch booking name = %q, want booking", branchBooking.Name)
	}
	if customerBooking.Name != "booking" {
		t.Fatalf("customer booking name = %q, want booking", customerBooking.Name)
	}
}

func TestAppHandler_HeaderParameter(t *testing.T) {
	t.Parallel()

	app := newTestAppWithNodes(t, createOrderEndpointNode())

	req := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(`{"name":"A"}`))
	req.Header.Set("Idempotency-Key", "req-1")
	req.Header.Set("X-Request-Id", "request-id")
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	got := assertSuccessResponse(t, res, http.StatusCreated, "created", "Created", "success")
	data := resSuccessData(t, got)
	if data["id"] != "request-id" {
		t.Fatalf("response data = %+v, want request-id", data)
	}
}

func TestAppBuildGroupedSDKSpecJSON_HeaderParamsMarkedRequired(t *testing.T) {
	t.Parallel()

	app := newGroupedTestApp(
		"/api/v1",
		meta.Group(
			"/orders",
			createOrderEndpointNode(),
		),
	)

	specJSON, err := app.buildIRJSON(nil)
	if err != nil {
		t.Fatalf("buildIRJSON() error = %v", err)
	}

	spec := string(specJSON)
	if !strings.Contains(spec, `"key": "X-Request-Id"`) {
		t.Fatalf("spec = %s, want request header", spec)
	}
	if !strings.Contains(spec, `"required": true`) {
		t.Fatalf("spec = %s, want header required=true", spec)
	}
}

func TestAppBuildGroupedSDKSpecJSON_UsesQualifiedEndpointIdentity(t *testing.T) {
	t.Parallel()

	app := newGroupedTestApp(
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

	specJSON, err := app.buildIRJSON(nil)
	if err != nil {
		t.Fatalf("buildIRJSON() error = %v", err)
	}

	spec := decodeIRDocument(t, specJSON)
	groupOne := findIRGroupByPathString(t, spec.Routes.Groups, "one")
	groupTwo := findIRGroupByPathString(t, spec.Routes.Groups, "two")

	if got := groupOne.Endpoints[0].Request.Paths[0].Key; got != "id" {
		t.Fatalf("group one path param = %q, want %q", got, "id")
	}
	if got := groupTwo.Endpoints[0].Request.Paths[0].Key; got != "slug" {
		t.Fatalf("group two path param = %q, want %q", got, "slug")
	}
}

func TestAppGenerateIRJSON_BuildsGroupedSpecWithInitialisms(t *testing.T) {
	t.Parallel()

	app := newGroupedTestApp(
		"/api/v1",
		meta.Group(
			"/users",
			meta.RequireHeader(authorizationHeader),
			meta.Endpoint(&GroupedSpecTestEndpoint{}),
		),
	)

	specJSON, err := app.GenerateIRJSON(GenerateIROptions{
		Initialisms: []string{"ID"},
	})
	if err != nil {
		t.Fatalf("GenerateIRJSON() error = %v", err)
	}

	var topLevel map[string]json.RawMessage
	if err := json.Unmarshal(specJSON, &topLevel); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if _, ok := topLevel["routes"]; !ok {
		t.Fatalf("top-level keys = %#v, want routes", topLevel)
	}
	if _, ok := topLevel["models"]; !ok {
		t.Fatalf("top-level keys = %#v, want models", topLevel)
	}
	if _, ok := topLevel["types"]; ok {
		t.Fatalf("top-level types should be omitted: %s", specJSON)
	}
	if _, ok := topLevel["endpoints"]; ok {
		t.Fatalf("top-level endpoints should be omitted: %s", specJSON)
	}
	if _, ok := topLevel["groups"]; ok {
		t.Fatalf("top-level groups should be omitted: %s", specJSON)
	}
	if strings.Contains(string(specJSON), `"group":`) {
		t.Fatalf("endpoint group field should be omitted: %s", specJSON)
	}
	if strings.Contains(string(specJSON), `"nullable":`) {
		t.Fatalf("field nullable should be represented only in type expressions: %s", specJSON)
	}

	spec := decodeIRDocument(t, specJSON)
	if strings.Join(spec.Initialisms, ",") != "ID" {
		t.Fatalf("Initialisms = %#v, want ID", spec.Initialisms)
	}
	users := findIRGroupByPathString(t, spec.Routes.Groups, "users")
	if len(users.Endpoints) != 1 {
		t.Fatalf("len(users.Endpoints) = %d, want 1", len(users.Endpoints))
	}
	if got := users.Endpoints[0].Path; got != "/api/v1/users/{id}" {
		t.Fatalf("endpoint path = %q, want grouped full path", got)
	}
	if !containsHeaderFold(users.Headers, "Authorization") {
		t.Fatalf("group header = %#v, want Authorization", users.Headers)
	}
}

func TestAppGenerateIRJSON_IncludesUngroupedAndGroupedEndpoints(t *testing.T) {
	t.Parallel()

	app := newGroupedTestApp(
		"/api/v1",
		meta.Endpoint(&createUserEndpoint{}),
		meta.Group("/users", meta.Endpoint(&GroupedSpecTestEndpoint{})),
	)

	specJSON, err := app.GenerateIRJSON(GenerateIROptions{})
	if err != nil {
		t.Fatalf("GenerateIRJSON() error = %v", err)
	}

	spec := decodeIRDocument(t, specJSON)
	if len(spec.Routes.Endpoints) != 1 {
		t.Fatalf("len(Endpoints) = %d, want 1", len(spec.Routes.Endpoints))
	}
	if got := spec.Routes.Endpoints[0].Name; got != "createUserEndpoint" {
		t.Fatalf("flat endpoint name = %q, want createUserEndpoint", got)
	}
	if got := spec.Routes.Endpoints[0].Path; got != "/api/v1/users" {
		t.Fatalf("flat endpoint path = %q, want root-prefixed path", got)
	}
	users := findIRGroupByPathString(t, spec.Routes.Groups, "users")
	if len(users.Endpoints) != 1 {
		t.Fatalf("len(users.Endpoints) = %d, want 1", len(users.Endpoints))
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

	app := newGroupedTestApp(
		"/api/v1",
		meta.Group(
			"/users",
			meta.RequireHeader(authorizationHeader),
			meta.Group(
				"/public",
				meta.OmitHeader(authorizationHeader),
				meta.Endpoint(&GroupedSpecTestEndpoint{}),
			),
		),
	)

	specJSON, err := app.buildIRJSON(nil)
	if err != nil {
		t.Fatalf("buildIRJSON() error = %v", err)
	}

	spec := decodeIRDocument(t, specJSON)
	public := findIRGroupByPathString(t, spec.Routes.Groups, "users.public")
	if containsHeaderFold(public.Headers, "Authorization") {
		t.Fatalf("public group headers = %#v, should not contain Authorization", public.Headers)
	}
}

func TestAppGroup_OmitHeaderMatchesAuthorizationExactlyAndCaseInsensitively(t *testing.T) {
	t.Parallel()

	app := newGroupedTestApp(
		"/api/v1",
		meta.Group(
			"/users",
			meta.RequireHeader(authorizationHeader),
			meta.RequireHeader(authorizationToken),
			meta.Group(
				"/public",
				meta.OmitHeader(authorizationHeader),
				meta.Endpoint(&GroupedSpecTestEndpoint{}),
			),
		),
	)

	specJSON, err := app.buildIRJSON(nil)
	if err != nil {
		t.Fatalf("buildIRJSON() error = %v", err)
	}

	spec := decodeIRDocument(t, specJSON)
	users := findIRGroupByPathString(t, spec.Routes.Groups, "users")
	if !containsHeaderFold(users.Headers, "X-Authorization-Token") {
		t.Fatalf("users group headers = %#v, want X-Authorization-Token", users.Headers)
	}

	public := findIRGroupByPathString(t, spec.Routes.Groups, "users.public")
	headers := public.Headers
	if containsHeaderFold(headers, "Authorization") {
		t.Fatalf("public endpoint headers = %#v, should not contain Authorization", headers)
	}
	if containsHeaderFold(headers, "X-Authorization-Token") {
		t.Fatalf("public endpoint headers = %#v, should inherit X-Authorization-Token from parent only", headers)
	}
}

func TestAppGroup_AllowsStructHeaderBindingForInheritedHeader(t *testing.T) {
	t.Parallel()

	app := newGroupedTestApp(
		"/api/v1",
		meta.Group(
			"/secure",
			meta.RequireHeader(authorizationHeader),
			meta.Endpoint(&GroupHeaderBindingSpecTestEndpoint{}),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/secure", nil)
	req.Header.Set("Authorization", "Bearer token")
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	got := assertSuccessResponse(t, res, http.StatusOK, "ok", "OK", "success")
	data := resSuccessData(t, got)
	if data["authorization"] != "Bearer token" {
		t.Fatalf("response data = %+v, want bound Authorization header", data)
	}

	specJSON, err := app.buildIRJSON(nil)
	if err != nil {
		t.Fatalf("buildIRJSON() error = %v", err)
	}
	spec := decodeIRDocument(t, specJSON)
	secure := findIRGroupByPathString(t, spec.Routes.Groups, "secure")
	if len(secure.Endpoints[0].Request.Headers) != 0 {
		t.Fatalf("header params = %#v, want no method params for group-bound header", secure.Endpoints[0].Request.Headers)
	}
}

func TestAppGroup_PanicsForStructHeaderWithoutRequireHeader(t *testing.T) {
	t.Parallel()

	defer func() {
		msg, ok := panicMessage(recover())
		if !ok {
			t.Fatal("expected panic, got nil")
		}
		if !strings.Contains(msg, "must be declared by RequireHeader") {
			t.Fatalf("panic = %q, want RequireHeader declaration message", msg)
		}
	}()

	newTestApp(t, &CreateOrderSpecTestEndpoint{})
}

func TestAppGroup_PanicsForDuplicateEndpointHeaderInheritedFromGroup(t *testing.T) {
	t.Parallel()

	defer func() {
		msg, ok := panicMessage(recover())
		if !ok {
			t.Fatal("expected panic, got nil")
		}
		if !strings.Contains(msg, "duplicate required header") {
			t.Fatalf("panic = %q, want duplicate required header message", msg)
		}
	}()

	newGroupedTestApp(
		"/api/v1",
		meta.Group(
			"/secure",
			meta.RequireHeader(authorizationHeader),
			meta.Endpoint(&GroupedSpecTestEndpoint{}, meta.RequireHeader(authorizationHeader)),
		),
	)
}

func TestAppGroup_PanicsForDuplicateStructHeaderBinding(t *testing.T) {
	t.Parallel()

	defer func() {
		msg, ok := panicMessage(recover())
		if !ok {
			t.Fatal("expected panic, got nil")
		}
		if !strings.Contains(msg, "duplicate header binding") {
			t.Fatalf("panic = %q, want duplicate header binding message", msg)
		}
	}()

	newGroupedTestApp(
		"/api/v1",
		meta.Endpoint(
			&DuplicateStructHeaderBindingSpecTestEndpoint{},
			meta.RequireHeader(requestIDHeader),
		),
	)
}

func TestAppGroup_EndpointRequireHeaderWithoutStructBindingSynthesizesStringParam(t *testing.T) {
	t.Parallel()

	app := newGroupedTestApp(
		"/api/v1",
		meta.Group(
			"/synthetic",
			meta.Endpoint(
				&SyntheticEndpointHeaderSpecTestEndpoint{},
				meta.RequireHeader(idempotencyHeader),
			),
		),
	)

	specJSON, err := app.buildIRJSON(nil)
	if err != nil {
		t.Fatalf("buildIRJSON() error = %v", err)
	}

	spec := decodeIRDocument(t, specJSON)
	synthetic := findIRGroupByPathString(t, spec.Routes.Groups, "synthetic")
	params := synthetic.Endpoints[0].Request.Headers
	if len(params) != 1 {
		t.Fatalf("len(header params) = %d, want 1", len(params))
	}
	if got := params[0].Name; got != "IdempotencyKey" {
		t.Fatalf("header param name = %q, want IdempotencyKey", got)
	}
	if got := params[0].Type.Kind; got != ir.TypeKindString {
		t.Fatalf("header param type = %q, want string", got)
	}
	if !params[0].Required {
		t.Fatalf("header param required = false, want true")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/synthetic/synthetic-header", nil)
	res := httptest.NewRecorder()
	app.mux.ServeHTTP(res, req)
	assertErrorResponse(t, res, http.StatusBadRequest, "missing_header_parameter", "Bad Request", "required request header is missing")

	req = httptest.NewRequest(http.MethodGet, "/api/v1/synthetic/synthetic-header", nil)
	req.Header.Set("Idempotency-Key", "req-1")
	res = httptest.NewRecorder()
	app.mux.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
}

func TestAppGroup_EndpointRequireHeaderUsesTypedStructBinding(t *testing.T) {
	t.Parallel()

	app := newGroupedTestApp(
		"/api/v1",
		meta.Group(
			"/orders",
			meta.Endpoint(
				&TypedEndpointHeaderSpecTestEndpoint{},
				meta.RequireHeader(intIdempotencyHeader),
			),
		),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/typed-orders", strings.NewReader(`{"name":"A"}`))
	req.Header.Set("Idempotency-Key", "123")
	res := httptest.NewRecorder()
	app.mux.ServeHTTP(res, req)

	got := assertSuccessResponse(t, res, http.StatusCreated, "created", "Created", "success")
	data := resSuccessData(t, got)
	if data["id"] != float64(123) {
		t.Fatalf("response data = %+v, want typed header value 123", data)
	}

	specJSON, err := app.buildIRJSON(nil)
	if err != nil {
		t.Fatalf("buildIRJSON() error = %v", err)
	}
	spec := decodeIRDocument(t, specJSON)
	orders := findIRGroupByPathString(t, spec.Routes.Groups, "orders")
	params := orders.Endpoints[0].Request.Headers
	if len(params) != 1 {
		t.Fatalf("len(header params) = %d, want 1", len(params))
	}
	if got := params[0].Type.Kind; got != ir.TypeKindInt {
		t.Fatalf("header param type = %q, want int", got)
	}
}

func TestAppGroup_OmitHeaderRemovesAnyInheritedHeader(t *testing.T) {
	t.Parallel()

	app := newGroupedTestApp(
		"/api/v1",
		meta.Group(
			"/users",
			meta.RequireHeader(customIDHeader),
			meta.Group(
				"/public",
				meta.OmitHeader(customIDHeader),
				meta.Endpoint(&GroupedSpecTestEndpoint{}),
			),
		),
	)

	specJSON, err := app.buildIRJSON(nil)
	if err != nil {
		t.Fatalf("buildIRJSON() error = %v", err)
	}

	spec := decodeIRDocument(t, specJSON)
	public := findIRGroupByPathString(t, spec.Routes.Groups, "users.public")
	if containsHeaderFold(public.Headers, "X-Custom-Id") {
		t.Fatalf("public group headers = %#v, should not contain X-Custom-Id", public.Headers)
	}
}

func TestAppGroup_OmitHeaderPanicsWithoutInheritedAuth(t *testing.T) {
	t.Parallel()

	defer func() {
		msg, ok := panicMessage(recover())
		if !ok {
			t.Fatal("expected panic, got nil")
		}
		if !strings.Contains(msg, "not required by any parent group") {
			t.Fatalf("panic = %q, want not required message", msg)
		}
	}()

	newGroupedTestApp(
		"/api/v1",
		meta.Group(
			"/public",
			meta.OmitHeader(authorizationHeader),
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

	repo := &staticDependencyRepo{}
	logger := &dependencyLogger{}
	app := newGroupedTestApp(
		"/api",
		meta.NewProvideBinding(repo),
		meta.NewProvideBinding[dependencyUserRepo](repo),
		meta.NewProvideBinding(logger),
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

	repo := &namedDependencyRepo{name: "Parent"}
	logger := &dependencyLogger{}

	app := newGroupedTestApp(
		"/api",
		meta.NewProvideBinding[dependencyUserRepo](repo),
		meta.NewProvideBinding(logger),
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

	parentRepo := &namedDependencyRepo{name: "Parent"}
	childRepo := &namedDependencyRepo{name: "Child"}
	logger := &dependencyLogger{}

	app := newGroupedTestApp(
		"/api",
		meta.NewProvideBinding[dependencyUserRepo](parentRepo),
		meta.NewProvideBinding(logger),
		meta.Group(
			"/child",
			meta.NewProvideBinding[dependencyUserRepo](childRepo),
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

func TestAppHandler_EndpointProvideShadowsGroup(t *testing.T) {
	t.Parallel()

	parentRepo := &namedDependencyRepo{name: "Parent"}
	endpointRepo := &namedDependencyRepo{name: "Endpoint"}
	logger := &dependencyLogger{}

	app := newGroupedTestApp(
		"/api",
		meta.NewProvideBinding[dependencyUserRepo](parentRepo),
		meta.NewProvideBinding(logger),
		meta.Endpoint(
			&dependencyEndpoint{},
			meta.EndpointProvide(meta.NewProvideBinding[dependencyUserRepo](endpointRepo)),
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
			meta.NewProvideBinding(repo),
			meta.NewProvideBinding[dependencyUserRepo](repo),
			meta.NewProvideBinding(logger),
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

	app := New(meta.GroupNode{
		Path:     "/",
		Children: []meta.Node{meta.Endpoints(endpoints...)},
	})
	app.finalizeDefinitions()
	return app
}

func newTestAppWithNodes(t *testing.T, children ...meta.Node) *App {
	t.Helper()

	app := New(meta.GroupNode{
		Path:     "/",
		Children: children,
	})
	app.finalizeDefinitions()
	return app
}

func newGroupedTestApp(path string, children ...meta.Node) *App {
	app := New(meta.GroupNode{
		Path:     path,
		Children: children,
	})
	app.finalizeDefinitions()
	return app
}

func createOrderEndpointNode(opts ...meta.EndpointOption) meta.Node {
	base := []meta.EndpointOption{
		meta.RequireHeader(idempotencyHeader),
		meta.RequireHeader(requestIDHeader),
	}
	base = append(base, opts...)
	return meta.Endpoint(&CreateOrderSpecTestEndpoint{}, base...)
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

func decodeIRDocument(t *testing.T, specJSON []byte) ir.Document {
	t.Helper()

	var spec ir.Document
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return spec
}

func findIRGroupByPathString(t *testing.T, groups []ir.Group, path string) ir.Group {
	t.Helper()

	if found := findIRGroupByPath(groups, strings.Split(path, ".")); found != nil {
		return *found
	}

	t.Fatalf("group %q not found", path)
	return ir.Group{}
}

func findIRGroupByPath(groups []ir.Group, path []string) *ir.Group {
	if len(path) == 0 {
		return nil
	}
	for _, group := range groups {
		if group.Name != path[0] {
			continue
		}
		if len(path) == 1 {
			return &group
		}
		return findIRGroupByPath(group.Groups, path[1:])
	}
	return nil
}

func irDocumentHasModel(spec ir.Document, name string) bool {
	for _, modelDef := range spec.Models {
		if modelDef.Name == name {
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

func containsHeaderFold(values []ir.Header, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value.Key, target) {
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
