package meta

import "reflect"

type EndpointStruct struct {
	StructName        string
	Method            EndpointMethod
	Path              string
	Request           RequestField
	StructType reflect.Type
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

type RequestField struct {
	PathParameterFields  []PathParameterField
	QueryParameterFields []QueryParameterField
	FieldType            reflect.Type
}
