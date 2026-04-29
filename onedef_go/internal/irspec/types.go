package irspec

import (
	"sort"
	"strings"
)

const VersionV1 = "v1"

const BuiltinDefaultError = "DefaultError"

type Document struct {
	Version   string     `json:"version"`
	Naming    *Naming    `json:"naming,omitempty"`
	Groups    []Group    `json:"groups,omitempty"`
	Endpoints []Endpoint `json:"endpoints,omitempty"`
	Types     []TypeDef  `json:"types"`
}

type Naming struct {
	Initialisms []string `json:"initialisms,omitempty"`
}

type Group struct {
	ID              string      `json:"id"`
	Name            string      `json:"name"`
	PathSegments    []string    `json:"pathSegments,omitempty"`
	RequiredHeaders []string    `json:"requiredHeaders,omitempty"`
	ProviderHeaders []Parameter `json:"providerHeaders,omitempty"`
	Endpoints       []Endpoint  `json:"endpoints,omitempty"`
	Groups          []Group     `json:"groups,omitempty"`
}

type Endpoint struct {
	Name            string   `json:"name"`
	SDKName         string   `json:"sdkName,omitempty"`
	Method          string   `json:"method"`
	Path            string   `json:"path"`
	SuccessStatus   int      `json:"successStatus"`
	Group           string   `json:"group,omitempty"`
	RequiredHeaders []string `json:"requiredHeaders,omitempty"`
	Request         Request  `json:"request"`
	Response        Response `json:"response"`
	Error           Error    `json:"error"`
}

type Request struct {
	PathParams   []Parameter `json:"pathParams,omitempty"`
	QueryParams  []Parameter `json:"queryParams,omitempty"`
	HeaderParams []Parameter `json:"headerParams,omitempty"`
	Body         *TypeRef    `json:"body,omitempty"`
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
	WireName    string   `json:"wireName"`
	Type        TypeRef  `json:"type"`
	Required    bool     `json:"required"`
	Description string   `json:"description,omitempty"`
	Examples    []string `json:"examples,omitempty"`
}

type TypeDef struct {
	Name   string     `json:"name"`
	Kind   TypeKind   `json:"kind"`
	Fields []FieldDef `json:"fields,omitempty"`
}

type FieldDef struct {
	Name      string  `json:"name"`
	WireName  string  `json:"wireName"`
	Type      TypeRef `json:"type"`
	Required  bool    `json:"required"`
	Nullable  bool    `json:"nullable,omitempty"`
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

type TypeKind string

const (
	TypeKindAny    TypeKind = "any"
	TypeKindBool   TypeKind = "bool"
	TypeKindFloat  TypeKind = "float"
	TypeKindInt    TypeKind = "int"
	TypeKindList   TypeKind = "list"
	TypeKindMap    TypeKind = "map"
	TypeKindNamed  TypeKind = "named"
	TypeKindObject TypeKind = "object"
	TypeKindString TypeKind = "string"
	TypeKindUUID   TypeKind = "uuid"
)

func Normalize(doc *Document) {
	if doc == nil {
		return
	}
	if doc.Naming != nil {
		doc.Naming.Initialisms = NormalizeInitialisms(doc.Naming.Initialisms)
	}
	if doc.Groups == nil {
		doc.Groups = []Group{}
	}
	if doc.Endpoints == nil {
		doc.Endpoints = []Endpoint{}
	}
	if doc.Types == nil {
		doc.Types = []TypeDef{}
	}
	normalizeGroups(doc.Groups)
	normalizeEndpoints(doc.Endpoints)
	for i := range doc.Types {
		if doc.Types[i].Fields == nil {
			doc.Types[i].Fields = []FieldDef{}
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

func normalizeGroups(groups []Group) {
	for i := range groups {
		if groups[i].PathSegments == nil {
			groups[i].PathSegments = []string{}
		}
		if len(groups[i].PathSegments) == 0 && groups[i].Name != "" {
			groups[i].PathSegments = []string{groups[i].Name}
		}
		if groups[i].ID == "" {
			groups[i].ID = strings.Join(groups[i].PathSegments, ".")
		}
		if groups[i].RequiredHeaders == nil {
			groups[i].RequiredHeaders = []string{}
		}
		if groups[i].ProviderHeaders == nil {
			groups[i].ProviderHeaders = []Parameter{}
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
		if endpoints[i].RequiredHeaders == nil {
			endpoints[i].RequiredHeaders = []string{}
		}
		if endpoints[i].Request.PathParams == nil {
			endpoints[i].Request.PathParams = []Parameter{}
		}
		if endpoints[i].Request.QueryParams == nil {
			endpoints[i].Request.QueryParams = []Parameter{}
		}
		if endpoints[i].Request.HeaderParams == nil {
			endpoints[i].Request.HeaderParams = []Parameter{}
		}
		if endpoints[i].Error.Body.Kind == "" {
			endpoints[i].Error = Error{
				Body: TypeRef{Kind: TypeKindNamed, Name: BuiltinDefaultError},
			}
		}
	}
}
