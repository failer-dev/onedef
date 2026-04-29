package inspect

import (
	"reflect"
	"strings"
	"testing"

	"github.com/failer-dev/onedef/internal/meta"
)

type getDefaultStatus struct {
	meta.GET `path:"/users/{id}"`
	Request  struct{ ID string }
	Response struct{}
}

type postCreatedStatus struct {
	meta.POST `path:"/users" status:"201"`
	Request   struct{ Name string }
	Response  struct{ ID string }
}

type deleteNoContentStatus struct {
	meta.DELETE `path:"/users/{id}" status:"204"`
	Request     struct{ ID string }
	Response    struct{}
}

type invalidStatusNonInteger struct {
	meta.POST `path:"/users" status:"created"`
	Request   struct{}
	Response  struct{}
}

type invalidStatusTooLow struct {
	meta.POST `path:"/users" status:"199"`
	Request   struct{}
	Response  struct{}
}

type invalidStatusTooHigh struct {
	meta.POST `path:"/users" status:"300"`
	Request   struct{}
	Response  struct{}
}

type dependencyContract interface {
	FindUser(string) string
}

type namedDependencySet struct {
	Users dependencyContract
}

type validNamedDeps struct {
	meta.GET `path:"/users/{id}"`
	Request  struct{ ID string }
	Response struct{}
	Deps     namedDependencySet
}

type invalidDepsAny struct {
	meta.GET `path:"/broken"`
	Request  struct{}
	Response struct{}
	Deps     any
}

type invalidDepsPointer struct {
	meta.GET `path:"/broken"`
	Request  struct{}
	Response struct{}
	Deps     *namedDependencySet
}

type invalidDepsUnexported struct {
	meta.GET `path:"/broken"`
	Request  struct{}
	Response struct{}
	Deps     struct {
		users dependencyContract
	}
}

type invalidDepsAnonymous struct {
	meta.GET `path:"/broken"`
	Request  struct{}
	Response struct{}
	Deps     struct {
		dependencyContract
	}
}

func TestInspectEndpointMethodMarker_SuccessStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		endpoint   any
		wantMethod meta.EndpointMethod
		wantStatus int
	}{
		{name: "default", endpoint: getDefaultStatus{}, wantMethod: meta.EndpointMethodGet, wantStatus: 200},
		{name: "created", endpoint: postCreatedStatus{}, wantMethod: meta.EndpointMethodPost, wantStatus: 201},
		{name: "no content", endpoint: deleteNoContentStatus{}, wantMethod: meta.EndpointMethodDelete, wantStatus: 204},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			method, _, _, status, err := InspectEndpointMethodMarker(reflect.TypeOf(tc.endpoint))
			if err != nil {
				t.Fatalf("InspectEndpointMethodMarker() error = %v", err)
			}
			if method != tc.wantMethod {
				t.Fatalf("method = %q, want %q", method, tc.wantMethod)
			}
			if status != tc.wantStatus {
				t.Fatalf("status = %d, want %d", status, tc.wantStatus)
			}
		})
	}
}

func TestInspectEndpointMethodMarker_InvalidStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		endpoint any
		wantErr  string
	}{
		{name: "non-integer", endpoint: invalidStatusNonInteger{}, wantErr: `invalid ` + "`status`" + ` tag "created"`},
		{name: "too low", endpoint: invalidStatusTooLow{}, wantErr: `status must be between 200 and 299`},
		{name: "too high", endpoint: invalidStatusTooHigh{}, wantErr: `status must be between 200 and 299`},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, _, _, _, err := InspectEndpointMethodMarker(reflect.TypeOf(tc.endpoint))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestInspectDependencies(t *testing.T) {
	t.Parallel()

	t.Run("missing deps", func(t *testing.T) {
		t.Parallel()

		got, err := InspectDependencies(reflect.TypeOf(getDefaultStatus{}))
		if err != nil {
			t.Fatalf("InspectDependencies() error = %v", err)
		}
		if got.Exists {
			t.Fatalf("Exists = %v, want false", got.Exists)
		}
	})

	t.Run("named struct deps", func(t *testing.T) {
		t.Parallel()

		got, err := InspectDependencies(reflect.TypeOf(validNamedDeps{}))
		if err != nil {
			t.Fatalf("InspectDependencies() error = %v", err)
		}
		if !got.Exists {
			t.Fatal("Exists = false, want true")
		}
		if got.StructIndex != 3 {
			t.Fatalf("StructIndex = %d, want %d", got.StructIndex, 3)
		}
		if len(got.Fields) != 1 {
			t.Fatalf("len(Fields) = %d, want 1", len(got.Fields))
		}
		if got.Fields[0].FieldName != "Users" {
			t.Fatalf("FieldName = %q, want %q", got.Fields[0].FieldName, "Users")
		}
		if got.Fields[0].FieldType != reflect.TypeFor[dependencyContract]() {
			t.Fatalf("FieldType = %v, want %v", got.Fields[0].FieldType, reflect.TypeFor[dependencyContract]())
		}
	})
}

type headerTaggedRequest struct {
	meta.POST `path:"/orders"`
	Request   struct {
		IdempotencyKey string `header:"Idempotency-Key"`
		Name           string `json:"name"`
	}
	Response struct{ ID string }
}

type unexportedHeaderField struct {
	meta.POST `path:"/orders"`
	Request   struct {
		idempotencyKey string `header:"Idempotency-Key"`
	}
	Response struct{}
}

type emptyHeaderTag struct {
	meta.POST `path:"/orders"`
	Request   struct {
		Key string `header:""`
	}
	Response struct{}
}

func TestInspectRequest_HeaderFields(t *testing.T) {
	t.Parallel()

	t.Run("parses header fields", func(t *testing.T) {
		t.Parallel()

		result, err := InspectRequest(reflect.TypeOf(headerTaggedRequest{}), meta.EndpointMethodPost, nil)
		if err != nil {
			t.Fatalf("InspectRequest() error = %v", err)
		}
		if len(result.HeaderParameterFields) != 1 {
			t.Fatalf("len(HeaderParameterFields) = %d, want 1", len(result.HeaderParameterFields))
		}
		h := result.HeaderParameterFields[0]
		if h.FieldName != "IdempotencyKey" {
			t.Fatalf("FieldName = %q, want IdempotencyKey", h.FieldName)
		}
		if h.HeaderName != "Idempotency-Key" {
			t.Fatalf("HeaderName = %q, want Idempotency-Key", h.HeaderName)
		}
		if !h.Required {
			t.Fatal("Required = false, want true")
		}
		if len(result.QueryParameterFields) != 0 {
			t.Fatalf("len(QueryParameterFields) = %d, want 0", len(result.QueryParameterFields))
		}
	})

	t.Run("pointer header field is optional", func(t *testing.T) {
		t.Parallel()

		type optionalHeaderRequest struct {
			meta.POST `path:"/orders"`
			Request   struct {
				RequestID *string `header:"X-Request-Id"`
			}
			Response struct{}
		}

		result, err := InspectRequest(reflect.TypeOf(optionalHeaderRequest{}), meta.EndpointMethodPost, nil)
		if err != nil {
			t.Fatalf("InspectRequest() error = %v", err)
		}
		if len(result.HeaderParameterFields) != 1 {
			t.Fatalf("len(HeaderParameterFields) = %d, want 1", len(result.HeaderParameterFields))
		}
		if result.HeaderParameterFields[0].Required {
			t.Fatal("Required = true, want false")
		}
	})

	t.Run("rejects unexported header field", func(t *testing.T) {
		t.Parallel()

		_, err := InspectRequest(reflect.TypeOf(unexportedHeaderField{}), meta.EndpointMethodPost, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "must be exported") {
			t.Fatalf("error = %q, want exported message", err.Error())
		}
	})

	t.Run("rejects empty header tag", func(t *testing.T) {
		t.Parallel()

		_, err := InspectRequest(reflect.TypeOf(emptyHeaderTag{}), meta.EndpointMethodPost, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "must not be empty") {
			t.Fatalf("error = %q, want empty tag message", err.Error())
		}
	})
}

func TestInspectDependencies_InvalidDeps(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		endpoint any
		wantErr  string
	}{
		{name: "any", endpoint: invalidDepsAny{}, wantErr: "Deps must be a struct field"},
		{name: "pointer", endpoint: invalidDepsPointer{}, wantErr: "Deps must be a struct field"},
		{name: "unexported", endpoint: invalidDepsUnexported{}, wantErr: "Deps.users must be exported"},
		{name: "anonymous", endpoint: invalidDepsAnonymous{}, wantErr: "Deps must not contain anonymous fields"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := InspectDependencies(reflect.TypeOf(tc.endpoint))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}
