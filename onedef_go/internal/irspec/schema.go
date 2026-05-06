package irspec

import (
	"sort"
	"strings"
)

const VersionV1 = "v1"

const BuiltinDefaultError = "DefaultError"

type Document struct {
	Version     string     `json:"version"`
	Initialisms []string   `json:"initialisms,omitempty"`
	Routes      Routes     `json:"routes"`
	Models      []ModelDef `json:"models"`
}

type Routes struct {
	Headers   []Header   `json:"headers,omitempty"`
	Endpoints []Endpoint `json:"endpoints,omitempty"`
	Groups    []Group    `json:"groups,omitempty"`
}

type Group struct {
	Name      string     `json:"name"`
	Headers   []Header   `json:"headers,omitempty"`
	Endpoints []Endpoint `json:"endpoints,omitempty"`
	Groups    []Group    `json:"groups,omitempty"`
}

type Endpoint struct {
	Name          string     `json:"name"`
	SDKName       string     `json:"sdkName,omitempty"`
	Method        HTTPMethod `json:"method"`
	Path          string     `json:"path"`
	SuccessStatus int        `json:"successStatus"`
	Request       Request    `json:"request"`
	Response      Response   `json:"response"`
	Error         Error      `json:"error"`
}

type Request struct {
	Paths   []Parameter       `json:"paths,omitempty"`
	Queries []Parameter       `json:"queries,omitempty"`
	Headers []HeaderParameter `json:"headers,omitempty"`
	Body    *TypeRef          `json:"body,omitempty"`
}

type Response struct {
	Envelope bool     `json:"envelope"`
	Body     *TypeRef `json:"body,omitempty"`
}

type Error struct {
	Body TypeRef `json:"body"`
}

type Parameter struct {
	Name        string   `json:"name"`
	Key         string   `json:"key"`
	Type        TypeRef  `json:"type"`
	Description string   `json:"description,omitempty"`
	Examples    []string `json:"examples,omitempty"`
}

type HeaderParameter struct {
	Name        string   `json:"name"`
	Key         string   `json:"key"`
	Type        TypeRef  `json:"type"`
	Required    bool     `json:"required"`
	Description string   `json:"description,omitempty"`
	Examples    []string `json:"examples,omitempty"`
}

type Header struct {
	Key         string   `json:"key"`
	Type        TypeRef  `json:"type"`
	Description string   `json:"description,omitempty"`
	Examples    []string `json:"examples,omitempty"`
}

type ModelDef struct {
	Name   string     `json:"name"`
	Kind   ModelKind  `json:"kind"`
	Fields []FieldDef `json:"fields,omitempty"`
}

type FieldDef struct {
	Name      string  `json:"name"`
	Key       string  `json:"key"`
	Type      TypeRef `json:"type"`
	Required  bool    `json:"required"`
	OmitEmpty bool    `json:"omitEmpty,omitempty"`
}

type TypeRef struct {
	Kind     TypeKind `json:"kind"`
	Name     string   `json:"name,omitempty"`
	Nullable bool     `json:"nullable,omitempty"`
	Elem     *TypeRef `json:"elem,omitempty"`
	Key      *TypeRef `json:"key,omitempty"`
	Value    *TypeRef `json:"value,omitempty"`
}

type HTTPMethod string

const (
	HTTPMethodGET     HTTPMethod = "GET"
	HTTPMethodPOST    HTTPMethod = "POST"
	HTTPMethodPUT     HTTPMethod = "PUT"
	HTTPMethodPATCH   HTTPMethod = "PATCH"
	HTTPMethodDELETE  HTTPMethod = "DELETE"
	HTTPMethodHEAD    HTTPMethod = "HEAD"
	HTTPMethodOPTIONS HTTPMethod = "OPTIONS"
)

type ModelKind string

const (
	ModelKindObject ModelKind = "object"
)

type TypeKind string

const (
	TypeKindAny    TypeKind = "any"
	TypeKindBool   TypeKind = "bool"
	TypeKindFloat  TypeKind = "float"
	TypeKindInt    TypeKind = "int"
	TypeKindList   TypeKind = "list"
	TypeKindMap    TypeKind = "map"
	TypeKindNamed  TypeKind = "named"
	TypeKindString TypeKind = "string"
	TypeKindUUID   TypeKind = "uuid"
)

func Normalize(doc *Document) {
	if doc == nil {
		return
	}
	doc.Initialisms = NormalizeInitialisms(doc.Initialisms)
	if doc.Models == nil {
		doc.Models = []ModelDef{}
	}
	normalizeRoutes(&doc.Routes)
	for i := range doc.Models {
		if doc.Models[i].Fields == nil {
			doc.Models[i].Fields = []FieldDef{}
		}
	}
}

func NormalizeInitialisms(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		key := strings.ToUpper(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool {
		if len(result[i]) != len(result[j]) {
			return len(result[i]) > len(result[j])
		}
		return result[i] < result[j]
	})
	return result
}

func normalizeRoutes(routes *Routes) {
	if routes.Headers == nil {
		routes.Headers = []Header{}
	}
	if routes.Endpoints == nil {
		routes.Endpoints = []Endpoint{}
	}
	if routes.Groups == nil {
		routes.Groups = []Group{}
	}
	normalizeEndpoints(routes.Endpoints)
	normalizeGroups(routes.Groups)
}

func normalizeGroups(groups []Group) {
	for i := range groups {
		if groups[i].Headers == nil {
			groups[i].Headers = []Header{}
		}
		if groups[i].Endpoints == nil {
			groups[i].Endpoints = []Endpoint{}
		}
		if groups[i].Groups == nil {
			groups[i].Groups = []Group{}
		}
		normalizeEndpoints(groups[i].Endpoints)
		normalizeGroups(groups[i].Groups)
	}
}

func normalizeEndpoints(endpoints []Endpoint) {
	for i := range endpoints {
		if endpoints[i].Request.Paths == nil {
			endpoints[i].Request.Paths = []Parameter{}
		}
		if endpoints[i].Request.Queries == nil {
			endpoints[i].Request.Queries = []Parameter{}
		}
		if endpoints[i].Request.Headers == nil {
			endpoints[i].Request.Headers = []HeaderParameter{}
		}
		if endpoints[i].Error.Body.Kind == "" {
			endpoints[i].Error = Error{
				Body: TypeRef{Kind: TypeKindNamed, Name: BuiltinDefaultError},
			}
		}
	}
}
