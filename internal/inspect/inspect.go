package inspect

import (
	"reflect"

	"github.com/failer-dev/onedef/internal/meta"
	"github.com/failer-dev/wherr"
)

var markerReflectType = reflect.TypeOf((*meta.EndpointMethodMarker)(nil)).Elem()

func InspectEndpointMethodMarker(structType reflect.Type) (meta.EndpointMethod, string, map[string]string, error) {
	var markerField reflect.StructField
	found := false

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if field.Anonymous && field.Type.Implements(markerReflectType) {
			markerField = field
			found = true
		}
	}

	if !found {
		return "", "", nil, wherr.Errorf("struct %q must embed an EndpointMethodMarker (onedef.GET, onedef.POST, etc.)", structType.Name())
	}

	path, ok := markerField.Tag.Lookup("path")
	if !ok {
		return "", "", nil, wherr.Errorf("endpointMethodMarker field %q in %q must have a `path` tag", markerField.Name, structType.Name())
	}

	pathParams := extractPathParams(path)

	var method meta.EndpointMethod
	marker := reflect.New(markerField.Type).Elem().Interface()
	switch marker.(type) {
	case meta.GET:
		method = meta.EndpointMethodGet
	case meta.POST:
		method = meta.EndpointMethodPost
	case meta.PUT:
		method = meta.EndpointMethodPut
	case meta.PATCH:
		method = meta.EndpointMethodPatch
	case meta.DELETE:
		method = meta.EndpointMethodDelete
	case meta.HEAD:
		method = meta.EndpointMethodHead
	case meta.OPTIONS:
		method = meta.EndpointMethodOptions
	default:
		return "", "", nil, wherr.Errorf("struct %q has unknown EndpointMethodMarker type %q", structType.Name(), markerField.Type.Name())
	}

	return method, path, pathParams, nil
}

func InspectRequest(
	structType reflect.Type,
	method meta.EndpointMethod,
	pathParams map[string]string,
) (meta.RequestField, error) {
	requestField, ok := structType.FieldByName("Request")
	if !ok {
		return meta.RequestField{}, wherr.Errorf("struct %q must have a Request field", structType.Name())
	}

	requestType := requestField.Type
	result := meta.RequestField{
		FieldType: requestType,
	}

	for i := 0; i < requestType.NumField(); i++ {
		field := requestType.Field(i)
		if !field.IsExported() {
			continue
		}

		if variableName, isPath := pathParams[normalizePathParam(field.Name)]; isPath {
			result.PathParameterFields = append(result.PathParameterFields, meta.PathParameterField{
				FieldName:    field.Name,
				FieldIndex:   i,
				FieldType:    field.Type,
				VariableName: variableName,
			})
		} else if method == meta.EndpointMethodGet || method == meta.EndpointMethodDelete {
			result.QueryParameterFields = append(result.QueryParameterFields, meta.QueryParameterField{
				FieldName:  field.Name,
				FieldIndex: i,
				FieldType:  field.Type,
				QueryKey:   WireName(field),
			})
		}
		// POST/PUT/PATCH의 body 필드는 RequestType 통째로 json.Unmarshal하므로 개별 추적 불필요
	}

	return result, nil
}
