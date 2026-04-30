package validator

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/textproto"
	"regexp"
	"sort"
	"strings"
)

const VersionV1 = "v1"

const BuiltinDefaultError = "DefaultError"

type ErrorCode string

const (
	ErrorCodeDuplicateHeader        ErrorCode = "duplicate_header"
	ErrorCodeDuplicateName          ErrorCode = "duplicate_name"
	ErrorCodeDuplicateBinding       ErrorCode = "duplicate_binding"
	ErrorCodeDuplicateModel         ErrorCode = "duplicate_model"
	ErrorCodeInvalidSuccessResponse ErrorCode = "invalid_success_response"
	ErrorCodeInvalidTypeRef         ErrorCode = "invalid_type_ref"
	ErrorCodePathParamMismatch      ErrorCode = "path_param_mismatch"
	ErrorCodeUnknownTypeRef         ErrorCode = "unknown_type_ref"
	ErrorCodeUnsupportedVersion     ErrorCode = "unsupported_version"
)

type ValidationError struct {
	Code    ErrorCode
	Path    string
	Message string
}

func (e *ValidationError) Error() string {
	if e == nil {
		return ""
	}
	if e.Path == "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s at %s: %s", e.Code, e.Path, e.Message)
}

func ErrorCodeOf(err error) ErrorCode {
	var validationErr *ValidationError
	if errors.As(err, &validationErr) {
		return validationErr.Code
	}
	return ""
}

func DecodeJSON(data []byte) (*Document, error) {
	var doc Document
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			return nil, validationError(ErrorCodeInvalidTypeRef, "$", "%v", err)
		}
		return nil, err
	}
	Normalize(&doc)
	if err := Validate(&doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func Normalize(doc *Document) {
	if doc == nil {
		return
	}
	doc.Initialisms = NormalizeInitialisms(doc.Initialisms)
	if doc.Models == nil {
		doc.Models = []ModelDef{}
	}
	if doc.Routes != nil {
		normalizeRoutes(doc.Routes)
	}
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

func Validate(doc *Document) error {
	if doc == nil {
		return validationError(ErrorCodeUnsupportedVersion, "$", "document must not be nil")
	}
	if doc.Version != VersionV1 {
		return validationError(ErrorCodeUnsupportedVersion, "$.version", "version must be v1")
	}
	if doc.Routes == nil {
		return validationError(ErrorCodeInvalidTypeRef, "$.routes", "routes is required")
	}

	models := make(map[string]ModelDef, len(doc.Models))
	for i, modelDef := range doc.Models {
		path := fmt.Sprintf("$.models[%d]", i)
		if modelDef.Name == "" {
			return validationError(ErrorCodeInvalidTypeRef, path+".name", "model name must not be empty")
		}
		if _, ok := models[modelDef.Name]; ok {
			return validationError(ErrorCodeDuplicateModel, path+".name", "model name %q is duplicated", modelDef.Name)
		}
		if modelDef.Kind != ModelKindObject {
			return validationError(ErrorCodeInvalidTypeRef, path+".kind", "model kind must be object")
		}
		models[modelDef.Name] = modelDef
	}

	for i, modelDef := range doc.Models {
		for j, field := range modelDef.Fields {
			path := fmt.Sprintf("$.models[%d].fields[%d].type", i, j)
			if err := validateTypeRef(field.Type, path, models); err != nil {
				return err
			}
		}
		if err := validateFields(modelDef.Fields, fmt.Sprintf("$.models[%d].fields", i)); err != nil {
			return err
		}
	}

	if err := validateRoutes(*doc.Routes, "$.routes", models); err != nil {
		return err
	}

	return nil
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

func validateRoutes(routes Routes, path string, models map[string]ModelDef) error {
	for i, header := range routes.Headers {
		if err := validateHeader(header, fmt.Sprintf("%s.headers[%d]", path, i), models); err != nil {
			return err
		}
	}
	if err := validateHeaders(routes.Headers, path+".headers"); err != nil {
		return err
	}
	if err := validateEndpointNames(routes.Endpoints, path+".endpoints"); err != nil {
		return err
	}
	if err := validateGroupNames(routes.Groups, path+".groups"); err != nil {
		return err
	}
	effectiveHeaders := cloneHeaders(routes.Headers)
	for i, endpoint := range routes.Endpoints {
		if err := validateEndpoint(endpoint, fmt.Sprintf("%s.endpoints[%d]", path, i), models, effectiveHeaders); err != nil {
			return err
		}
	}
	for i, group := range routes.Groups {
		if err := validateGroup(group, fmt.Sprintf("%s.groups[%d]", path, i), models, effectiveHeaders); err != nil {
			return err
		}
	}
	return nil
}

func validateGroup(group Group, path string, models map[string]ModelDef, inheritedHeaders []Header) error {
	if group.Name == "" {
		return validationError(ErrorCodeInvalidTypeRef, path+".name", "group name must not be empty")
	}
	for i, header := range group.Headers {
		if err := validateHeader(header, fmt.Sprintf("%s.headers[%d]", path, i), models); err != nil {
			return err
		}
	}
	if err := validateHeaders(group.Headers, path+".headers"); err != nil {
		return err
	}
	if err := validateEndpointNames(group.Endpoints, path+".endpoints"); err != nil {
		return err
	}
	if err := validateGroupNames(group.Groups, path+".groups"); err != nil {
		return err
	}
	effectiveHeaders := append(cloneHeaders(inheritedHeaders), group.Headers...)
	for i, endpoint := range group.Endpoints {
		if err := validateEndpoint(endpoint, fmt.Sprintf("%s.endpoints[%d]", path, i), models, effectiveHeaders); err != nil {
			return err
		}
	}
	for i, child := range group.Groups {
		if err := validateGroup(child, fmt.Sprintf("%s.groups[%d]", path, i), models, effectiveHeaders); err != nil {
			return err
		}
	}
	return nil
}

func validateEndpoint(endpoint Endpoint, path string, models map[string]ModelDef, groupHeaders []Header) error {
	if endpoint.Name == "" {
		return validationError(ErrorCodeInvalidTypeRef, path+".name", "endpoint name must not be empty")
	}
	if !validHTTPMethod(endpoint.Method) {
		return validationError(ErrorCodeInvalidTypeRef, path+".method", "unsupported HTTP method %q", endpoint.Method)
	}
	if !strings.HasPrefix(endpoint.Path, "/") {
		return validationError(ErrorCodePathParamMismatch, path+".path", "endpoint path must start with /")
	}
	if endpoint.SuccessStatus < 200 || endpoint.SuccessStatus > 299 {
		return validationError(ErrorCodeInvalidSuccessResponse, path+".successStatus", "success status must be 2xx")
	}
	if err := validatePathParams(endpoint, path); err != nil {
		return err
	}
	if err := validateRequest(endpoint.Request, path+".request", models); err != nil {
		return err
	}
	if err := validateGroupAndRequestHeaders(groupHeaders, endpoint.Request.Headers, path+".request.headers"); err != nil {
		return err
	}
	if err := validateResponse(endpoint, path+".response", models); err != nil {
		return err
	}
	if err := validateTypeRef(endpoint.Error.Body, path+".error.body", models); err != nil {
		return err
	}
	return nil
}

func validateRequest(request Request, path string, models map[string]ModelDef) error {
	for i, param := range request.Paths {
		if err := validateParameter(param, fmt.Sprintf("%s.paths[%d]", path, i), models); err != nil {
			return err
		}
	}
	for i, param := range request.Queries {
		if err := validateParameter(param, fmt.Sprintf("%s.queries[%d]", path, i), models); err != nil {
			return err
		}
	}
	for i, param := range request.Headers {
		if err := validateHeaderParameter(param, fmt.Sprintf("%s.headers[%d]", path, i), models); err != nil {
			return err
		}
	}
	if err := validateHeaderParameters(request.Headers, path+".headers"); err != nil {
		return err
	}
	if err := validateParameters(request.Paths, path+".paths"); err != nil {
		return err
	}
	if err := validateParameters(request.Queries, path+".queries"); err != nil {
		return err
	}
	if request.Body != nil {
		if err := validateTypeRef(*request.Body, path+".body", models); err != nil {
			return err
		}
	}
	return nil
}

func validateEndpointNames(endpoints []Endpoint, path string) error {
	seen := make(map[string]string, len(endpoints))
	for i, endpoint := range endpoints {
		name := endpoint.Name
		if endpoint.SDKName != "" {
			name = endpoint.SDKName
		}
		if existing, ok := seen[name]; ok {
			return validationError(ErrorCodeDuplicateName, fmt.Sprintf("%s[%d].name", path, i), "endpoint name %q duplicates %q", name, existing)
		}
		seen[name] = name
	}
	return nil
}

func validateGroupNames(groups []Group, path string) error {
	seen := make(map[string]string, len(groups))
	for i, group := range groups {
		if existing, ok := seen[group.Name]; ok {
			return validationError(ErrorCodeDuplicateName, fmt.Sprintf("%s[%d].name", path, i), "group name %q duplicates %q", group.Name, existing)
		}
		seen[group.Name] = group.Name
	}
	return nil
}

func validateFields(fields []FieldDef, path string) error {
	seenNames := make(map[string]string, len(fields))
	seenKeys := make(map[string]string, len(fields))
	for i, field := range fields {
		if existing, ok := seenNames[field.Name]; ok {
			return validationError(ErrorCodeDuplicateName, fmt.Sprintf("%s[%d].name", path, i), "field name %q duplicates %q", field.Name, existing)
		}
		seenNames[field.Name] = field.Name
		if existing, ok := seenKeys[field.Key]; ok {
			return validationError(ErrorCodeDuplicateBinding, fmt.Sprintf("%s[%d].key", path, i), "field key %q duplicates %q", field.Key, existing)
		}
		seenKeys[field.Key] = field.Key
	}
	return nil
}

func validateParameters(params []Parameter, path string) error {
	seen := make(map[string]string, len(params))
	for i, param := range params {
		if existing, ok := seen[param.Key]; ok {
			return validationError(ErrorCodeDuplicateBinding, fmt.Sprintf("%s[%d].key", path, i), "parameter key %q duplicates %q", param.Key, existing)
		}
		seen[param.Key] = param.Key
	}
	return nil
}

func validateParameter(param Parameter, path string, models map[string]ModelDef) error {
	if param.Name == "" {
		return validationError(ErrorCodeInvalidTypeRef, path+".name", "parameter name must not be empty")
	}
	if param.Key == "" {
		return validationError(ErrorCodeInvalidTypeRef, path+".key", "parameter key must not be empty")
	}
	return validateTypeRef(param.Type, path+".type", models)
}

func validateHeaderParameter(param HeaderParameter, path string, models map[string]ModelDef) error {
	if param.Name == "" {
		return validationError(ErrorCodeInvalidTypeRef, path+".name", "header parameter name must not be empty")
	}
	if param.Key == "" {
		return validationError(ErrorCodeInvalidTypeRef, path+".key", "header parameter key must not be empty")
	}
	return validateTypeRef(param.Type, path+".type", models)
}

func validateHeader(header Header, path string, models map[string]ModelDef) error {
	if header.Key == "" {
		return validationError(ErrorCodeInvalidTypeRef, path+".key", "header key must not be empty")
	}
	return validateTypeRef(header.Type, path+".type", models)
}

func validateHeaderParameters(params []HeaderParameter, path string) error {
	seen := make(map[string]string, len(params))
	for i, param := range params {
		normalized := normalizeHeaderName(param.Key)
		if existing, ok := seen[normalized]; ok {
			return validationError(ErrorCodeDuplicateHeader, fmt.Sprintf("%s[%d].key", path, i), "header parameter %q duplicates %q", param.Key, existing)
		}
		seen[normalized] = param.Key
	}
	return nil
}

func validateHeaders(headers []Header, path string) error {
	seen := make(map[string]string, len(headers))
	for i, header := range headers {
		normalized := normalizeHeaderName(header.Key)
		if existing, ok := seen[normalized]; ok {
			return validationError(ErrorCodeDuplicateHeader, fmt.Sprintf("%s[%d].key", path, i), "header %q duplicates %q", header.Key, existing)
		}
		seen[normalized] = header.Key
	}
	return nil
}

func validateGroupAndRequestHeaders(groupHeaders []Header, requestHeaders []HeaderParameter, path string) error {
	seen := make(map[string]string, len(groupHeaders))
	for _, header := range groupHeaders {
		seen[normalizeHeaderName(header.Key)] = header.Key
	}
	for i, header := range requestHeaders {
		normalized := normalizeHeaderName(header.Key)
		if existing, ok := seen[normalized]; ok {
			return validationError(ErrorCodeDuplicateHeader, fmt.Sprintf("%s[%d].key", path, i), "request header %q duplicates group header %q", header.Key, existing)
		}
	}
	return nil
}

func cloneHeaders(headers []Header) []Header {
	if headers == nil {
		return nil
	}
	return append([]Header(nil), headers...)
}

func validateResponse(endpoint Endpoint, path string, models map[string]ModelDef) error {
	if endpoint.SuccessStatus == http.StatusNoContent {
		if endpoint.Response.Envelope || endpoint.Response.Body != nil {
			return validationError(ErrorCodeInvalidSuccessResponse, path, "204 response must not use an envelope or body")
		}
		return nil
	}
	if !endpoint.Response.Envelope {
		return validationError(ErrorCodeInvalidSuccessResponse, path+".envelope", "non-204 response must use the success envelope")
	}
	if endpoint.Response.Body == nil {
		return validationError(ErrorCodeInvalidSuccessResponse, path+".body", "enveloped response must declare a body")
	}
	return validateTypeRef(*endpoint.Response.Body, path+".body", models)
}

func validateTypeRef(typeRef TypeRef, path string, models map[string]ModelDef) error {
	switch typeRef.Kind {
	case TypeKindAny, TypeKindBool, TypeKindFloat, TypeKindInt, TypeKindString, TypeKindUUID:
		return nil
	case TypeKindNamed:
		if typeRef.Name == "" {
			return validationError(ErrorCodeInvalidTypeRef, path+".name", "named type ref must declare name")
		}
		if typeRef.Name == BuiltinDefaultError {
			return nil
		}
		if _, ok := models[typeRef.Name]; !ok {
			return validationError(ErrorCodeUnknownTypeRef, path+".name", "unknown named type %q", typeRef.Name)
		}
		return nil
	case TypeKindList:
		if typeRef.Elem == nil {
			return validationError(ErrorCodeInvalidTypeRef, path+".elem", "list type ref must declare elem")
		}
		return validateTypeRef(*typeRef.Elem, path+".elem", models)
	case TypeKindMap:
		if typeRef.Key != nil && typeRef.Key.Kind != TypeKindString {
			return validationError(ErrorCodeInvalidTypeRef, path+".key", "map key must be string")
		}
		if typeRef.Value == nil {
			return validationError(ErrorCodeInvalidTypeRef, path+".value", "map type ref must declare value")
		}
		return validateTypeRef(*typeRef.Value, path+".value", models)
	default:
		return validationError(ErrorCodeInvalidTypeRef, path+".kind", "unsupported type kind %q", typeRef.Kind)
	}
}

var pathParamPattern = regexp.MustCompile(`\{(\w+)\}`)

func validatePathParams(endpoint Endpoint, path string) error {
	matches := pathParamPattern.FindAllStringSubmatch(endpoint.Path, -1)
	pathVars := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		pathVars[match[1]] = struct{}{}
	}

	paramVars := make(map[string]struct{}, len(endpoint.Request.Paths))
	for _, param := range endpoint.Request.Paths {
		paramVars[param.Key] = struct{}{}
	}

	for pathVar := range pathVars {
		if _, ok := paramVars[pathVar]; !ok {
			return validationError(ErrorCodePathParamMismatch, path+".request.paths", "path variable %q is missing from request paths", pathVar)
		}
	}
	for paramVar := range paramVars {
		if _, ok := pathVars[paramVar]; !ok {
			return validationError(ErrorCodePathParamMismatch, path+".request.paths", "path parameter %q does not exist in endpoint path", paramVar)
		}
	}
	return nil
}

func validHTTPMethod(method HTTPMethod) bool {
	switch method {
	case HTTPMethodGET, HTTPMethodPOST, HTTPMethodPUT, HTTPMethodPATCH, HTTPMethodDELETE, HTTPMethodHEAD, HTTPMethodOPTIONS:
		return true
	default:
		return false
	}
}

func normalizeHeaderName(name string) string {
	return textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(name))
}

func validationError(code ErrorCode, path, format string, args ...any) *ValidationError {
	return &ValidationError{
		Code:    code,
		Path:    path,
		Message: fmt.Sprintf(format, args...),
	}
}
