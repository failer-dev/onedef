package inspect

import (
	"net/http"
	"reflect"
	"strconv"

	"github.com/failer-dev/onedef/internal/meta"
	"github.com/failer-dev/wherr"
)

var markerReflectType = reflect.TypeOf((*meta.EndpointMethodMarker)(nil)).Elem()

func InspectEndpointMethodMarker(structType reflect.Type) (meta.EndpointMethod, string, map[string]string, int, error) {
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
		return "", "", nil, 0, wherr.Errorf("struct %q must embed an EndpointMethodMarker (onedef.GET, onedef.POST, etc.)", structType.Name())
	}

	path, ok := markerField.Tag.Lookup("path")
	if !ok {
		return "", "", nil, 0, wherr.Errorf("endpointMethodMarker field %q in %q must have a `path` tag", markerField.Name, structType.Name())
	}

	pathParams := extractPathParams(path)
	successStatus, err := inspectSuccessStatus(markerField, structType.Name())
	if err != nil {
		return "", "", nil, 0, err
	}

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
		return "", "", nil, 0, wherr.Errorf("struct %q has unknown EndpointMethodMarker type %q", structType.Name(), markerField.Type.Name())
	}

	return method, path, pathParams, successStatus, nil
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
		if headerName, hasHeader := HeaderName(field); hasHeader {
			if !field.IsExported() {
				return meta.RequestField{}, wherr.Errorf("onedef: %s.Request.%s with header tag must be exported", structType.Name(), field.Name)
			}
			if headerName == "" {
				return meta.RequestField{}, wherr.Errorf("onedef: %s.Request.%s header tag must not be empty", structType.Name(), field.Name)
			}
			result.HeaderParameterFields = append(result.HeaderParameterFields, meta.HeaderParameterField{
				FieldName:  field.Name,
				FieldIndex: i,
				FieldType:  field.Type,
				HeaderName: headerName,
				Required:   field.Type.Kind() != reflect.Pointer,
			})
			continue
		}
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

func InspectDependencies(structType reflect.Type) (meta.DependenciesField, error) {
	depsField, ok := structType.FieldByName("Deps")
	if !ok {
		return meta.DependenciesField{}, nil
	}
	if depsField.Anonymous || depsField.Type.Kind() != reflect.Struct {
		return meta.DependenciesField{}, wherr.Errorf("onedef: %s.Deps must be a struct field", structType.Name())
	}

	result := meta.DependenciesField{
		Exists:      true,
		StructIndex: depsField.Index[0],
	}

	for i := 0; i < depsField.Type.NumField(); i++ {
		field := depsField.Type.Field(i)
		if field.Anonymous {
			return meta.DependenciesField{}, wherr.Errorf("onedef: %s.Deps must not contain anonymous fields", structType.Name())
		}
		if !field.IsExported() {
			return meta.DependenciesField{}, wherr.Errorf("onedef: %s.Deps.%s must be exported", structType.Name(), field.Name)
		}

		result.Fields = append(result.Fields, meta.DependencyField{
			FieldName:  field.Name,
			FieldIndex: i,
			FieldType:  field.Type,
		})
	}

	return result, nil
}

func inspectSuccessStatus(markerField reflect.StructField, structName string) (int, error) {
	raw, ok := markerField.Tag.Lookup("status")
	if !ok || raw == "" {
		return http.StatusOK, nil
	}

	status, err := strconv.Atoi(raw)
	if err != nil {
		return 0, wherr.Errorf(
			"endpointMethodMarker field %q in %q has invalid `status` tag %q: %w",
			markerField.Name,
			structName,
			raw,
			err,
		)
	}
	if status < 200 || status > 299 {
		return 0, wherr.Errorf(
			"endpointMethodMarker field %q in %q has invalid `status` tag %q: status must be between 200 and 299",
			markerField.Name,
			structName,
			raw,
		)
	}

	return status, nil
}
