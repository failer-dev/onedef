package ir

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/failer-dev/onedef"
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

type CreateUser struct {
	onedef.POST `path:"/users" status:"201"`
	Request     createUserBody
	Response    sharedUser
}

func (*CreateUser) Handle(context.Context) error { return nil }

type DeleteUser struct {
	onedef.DELETE `path:"/users/{id}" status:"204"`
	Request       struct {
		ID string
	}
	Response struct{}
}

func (*DeleteUser) Handle(context.Context) error { return nil }

type LoginAuth struct {
	onedef.POST `path:"/auth/login"`
	Request     struct {
		Email string `json:"email"`
	}
	Response struct {
		Token string `json:"token"`
	}
}

func (*LoginAuth) Handle(context.Context) error { return nil }

type SearchUsers struct {
	onedef.GET `path:"/users"`
	Request    struct {
		Query *string `json:"query,omitempty"`
		Page  int     `json:"page"`
	}
	Response []sharedUser
}

func (*SearchUsers) Handle(context.Context) error { return nil }

type UpdateUser struct {
	onedef.PUT `path:"/users/{id}"`
	Request    struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	Response sharedUser
}

func (*UpdateUser) Handle(context.Context) error { return nil }

type InvalidMapKey struct {
	onedef.POST `path:"/broken"`
	Request     struct {
		Lookup map[int]string `json:"lookup"`
	}
	Response struct{}
}

func (*InvalidMapKey) Handle(context.Context) error { return nil }

type ConflictingUserA struct {
	onedef.GET `path:"/conflict-a"`
	Request    struct{}
	Response   struct {
		Name string `json:"name"`
	}
}

func (*ConflictingUserA) Handle(context.Context) error { return nil }

type ConflictingUserB struct {
	onedef.GET `path:"/conflict-b"`
	Request    struct{}
	Response   struct {
		Age int `json:"age"`
	}
}

func (*ConflictingUserB) Handle(context.Context) error { return nil }

func TestParse_BuildsSpecFromEndpoints(t *testing.T) {
	t.Parallel()

	spec, err := Parse(&CreateUser{}, &DeleteUser{}, &LoginAuth{}, &SearchUsers{}, &UpdateUser{})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if spec.Version != "v1" {
		t.Fatalf("Version = %q, want v1", spec.Version)
	}
	if len(spec.Endpoints) != 5 {
		t.Fatalf("len(Endpoints) = %d, want 5", len(spec.Endpoints))
	}

	create := findEndpoint(t, spec, "CreateUser")
	if create.Group != "user" {
		t.Fatalf("CreateUser group = %q, want user", create.Group)
	}
	if create.Request.Body == nil || create.Request.Body.Name != "createUserBody" {
		t.Fatalf("CreateUser body = %#v, want named createUserBody", create.Request.Body)
	}
	if !create.Response.Envelope || create.Response.Body == nil || create.Response.Body.Name != "sharedUser" {
		t.Fatalf("CreateUser response = %#v, want sharedUser envelope", create.Response)
	}

	login := findEndpoint(t, spec, "LoginAuth")
	if login.Group != "auth" {
		t.Fatalf("LoginAuth group = %q, want auth", login.Group)
	}
	if login.Request.Body == nil || login.Request.Body.Name != "LoginAuthRequest" {
		t.Fatalf("LoginAuth body = %#v, want LoginAuthRequest", login.Request.Body)
	}
	if login.Response.Body == nil || login.Response.Body.Name != "LoginAuthResponse" {
		t.Fatalf("LoginAuth response = %#v, want LoginAuthResponse", login.Response.Body)
	}

	search := findEndpoint(t, spec, "SearchUsers")
	if len(search.Request.QueryParams) != 2 {
		t.Fatalf("len(SearchUsers query params) = %d, want 2", len(search.Request.QueryParams))
	}
	if search.Request.QueryParams[0].Required || search.Request.QueryParams[1].Required {
		t.Fatalf("query params should be optional: %#v", search.Request.QueryParams)
	}
	if search.Response.Body == nil || search.Response.Body.Kind != TypeKindList || search.Response.Body.Elem == nil || search.Response.Body.Elem.Name != "sharedUser" {
		t.Fatalf("SearchUsers response = %#v, want list of sharedUser", search.Response.Body)
	}

	deleteUser := findEndpoint(t, spec, "DeleteUser")
	if deleteUser.Group != "user" {
		t.Fatalf("DeleteUser group = %q, want user", deleteUser.Group)
	}
	if deleteUser.Response.Envelope {
		t.Fatalf("DeleteUser envelope = true, want false")
	}
	if deleteUser.Response.Body != nil {
		t.Fatalf("DeleteUser body = %#v, want nil", deleteUser.Response.Body)
	}

	update := findEndpoint(t, spec, "UpdateUser")
	if update.Request.Body == nil || update.Request.Body.Name != "UpdateUserRequest" {
		t.Fatalf("UpdateUser body = %#v, want synthetic UpdateUserRequest", update.Request.Body)
	}
	bodyType := findType(t, spec, "UpdateUserRequest")
	if len(bodyType.Fields) != 1 || bodyType.Fields[0].WireName != "name" {
		t.Fatalf("UpdateUserRequest fields = %#v, want only name field", bodyType.Fields)
	}

	userType := findType(t, spec, "sharedUser")
	if len(userType.Fields) != 4 {
		t.Fatalf("sharedUser fields = %#v, want 4 fields", userType.Fields)
	}
	profileField := findField(t, userType, "Profile")
	if !profileField.Nullable {
		t.Fatalf("Profile field = %#v, want nullable", profileField)
	}
	if profileField.Type.Name != "sharedUserProfile" {
		t.Fatalf("Profile type = %#v, want sharedUserProfile", profileField.Type)
	}
	aliasesField := findField(t, userType, "Aliases")
	if aliasesField.Required {
		t.Fatalf("Aliases field required = true, want false")
	}
	if aliasesField.Type.Kind != TypeKindList || aliasesField.Type.Elem == nil || aliasesField.Type.Elem.Kind != TypeKindString {
		t.Fatalf("Aliases type = %#v, want list<string>", aliasesField.Type)
	}
	idField := findField(t, userType, "ID")
	if idField.Type.Kind != TypeKindUUID {
		t.Fatalf("ID type = %#v, want uuid", idField.Type)
	}
}

func TestParseWithOptions_IncludesNamingInitialisms(t *testing.T) {
	t.Parallel()

	spec, err := ParseWithOptions(
		ParseOptions{Initialisms: []string{"ID", "OAuth", "id", "  JWT  "}},
		&CreateUser{},
	)
	if err != nil {
		t.Fatalf("ParseWithOptions() error = %v", err)
	}
	if spec.Naming == nil {
		t.Fatal("Naming = nil, want initialisms")
	}
	got := strings.Join(spec.Naming.Initialisms, ",")
	if got != "OAuth,JWT,ID" {
		t.Fatalf("Initialisms = %q, want OAuth,JWT,ID", got)
	}
}

func TestParse_ErrorsForUnsupportedMapKey(t *testing.T) {
	t.Parallel()

	_, err := Parse(&InvalidMapKey{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMustParse_PanicsOnError(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic, got nil")
		}
	}()

	MustParse(&InvalidMapKey{})
}

func findEndpoint(t *testing.T, spec *Spec, name string) Endpoint {
	t.Helper()

	for _, endpoint := range spec.Endpoints {
		if endpoint.Name == name {
			return endpoint
		}
	}

	t.Fatalf("endpoint %q not found", name)
	return Endpoint{}
}

func findType(t *testing.T, spec *Spec, name string) TypeDef {
	t.Helper()

	for _, typeDef := range spec.Types {
		if typeDef.Name == name {
			return typeDef
		}
	}

	t.Fatalf("type %q not found", name)
	return TypeDef{}
}

func findField(t *testing.T, typeDef TypeDef, name string) FieldDef {
	t.Helper()

	for _, field := range typeDef.Fields {
		if field.Name == name {
			return field
		}
	}

	t.Fatalf("field %q not found in %q", name, typeDef.Name)
	return FieldDef{}
}

func TestAssignGroups_RootEndpointsStayUngrouped(t *testing.T) {
	t.Parallel()

	endpoints := assignGroups([]Endpoint{
		{Name: "Health", Path: "/health"},
		{Name: "CreateUser", Path: "/users"},
	})

	if endpoints[0].Group != "" {
		t.Fatalf("Health group = %q, want empty", endpoints[0].Group)
	}
	if endpoints[1].Group != "user" {
		t.Fatalf("CreateUser group = %q, want user", endpoints[1].Group)
	}
}

func TestFieldHasOmitEmpty(t *testing.T) {
	t.Parallel()

	field, ok := reflect.TypeOf(struct {
		Name string `json:"name,omitempty"`
	}{}).FieldByName("Name")
	if !ok {
		t.Fatal("field Name not found")
	}

	if !fieldHasOmitEmpty(field) {
		t.Fatal("fieldHasOmitEmpty = false, want true")
	}
}
