package meta

import "reflect"

type EndpointStruct struct {
	StructName               string
	StructPkgPath            string
	StructQualifiedName      string
	SDKName                  string
	Method                   EndpointMethod
	Path                     string
	LeafPath                 string
	GroupPath                []string
	InheritedRequiredHeaders []string
	EndpointRequiredHeaders  []string
	FinalRequiredHeaders     []string
	SuccessStatus            int
	Request                  RequestField
	Dependencies             DependenciesField
	StructType               reflect.Type
	ErrorBodyType            reflect.Type
}

type PathParameterField struct {
	FieldName    string       // ex: UserID
	FieldIndex   int          // for faster indexing, not to use FieldByName
	FieldType    reflect.Type // ex: uuid.UUID
	VariableName string       // ex: userId
}

type QueryParameterField struct {
	FieldName  string // ex: Page, Age
	FieldIndex int
	FieldType  reflect.Type
	QueryKey   string // ex: page, query
}

type HeaderParameterField struct {
	FieldName  string
	FieldIndex int
	FieldType  reflect.Type
	HeaderName string
	Required   bool
}

type RequestField struct {
	PathParameterFields   []PathParameterField
	QueryParameterFields  []QueryParameterField
	HeaderParameterFields []HeaderParameterField
	FieldType             reflect.Type
}
