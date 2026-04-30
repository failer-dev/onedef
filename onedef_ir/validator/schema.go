package validator

type Document struct {
	Version     string     `json:"version"`
	Initialisms []string   `json:"initialisms,omitempty"`
	Routes      *Routes    `json:"routes"`
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
