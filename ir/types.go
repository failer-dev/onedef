package ir

type Spec struct {
	Version   string     `json:"version"`
	Naming    *Naming    `json:"naming,omitempty"`
	Endpoints []Endpoint `json:"endpoints"`
	Types     []TypeDef  `json:"types"`
}

type Naming struct {
	Initialisms []string `json:"initialisms,omitempty"`
}

type Endpoint struct {
	Name          string   `json:"name"`
	SDKName       string   `json:"sdkName,omitempty"`
	Method        string   `json:"method"`
	Path          string   `json:"path"`
	SuccessStatus int      `json:"successStatus"`
	Group         string   `json:"group,omitempty"`
	Request       Request  `json:"request"`
	Response      Response `json:"response"`
	Error         Error    `json:"error"`
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
	Name     string  `json:"name"`
	WireName string  `json:"wireName"`
	Type     TypeRef `json:"type"`
	Required bool    `json:"required"`
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
