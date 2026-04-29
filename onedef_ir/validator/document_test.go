package validator

import (
	"encoding/json"
	"os"
	"path/filepath"
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
		"duplicate-type.json":        ErrorCodeDuplicateType,
		"duplicate-header.json":      ErrorCodeDuplicateHeader,
		"unknown-type-ref.json":      ErrorCodeUnknownTypeRef,
		"bad-path-param.json":        ErrorCodePathParamMismatch,
		"bad-204-response-body.json": ErrorCodeInvalidSuccessResponse,
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
							"pathParams": [
								{
									"name": "ID",
									"wireName": "id",
									"type": { "kind": "uuid" },
									"required": true
								}
							]
						},
						"response": { "envelope": false }
					}
				]
			}
		],
		"types": []
	}`), &doc); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	Normalize(&doc)
	if err := Validate(&doc); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	group := doc.Groups[0]
	if group.ID != "users" {
		t.Fatalf("group.ID = %q, want users", group.ID)
	}
	if len(group.PathSegments) != 1 || group.PathSegments[0] != "users" {
		t.Fatalf("group.PathSegments = %#v, want users", group.PathSegments)
	}
	if got := group.Endpoints[0].Error.Body.Name; got != BuiltinDefaultError {
		t.Fatalf("error body = %q, want %q", got, BuiltinDefaultError)
	}
}

func TestValidate_RejectsUnknownTypeKind(t *testing.T) {
	t.Parallel()

	var doc Document
	if err := json.Unmarshal([]byte(`{
		"version": "v1",
		"endpoints": [
			{
				"name": "GetClock",
				"method": "GET",
				"path": "/clock",
				"successStatus": 200,
				"request": {},
				"response": {
					"envelope": true,
					"body": { "kind": "datetime" }
				}
			}
		],
		"types": []
	}`), &doc); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
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

func readIRFixture(t *testing.T, kind, name string) []byte {
	t.Helper()

	path := filepath.Join("..", "fixtures", kind, name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	return data
}
