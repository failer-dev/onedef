package inspect

import (
	"net/http"
	"net/textproto"
	"reflect"
	"strconv"
	"strings"

	"github.com/failer-dev/onedef/onedef_go/internal/meta"
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
				Header: meta.HeaderContract{
					Name:     field.Name,
					WireName: headerName,
					Type:     field.Type,
				},
				Required: true,
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

func InspectProvide(structType reflect.Type) (meta.ProvideFieldSet, error) {
	if _, ok := structType.FieldByName("Deps"); ok {
		return meta.ProvideFieldSet{}, wherr.Errorf("onedef: %s.Deps is no longer supported; use Provide", structType.Name())
	}

	provideField, ok := structType.FieldByName("Provide")
	if !ok {
		return meta.ProvideFieldSet{}, nil
	}
	if provideField.Anonymous || provideField.Type.Kind() != reflect.Struct {
		return meta.ProvideFieldSet{}, wherr.Errorf("onedef: %s.Provide must be a struct field", structType.Name())
	}

	result := meta.ProvideFieldSet{
		Exists:      true,
		StructIndex: provideField.Index[0],
	}

	for i := 0; i < provideField.Type.NumField(); i++ {
		field := provideField.Type.Field(i)
		if field.Anonymous {
			return meta.ProvideFieldSet{}, wherr.Errorf("onedef: %s.Provide must not contain anonymous fields", structType.Name())
		}
		if !field.IsExported() {
			return meta.ProvideFieldSet{}, wherr.Errorf("onedef: %s.Provide.%s must be exported", structType.Name(), field.Name)
		}

		result.Fields = append(result.Fields, meta.ProvideField{
			FieldName:  field.Name,
			FieldIndex: i,
			FieldType:  field.Type,
		})
	}

	return result, nil
}

func InspectBeforeHandle(
	handler any,
	pathParams map[string]string,
	activeHeaders []meta.HeaderContract,
) (meta.BeforeHandleStruct, error) {
	handlerType := reflect.TypeOf(handler)
	if handlerType == nil {
		return meta.BeforeHandleStruct{}, wherr.Errorf("onedef: before handler must not be nil")
	}
	if handlerType.Kind() == reflect.Pointer {
		handlerType = handlerType.Elem()
	}
	if handlerType.Kind() != reflect.Struct {
		return meta.BeforeHandleStruct{}, wherr.Errorf("onedef: before handler must be a pointer to a struct")
	}

	request, err := inspectBeforeHandleRequest(handlerType, pathParams, activeHeaders)
	if err != nil {
		return meta.BeforeHandleStruct{}, err
	}
	provide, err := InspectProvide(handlerType)
	if err != nil {
		return meta.BeforeHandleStruct{}, err
	}

	return meta.BeforeHandleStruct{
		StructName: handlerType.Name(),
		Request:    request,
		Provide:    provide,
		StructType: handlerType,
	}, nil
}

func InspectAfterHandle(
	handler any,
	pathParams map[string]string,
	activeHeaders []meta.HeaderContract,
	endpointResponseType reflect.Type,
) (meta.AfterHandleStruct, error) {
	handlerType := reflect.TypeOf(handler)
	if handlerType == nil {
		return meta.AfterHandleStruct{}, wherr.Errorf("onedef: after handler must not be nil")
	}
	if handlerType.Kind() == reflect.Pointer {
		handlerType = handlerType.Elem()
	}
	if handlerType.Kind() != reflect.Struct {
		return meta.AfterHandleStruct{}, wherr.Errorf("onedef: after handler must be a pointer to a struct")
	}

	request, err := inspectScopedRequest(handlerType, pathParams, activeHeaders, "AfterHandle")
	if err != nil {
		return meta.AfterHandleStruct{}, err
	}
	provide, err := InspectProvide(handlerType)
	if err != nil {
		return meta.AfterHandleStruct{}, err
	}
	response, err := inspectAfterHandleResponse(handlerType, endpointResponseType)
	if err != nil {
		return meta.AfterHandleStruct{}, err
	}

	return meta.AfterHandleStruct{
		StructName: handlerType.Name(),
		Request:    request,
		Provide:    provide,
		Response:   response,
		StructType: handlerType,
	}, nil
}

func inspectBeforeHandleRequest(
	structType reflect.Type,
	pathParams map[string]string,
	activeHeaders []meta.HeaderContract,
) (meta.RequestField, error) {
	return inspectScopedRequest(structType, pathParams, activeHeaders, "BeforeHandle")
}

func inspectScopedRequest(
	structType reflect.Type,
	pathParams map[string]string,
	activeHeaders []meta.HeaderContract,
	hookName string,
) (meta.RequestField, error) {
	requestField, ok := structType.FieldByName("Request")
	if !ok {
		return meta.RequestField{}, nil
	}
	if requestField.Anonymous || requestField.Type.Kind() != reflect.Struct {
		return meta.RequestField{}, wherr.Errorf("onedef: %s.Request must be a struct field", structType.Name())
	}

	activeByWire := make(map[string]meta.HeaderContract, len(activeHeaders))
	for _, header := range activeHeaders {
		activeByWire[normalizeHeaderName(header.WireName)] = header
	}

	requestType := requestField.Type
	result := meta.RequestField{FieldType: requestType}
	for i := 0; i < requestType.NumField(); i++ {
		field := requestType.Field(i)
		if headerName, hasHeader := HeaderName(field); hasHeader {
			if !field.IsExported() {
				return meta.RequestField{}, wherr.Errorf("onedef: %s.Request.%s with header tag must be exported", structType.Name(), field.Name)
			}
			if headerName == "" {
				return meta.RequestField{}, wherr.Errorf("onedef: %s.Request.%s header tag must not be empty", structType.Name(), field.Name)
			}
			header, ok := activeByWire[normalizeHeaderName(headerName)]
			if !ok {
				return meta.RequestField{}, wherr.Errorf("onedef: header %q on %s handler %q Request.%s must be declared by RequireHeader in the endpoint or an ancestor group", headerName, hookName, structType.Name(), field.Name)
			}
			if err := validateHeaderFieldType(header, field.Type); err != nil {
				return meta.RequestField{}, wherr.Errorf("onedef: header %q on %s handler %q Request.%s: %w", headerName, hookName, structType.Name(), field.Name, err)
			}
			result.HeaderParameterFields = append(result.HeaderParameterFields, meta.HeaderParameterField{
				FieldName:  field.Name,
				FieldIndex: i,
				FieldType:  field.Type,
				Header:     header,
				Required:   true,
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
			continue
		}
		if field.Name == "Body" {
			return meta.RequestField{}, wherr.Errorf("onedef: %s.Request.Body is not available in %s", structType.Name(), hookName)
		}
		return meta.RequestField{}, wherr.Errorf("onedef: %s.Request.%s cannot be bound in %s; only declared headers and path params are supported", structType.Name(), field.Name, hookName)
	}

	return result, nil
}

func inspectAfterHandleResponse(
	structType reflect.Type,
	endpointResponseType reflect.Type,
) (meta.ResponseFieldSet, error) {
	responseField, ok := structType.FieldByName("Response")
	if !ok {
		return meta.ResponseFieldSet{}, nil
	}
	if responseField.Anonymous || responseField.Type.Kind() != reflect.Struct {
		return meta.ResponseFieldSet{}, wherr.Errorf("onedef: %s.Response must be a struct field", structType.Name())
	}
	if endpointResponseType.Kind() != reflect.Struct {
		return meta.ResponseFieldSet{}, wherr.Errorf("onedef: %s.Response cannot be used in AfterHandle because endpoint Response is not a struct", structType.Name())
	}

	result := meta.ResponseFieldSet{
		Exists:      true,
		StructIndex: responseField.Index[0],
		FieldType:   responseField.Type,
	}

	endpointFields := make(map[string]reflect.StructField, endpointResponseType.NumField())
	for i := 0; i < endpointResponseType.NumField(); i++ {
		field := endpointResponseType.Field(i)
		if !field.IsExported() {
			continue
		}
		endpointFields[field.Name] = field
	}

	for i := 0; i < responseField.Type.NumField(); i++ {
		field := responseField.Type.Field(i)
		if field.Anonymous {
			return meta.ResponseFieldSet{}, wherr.Errorf("onedef: %s.Response must not contain anonymous fields", structType.Name())
		}
		if !field.IsExported() {
			return meta.ResponseFieldSet{}, wherr.Errorf("onedef: %s.Response.%s must be exported", structType.Name(), field.Name)
		}

		source, ok := endpointFields[field.Name]
		if !ok {
			return meta.ResponseFieldSet{}, wherr.Errorf("onedef: %s.Response.%s does not exist on endpoint Response", structType.Name(), field.Name)
		}
		if !source.Type.AssignableTo(field.Type) && !source.Type.ConvertibleTo(field.Type) {
			return meta.ResponseFieldSet{}, wherr.Errorf("onedef: %s.Response.%s cannot receive endpoint Response.%s (%s to %s)", structType.Name(), field.Name, source.Name, source.Type, field.Type)
		}

		result.Fields = append(result.Fields, meta.ResponseParameterField{
			FieldName:        field.Name,
			FieldIndex:       i,
			SourceFieldIndex: source.Index[0],
			FieldType:        field.Type,
		})
	}

	return result, nil
}

func validateHeaderFieldType(header meta.HeaderContract, fieldType reflect.Type) error {
	value := reflect.Zero(header.Type)
	if _, err := meta.AssignHeaderValue(value, fieldType); err != nil {
		return err
	}
	return nil
}

func normalizeHeaderName(name string) string {
	return textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(name))
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
