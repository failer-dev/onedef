package meta

import "reflect"

// EndpointStruct is the fully inspected metadata for one endpoint struct.
// A zero value is not meaningful; values are expected to come from the
// inspector so reflection indexes, request metadata, and response type data
// stay consistent.
type EndpointStruct struct {
	StructName               string
	StructPkgPath            string
	StructQualifiedName      string
	SDKName                  string
	Method                   EndpointMethod
	Path                     string
	LeafPath                 string
	GroupPath                []string
	InheritedRequiredHeaders []HeaderContract
	EndpointRequiredHeaders  []HeaderContract
	FinalRequiredHeaders     []HeaderContract
	SuccessStatus            int
	Request                  RequestField
	Provide                  ProvideFieldSet
	BeforeHandlers           []BeforeHandleStruct
	AfterHandlers            []AfterHandleStruct
	Observers                []Observer
	StructType               reflect.Type
	ErrorBodyType            reflect.Type
}

// PathParameterField identifies a Request field populated from a path variable.
// FieldIndex indexes the endpoint's Request struct and must match FieldType.
type PathParameterField struct {
	FieldName    string       // ex: UserID
	FieldIndex   int          // for faster indexing, not to use FieldByName
	FieldType    reflect.Type // ex: uuid.UUID
	VariableName string       // ex: userId
}

// QueryParameterField identifies a Request field populated from the query
// string. It is only collected for methods whose request body is not decoded
// into the whole Request struct.
type QueryParameterField struct {
	FieldName  string // ex: Page, Age
	FieldIndex int
	FieldType  reflect.Type
	QueryKey   string // ex: page, query
}

// HeaderParameterField identifies a Request field populated from an HTTP
// header. Required is false only for pointer fields.
type HeaderParameterField struct {
	FieldName       string
	FieldIndex      int
	FieldType       reflect.Type
	Header          HeaderContract
	Required        bool
	MethodParameter bool
}

// RequestField groups the reflected Request metadata needed by the runtime and
// IR parser. FieldType is the original Request struct type.
type RequestField struct {
	PathParameterFields   []PathParameterField
	QueryParameterFields  []QueryParameterField
	HeaderParameterFields []HeaderParameterField
	FieldType             reflect.Type
}

type ResponseParameterField struct {
	FieldName        string
	FieldIndex       int
	SourceFieldIndex int
	FieldType        reflect.Type
}

type ResponseFieldSet struct {
	Exists      bool
	StructIndex int
	FieldType   reflect.Type
	Fields      []ResponseParameterField
}
