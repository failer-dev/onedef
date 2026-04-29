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
	ErrorCodeDuplicateType          ErrorCode = "duplicate_type"
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
	if err := json.Unmarshal(data, &doc); err != nil {
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

func Validate(doc *Document) error {
	if doc == nil {
		return validationError(ErrorCodeUnsupportedVersion, "$", "document must not be nil")
	}
	if doc.Version != VersionV1 {
		return validationError(ErrorCodeUnsupportedVersion, "$.version", "version must be v1")
	}

	types := make(map[string]TypeDef, len(doc.Types))
	for i, typeDef := range doc.Types {
		path := fmt.Sprintf("$.types[%d]", i)
		if typeDef.Name == "" {
			return validationError(ErrorCodeInvalidTypeRef, path+".name", "type name must not be empty")
		}
		if _, ok := types[typeDef.Name]; ok {
			return validationError(ErrorCodeDuplicateType, path+".name", "type name %q is duplicated", typeDef.Name)
		}
		if typeDef.Kind != TypeKindObject {
			return validationError(ErrorCodeInvalidTypeRef, path+".kind", "type kind must be object")
		}
		types[typeDef.Name] = typeDef
	}

	for i, typeDef := range doc.Types {
		for j, field := range typeDef.Fields {
			path := fmt.Sprintf("$.types[%d].fields[%d].type", i, j)
			if err := validateTypeRef(field.Type, path, types); err != nil {
				return err
			}
		}
	}

	for i, endpoint := range doc.Endpoints {
		if err := validateEndpoint(endpoint, fmt.Sprintf("$.endpoints[%d]", i), types); err != nil {
			return err
		}
	}
	for i, group := range doc.Groups {
		if err := validateGroup(group, fmt.Sprintf("$.groups[%d]", i), types); err != nil {
			return err
		}
	}

	return nil
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

func validateGroup(group Group, path string, types map[string]TypeDef) error {
	if group.Name == "" {
		return validationError(ErrorCodeInvalidTypeRef, path+".name", "group name must not be empty")
	}
	if err := validateHeaderNames(group.RequiredHeaders, path+".requiredHeaders"); err != nil {
		return err
	}
	for i, param := range group.ProviderHeaders {
		if err := validateParameter(param, fmt.Sprintf("%s.providerHeaders[%d]", path, i), types); err != nil {
			return err
		}
	}
	if err := validateHeaderParams(group.ProviderHeaders, path+".providerHeaders"); err != nil {
		return err
	}
	for i, endpoint := range group.Endpoints {
		if err := validateEndpoint(endpoint, fmt.Sprintf("%s.endpoints[%d]", path, i), types); err != nil {
			return err
		}
	}
	for i, child := range group.Groups {
		if err := validateGroup(child, fmt.Sprintf("%s.groups[%d]", path, i), types); err != nil {
			return err
		}
	}
	return nil
}

func validateEndpoint(endpoint Endpoint, path string, types map[string]TypeDef) error {
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
	if err := validateHeaderNames(endpoint.RequiredHeaders, path+".requiredHeaders"); err != nil {
		return err
	}
	if err := validatePathParams(endpoint, path); err != nil {
		return err
	}
	if err := validateRequest(endpoint.Request, path+".request", types); err != nil {
		return err
	}
	if err := validateResponse(endpoint, path+".response", types); err != nil {
		return err
	}
	if err := validateTypeRef(endpoint.Error.Body, path+".error.body", types); err != nil {
		return err
	}
	return nil
}

func validateRequest(request Request, path string, types map[string]TypeDef) error {
	for i, param := range request.PathParams {
		if err := validateParameter(param, fmt.Sprintf("%s.pathParams[%d]", path, i), types); err != nil {
			return err
		}
		if !param.Required {
			return validationError(ErrorCodePathParamMismatch, fmt.Sprintf("%s.pathParams[%d].required", path, i), "path parameters must be required")
		}
	}
	for i, param := range request.QueryParams {
		if err := validateParameter(param, fmt.Sprintf("%s.queryParams[%d]", path, i), types); err != nil {
			return err
		}
	}
	for i, param := range request.HeaderParams {
		if err := validateParameter(param, fmt.Sprintf("%s.headerParams[%d]", path, i), types); err != nil {
			return err
		}
	}
	if err := validateHeaderParams(request.HeaderParams, path+".headerParams"); err != nil {
		return err
	}
	if request.Body != nil {
		if err := validateTypeRef(*request.Body, path+".body", types); err != nil {
			return err
		}
	}
	return nil
}

func validateParameter(param Parameter, path string, types map[string]TypeDef) error {
	if param.Name == "" {
		return validationError(ErrorCodeInvalidTypeRef, path+".name", "parameter name must not be empty")
	}
	if param.WireName == "" {
		return validationError(ErrorCodeInvalidTypeRef, path+".wireName", "parameter wireName must not be empty")
	}
	return validateTypeRef(param.Type, path+".type", types)
}

func validateHeaderNames(headers []string, path string) error {
	seen := make(map[string]string, len(headers))
	for i, header := range headers {
		normalized := normalizeHeaderName(header)
		if normalized == "" {
			return validationError(ErrorCodeInvalidTypeRef, fmt.Sprintf("%s[%d]", path, i), "header name must not be empty")
		}
		if existing, ok := seen[normalized]; ok {
			return validationError(ErrorCodeDuplicateHeader, fmt.Sprintf("%s[%d]", path, i), "header %q duplicates %q", header, existing)
		}
		seen[normalized] = header
	}
	return nil
}

func validateHeaderParams(params []Parameter, path string) error {
	seen := make(map[string]string, len(params))
	for i, param := range params {
		normalized := normalizeHeaderName(param.WireName)
		if existing, ok := seen[normalized]; ok {
			return validationError(ErrorCodeDuplicateHeader, fmt.Sprintf("%s[%d].wireName", path, i), "header parameter %q duplicates %q", param.WireName, existing)
		}
		seen[normalized] = param.WireName
	}
	return nil
}

func validateResponse(endpoint Endpoint, path string, types map[string]TypeDef) error {
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
	return validateTypeRef(*endpoint.Response.Body, path+".body", types)
}

func validateTypeRef(typeRef TypeRef, path string, types map[string]TypeDef) error {
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
		if _, ok := types[typeRef.Name]; !ok {
			return validationError(ErrorCodeUnknownTypeRef, path+".name", "unknown named type %q", typeRef.Name)
		}
		return nil
	case TypeKindList:
		if typeRef.Elem == nil {
			return validationError(ErrorCodeInvalidTypeRef, path+".elem", "list type ref must declare elem")
		}
		return validateTypeRef(*typeRef.Elem, path+".elem", types)
	case TypeKindMap:
		if typeRef.Key != nil && typeRef.Key.Kind != TypeKindString {
			return validationError(ErrorCodeInvalidTypeRef, path+".key", "map key must be string")
		}
		if typeRef.Value == nil {
			return validationError(ErrorCodeInvalidTypeRef, path+".value", "map type ref must declare value")
		}
		return validateTypeRef(*typeRef.Value, path+".value", types)
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

	paramVars := make(map[string]struct{}, len(endpoint.Request.PathParams))
	for _, param := range endpoint.Request.PathParams {
		paramVars[param.WireName] = struct{}{}
	}

	for pathVar := range pathVars {
		if _, ok := paramVars[pathVar]; !ok {
			return validationError(ErrorCodePathParamMismatch, path+".request.pathParams", "path variable %q is missing from request pathParams", pathVar)
		}
	}
	for paramVar := range paramVars {
		if _, ok := pathVars[paramVar]; !ok {
			return validationError(ErrorCodePathParamMismatch, path+".request.pathParams", "path parameter %q does not exist in endpoint path", paramVar)
		}
	}
	return nil
}

func validHTTPMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions:
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
