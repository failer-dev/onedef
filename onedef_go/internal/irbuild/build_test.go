package irbuild

import (
	"reflect"
	"strings"
	"testing"

	"github.com/failer-dev/onedef/onedef_go/internal/inspect"
	ir "github.com/failer-dev/onedef/onedef_go/internal/irspec"
	"github.com/failer-dev/onedef/onedef_go/internal/meta"
	"github.com/google/uuid"
)

type sharedUser struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Aliases []string  `json:"aliases,omitempty"`
	Profile *struct {
		Bio string `json:"bio"`
	} `json:"profile,omitempty"`
}

type createUserBody struct {
	Name string `json:"name"`
}

type createUser struct {
	meta.POST `path:"/users" status:"201"`
	Request   createUserBody
	Response  sharedUser
}

type deleteUser struct {
	meta.DELETE `path:"/users/{id}" status:"204"`
	Request     struct {
		ID string
	}
	Response struct{}
}

type loginAuth struct {
	meta.POST `path:"/auth/login"`
	Request   struct {
		Email string `json:"email"`
	}
	Response struct {
		Token string `json:"token"`
	}
}

type searchUsers struct {
	meta.GET `path:"/users"`
	Request  struct {
		Query *string `json:"query,omitempty"`
		Page  int     `json:"page"`
	}
	Response []sharedUser
}

type updateUser struct {
	meta.PUT `path:"/users/{id}"`
	Request  struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	Response sharedUser
}

type invalidMapKey struct {
	meta.POST `path:"/broken"`
	Request   struct {
		Lookup map[int]string `json:"lookup"`
	}
	Response struct{}
}

type groupedEndpoint struct {
	meta.GET `path:"/{id}"`
	Request  struct{ ID string }
	Response sharedUser
}

type groupedError struct {
	Source string `json:"source"`
}

func TestBuildDocument_BuildsFlatEndpoints(t *testing.T) {
	t.Parallel()

	doc, err := BuildDocument(Options{
		Initialisms: []string{"ID", "OAuth", "id", "  JWT  "},
		Endpoints: []meta.EndpointStruct{
			endpointStruct(t, &createUser{}),
			endpointStruct(t, &deleteUser{}),
			endpointStruct(t, &loginAuth{}),
			endpointStruct(t, &searchUsers{}),
			endpointStruct(t, &updateUser{}),
		},
	})
	if err != nil {
		t.Fatalf("BuildDocument() error = %v", err)
	}

	if got := strings.Join(doc.Naming.Initialisms, ","); got != "OAuth,JWT,ID" {
		t.Fatalf("Initialisms = %q, want OAuth,JWT,ID", got)
	}
	if len(doc.Endpoints) != 5 {
		t.Fatalf("len(Endpoints) = %d, want 5", len(doc.Endpoints))
	}

	create := findEndpoint(t, doc, "createUser")
	if create.Group != "user" {
		t.Fatalf("createUser group = %q, want user", create.Group)
	}
	if create.Request.Body == nil || create.Request.Body.Name != "createUserBody" {
		t.Fatalf("createUser body = %#v, want named createUserBody", create.Request.Body)
	}
	if !create.Response.Envelope || create.Response.Body == nil || create.Response.Body.Name != "sharedUser" {
		t.Fatalf("createUser response = %#v, want sharedUser envelope", create.Response)
	}

	login := findEndpoint(t, doc, "loginAuth")
	if login.Group != "auth" {
		t.Fatalf("loginAuth group = %q, want auth", login.Group)
	}
	if login.Request.Body == nil || login.Request.Body.Name != "loginAuthRequest" {
		t.Fatalf("loginAuth body = %#v, want loginAuthRequest", login.Request.Body)
	}
	if login.Response.Body == nil || login.Response.Body.Name != "loginAuthResponse" {
		t.Fatalf("loginAuth response = %#v, want loginAuthResponse", login.Response.Body)
	}

	search := findEndpoint(t, doc, "searchUsers")
	if len(search.Request.QueryParams) != 2 {
		t.Fatalf("len(searchUsers query params) = %d, want 2", len(search.Request.QueryParams))
	}
	if search.Request.QueryParams[0].Required || search.Request.QueryParams[1].Required {
		t.Fatalf("query params should be optional: %#v", search.Request.QueryParams)
	}
	if search.Response.Body == nil || search.Response.Body.Kind != ir.TypeKindList || search.Response.Body.Elem == nil || search.Response.Body.Elem.Name != "sharedUser" {
		t.Fatalf("searchUsers response = %#v, want list of sharedUser", search.Response.Body)
	}

	deleteUser := findEndpoint(t, doc, "deleteUser")
	if deleteUser.Group != "user" {
		t.Fatalf("deleteUser group = %q, want user", deleteUser.Group)
	}
	if deleteUser.Response.Envelope || deleteUser.Response.Body != nil {
		t.Fatalf("deleteUser response = %#v, want no envelope/body", deleteUser.Response)
	}

	update := findEndpoint(t, doc, "updateUser")
	if update.Request.Body == nil || update.Request.Body.Name != "updateUserRequest" {
		t.Fatalf("updateUser body = %#v, want synthetic updateUserRequest", update.Request.Body)
	}
	bodyType := findType(t, doc, "updateUserRequest")
	if len(bodyType.Fields) != 1 || bodyType.Fields[0].WireName != "name" {
		t.Fatalf("updateUserRequest fields = %#v, want only name field", bodyType.Fields)
	}

	userType := findType(t, doc, "sharedUser")
	if len(userType.Fields) != 4 {
		t.Fatalf("sharedUser fields = %#v, want 4 fields", userType.Fields)
	}
	profileField := findField(t, userType, "Profile")
	if !profileField.Nullable || profileField.Type.Name != "sharedUserProfile" {
		t.Fatalf("Profile field = %#v, want nullable sharedUserProfile", profileField)
	}
}

func TestBuildDocument_BuildsGroupsHeadersAndCustomError(t *testing.T) {
	t.Parallel()

	endpoint := endpointStruct(t, &groupedEndpoint{})
	endpoint.Path = "/api/v1/users/{id}"
	endpoint.GroupPath = []string{"users"}
	authorization := stringHeader("Authorization")
	endpoint.InheritedRequiredHeaders = []meta.HeaderContract{authorization}
	endpoint.FinalRequiredHeaders = []meta.HeaderContract{authorization}
	endpoint.ErrorBodyType = reflect.TypeOf(groupedError{})

	doc, err := BuildDocument(Options{
		Groups: []*meta.GroupMeta{
			{
				ID:                      "users",
				Name:                    "users",
				PathSegments:            []string{"users"},
				ProviderRequiredHeaders: []meta.HeaderContract{authorization},
				FinalRequiredHeaders:    []meta.HeaderContract{authorization},
				Endpoints:               []meta.EndpointStruct{endpoint},
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildDocument() error = %v", err)
	}

	if len(doc.Groups) != 1 {
		t.Fatalf("len(Groups) = %d, want 1", len(doc.Groups))
	}
	group := doc.Groups[0]
	if got := strings.Join(group.RequiredHeaders, ","); got != "Authorization" {
		t.Fatalf("required headers = %q, want Authorization", got)
	}
	if len(group.ProviderHeaders) != 1 || group.ProviderHeaders[0].Type.Kind != ir.TypeKindString {
		t.Fatalf("provider headers = %#v, want string Authorization", group.ProviderHeaders)
	}
	if got := group.Endpoints[0].Path; got != "/api/v1/users/{id}" {
		t.Fatalf("endpoint path = %q, want grouped full path", got)
	}
	if got := group.Endpoints[0].Error.Body.Name; got != "groupedError" {
		t.Fatalf("error body = %q, want groupedError", got)
	}
	if !hasType(doc, "groupedError") {
		t.Fatalf("types = %#v, want groupedError", doc.Types)
	}
}

func TestBuildDocument_ErrorsForUnsupportedMapKey(t *testing.T) {
	t.Parallel()

	_, err := BuildDocument(Options{
		Endpoints: []meta.EndpointStruct{endpointStruct(t, &invalidMapKey{})},
	})
	if err == nil {
		t.Fatal("BuildDocument() error = nil, want error")
	}
}

func endpointStruct(t *testing.T, endpoint any) meta.EndpointStruct {
	t.Helper()

	structType := reflect.TypeOf(endpoint)
	if structType.Kind() == reflect.Pointer {
		structType = structType.Elem()
	}
	method, path, pathParams, successStatus, err := inspect.InspectEndpointMethodMarker(structType)
	if err != nil {
		t.Fatalf("InspectEndpointMethodMarker() error = %v", err)
	}
	request, err := inspect.InspectRequest(structType, method, pathParams)
	if err != nil {
		t.Fatalf("InspectRequest() error = %v", err)
	}
	for i := range request.HeaderParameterFields {
		request.HeaderParameterFields[i].MethodParameter = true
	}
	es := meta.EndpointStruct{
		StructName:          structType.Name(),
		StructPkgPath:       structType.PkgPath(),
		StructQualifiedName: qualifiedStructName(structType),
		Method:              method,
		Path:                path,
		LeafPath:            path,
		SuccessStatus:       successStatus,
		Request:             request,
		StructType:          structType,
		ErrorBodyType:       reflect.TypeOf(meta.DefaultError{}),
	}
	for _, h := range request.HeaderParameterFields {
		es.EndpointRequiredHeaders = append(es.EndpointRequiredHeaders, h.Header)
		if h.Required {
			es.FinalRequiredHeaders = append(es.FinalRequiredHeaders, h.Header)
		}
	}
	return es
}

func stringHeader(name string) meta.HeaderContract {
	return meta.MustHeaderContract(meta.NewHeader[string](name))
}

func qualifiedStructName(structType reflect.Type) string {
	if structType.PkgPath() == "" {
		return structType.Name()
	}
	return structType.PkgPath() + "." + structType.Name()
}

func findEndpoint(t *testing.T, doc *ir.Document, name string) ir.Endpoint {
	t.Helper()

	for _, endpoint := range doc.Endpoints {
		if endpoint.Name == name {
			return endpoint
		}
	}

	t.Fatalf("endpoint %q not found", name)
	return ir.Endpoint{}
}

func findType(t *testing.T, doc *ir.Document, name string) ir.TypeDef {
	t.Helper()

	for _, typeDef := range doc.Types {
		if typeDef.Name == name {
			return typeDef
		}
	}

	t.Fatalf("type %q not found", name)
	return ir.TypeDef{}
}

func findField(t *testing.T, typeDef ir.TypeDef, name string) ir.FieldDef {
	t.Helper()

	for _, field := range typeDef.Fields {
		if field.Name == name {
			return field
		}
	}

	t.Fatalf("field %q not found in %#v", name, typeDef)
	return ir.FieldDef{}
}

func hasType(doc *ir.Document, name string) bool {
	for _, typeDef := range doc.Types {
		if typeDef.Name == name {
			return true
		}
	}
	return false
}
