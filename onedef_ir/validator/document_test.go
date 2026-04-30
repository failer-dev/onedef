package validator

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeJSON_ValidCanonicalFixtures(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"simple.json",
		"grouped.json",
		"nested-groups.json",
		"custom-error.json",
	} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			data := readIRFixture(t, "valid", name)
			doc, err := DecodeJSON(data)
			if err != nil {
				t.Fatalf("DecodeJSON() error = %v", err)
			}
			if doc.Version != VersionV1 {
				t.Fatalf("Version = %q, want %q", doc.Version, VersionV1)
			}
		})
	}
}

func TestDecodeJSON_InvalidCanonicalFixtures(t *testing.T) {
	t.Parallel()

	tests := map[string]ErrorCode{
		"duplicate-model.json":           ErrorCodeDuplicateModel,
		"duplicate-header.json":          ErrorCodeDuplicateHeader,
		"duplicate-bound-header.json":    ErrorCodeDuplicateHeader,
		"duplicate-group.json":           ErrorCodeDuplicateName,
		"duplicate-endpoint.json":        ErrorCodeDuplicateName,
		"duplicate-field-key.json":       ErrorCodeDuplicateBinding,
		"duplicate-query-key.json":       ErrorCodeDuplicateBinding,
		"duplicate-path-key.json":        ErrorCodeDuplicateBinding,
		"unknown-type-ref.json":          ErrorCodeUnknownTypeRef,
		"bad-path-param.json":            ErrorCodePathParamMismatch,
		"bad-204-response-body.json":     ErrorCodeInvalidSuccessResponse,
		"field-nullable.json":            ErrorCodeInvalidTypeRef,
		"top-level-types.json":           ErrorCodeInvalidTypeRef,
		"top-level-endpoints.json":       ErrorCodeInvalidTypeRef,
		"endpoint-group.json":            ErrorCodeInvalidTypeRef,
		"request-singular-bindings.json": ErrorCodeInvalidTypeRef,
	}

	for name, wantCode := range tests {
		name := name
		wantCode := wantCode
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			data := readIRFixture(t, "invalid", name)
			_, err := DecodeJSON(data)
			if err == nil {
				t.Fatal("DecodeJSON() error = nil, want error")
			}
			if got := ErrorCodeOf(err); got != wantCode {
				t.Fatalf("ErrorCodeOf() = %q, want %q; err = %v", got, wantCode, err)
			}
		})
	}
}

func TestNormalize_DefaultsOmittedCollectionsAndError(t *testing.T) {
	t.Parallel()

	var doc Document
	if err := json.Unmarshal([]byte(`{
		"version": "v1",
		"routes": {
			"groups": [
				{
					"name": "users",
					"endpoints": [
						{
							"name": "DeleteUser",
							"method": "DELETE",
							"path": "/users/{id}",
							"successStatus": 204,
							"request": {
								"paths": [
									{
										"name": "ID",
										"key": "id",
										"type": "uuid"
									}
								]
							},
							"response": { "envelope": false }
						}
					]
				}
			]
		},
		"models": []
	}`), &doc); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	Normalize(&doc)
	if err := Validate(&doc); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	group := doc.Routes.Groups[0]
	if group.Name != "users" {
		t.Fatalf("group.Name = %q, want users", group.Name)
	}
	if got := group.Endpoints[0].Error.Body.Name; got != BuiltinDefaultError {
		t.Fatalf("error body = %q, want %q", got, BuiltinDefaultError)
	}
}

func TestValidate_RejectsUnknownTypeKind(t *testing.T) {
	t.Parallel()

	doc := Document{
		Version: VersionV1,
		Routes: &Routes{
			Endpoints: []Endpoint{
				{
					Name:          "GetClock",
					Method:        "GET",
					Path:          "/clock",
					SuccessStatus: 200,
					Response: Response{
						Envelope: true,
						Body:     &TypeRef{Kind: "datetime"},
					},
				},
			},
		},
		Models: []ModelDef{},
	}

	Normalize(&doc)
	err := Validate(&doc)
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if got := ErrorCodeOf(err); got != ErrorCodeInvalidTypeRef {
		t.Fatalf("ErrorCodeOf() = %q, want %q; err = %v", got, ErrorCodeInvalidTypeRef, err)
	}
}

func TestValidate_RejectsUnknownModelKind(t *testing.T) {
	t.Parallel()

	doc := Document{
		Version: VersionV1,
		Routes:  &Routes{},
		Models: []ModelDef{
			{
				Name: "BookingStatus",
				Kind: ModelKind("enum"),
			},
		},
	}

	Normalize(&doc)
	err := Validate(&doc)
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if got := ErrorCodeOf(err); got != ErrorCodeInvalidTypeRef {
		t.Fatalf("ErrorCodeOf() = %q, want %q; err = %v", got, ErrorCodeInvalidTypeRef, err)
	}
}

func TestTypeRefJSON_UsesReadableTypeExpression(t *testing.T) {
	t.Parallel()

	var got TypeRef
	if err := json.Unmarshal([]byte(`"map<string, list<Booking?>>"`), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got.Kind != TypeKindMap {
		t.Fatalf("Kind = %q, want map", got.Kind)
	}
	if got.Key == nil || got.Key.Kind != TypeKindString {
		t.Fatalf("Key = %#v, want string", got.Key)
	}
	if got.Value == nil || got.Value.Kind != TypeKindList {
		t.Fatalf("Value = %#v, want list", got.Value)
	}
	if got.Value.Elem == nil || got.Value.Elem.Name != "Booking" || !got.Value.Elem.Nullable {
		t.Fatalf("Value.Elem = %#v, want nullable Booking", got.Value.Elem)
	}

	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(got); err != nil {
		t.Fatalf("json.Encode() error = %v", err)
	}
	if strings.TrimSpace(buffer.String()) != `"map<string, list<Booking?>>"` {
		t.Fatalf("json.Encode() = %s, want readable expression", buffer.String())
	}
}

func readIRFixture(t *testing.T, kind, name string) []byte {
	t.Helper()

	path := filepath.Join("..", "fixtures", kind, name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	return data
}
