package ir

import (
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/failer-dev/onedef/internal/inspect"
	"github.com/failer-dev/onedef/internal/meta"
	"github.com/failer-dev/wherr"
)

type typeCollector struct {
	defs     map[string]TypeDef
	building map[string]bool
	order    []string
}

type ParseOptions struct {
	Initialisms []string
	ExtraTypes  []any
}

func Parse(endpoints ...any) (*Spec, error) {
	return ParseWithOptions(ParseOptions{}, endpoints...)
}

func ParseWithOptions(opts ParseOptions, endpoints ...any) (*Spec, error) {
	parser := typeCollector{
		defs:     map[string]TypeDef{},
		building: map[string]bool{},
	}

	result := &Spec{
		Version: "v1",
	}
	if initialisms := normalizeInitialisms(opts.Initialisms); len(initialisms) > 0 {
		result.Naming = &Naming{Initialisms: initialisms}
	}

	for _, endpoint := range endpoints {
		parsed, err := parser.parseEndpoint(endpoint)
		if err != nil {
			return nil, err
		}
		result.Endpoints = append(result.Endpoints, parsed)
	}
	for _, extra := range opts.ExtraTypes {
		if err := parser.parseExtraType(extra); err != nil {
			return nil, err
		}
	}

	result.Endpoints = assignGroups(result.Endpoints)
	for _, name := range parser.order {
		result.Types = append(result.Types, parser.defs[name])
	}

	return result, nil
}

func MustParse(endpoints ...any) *Spec {
	spec, err := Parse(endpoints...)
	if err != nil {
		panic(err)
	}
	return spec
}

func normalizeInitialisms(values []string) []string {
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

func (c *typeCollector) parseEndpoint(endpoint any) (Endpoint, error) {
	structType := reflect.TypeOf(endpoint)
	if structType == nil {
		return Endpoint{}, wherr.Errorf("endpoint must not be nil")
	}
	if structType.Kind() == reflect.Pointer {
		structType = structType.Elem()
	}
	if structType.Kind() != reflect.Struct {
		return Endpoint{}, wherr.Errorf("endpoint %T must be a struct or pointer to struct", endpoint)
	}

	method, path, pathParams, successStatus, err := inspect.InspectEndpointMethodMarker(structType)
	if err != nil {
		return Endpoint{}, err
	}
	requestField, err := inspect.InspectRequest(structType, method, pathParams)
	if err != nil {
		return Endpoint{}, err
	}

	result := Endpoint{
		Name:          structType.Name(),
		Method:        string(method),
		Path:          path,
		SuccessStatus: successStatus,
		Error: Error{
			Body: TypeRef{Kind: TypeKindNamed, Name: "DefaultError"},
		},
	}

	for _, p := range requestField.PathParameterFields {
		typeRef, err := c.parseTypeRef(p.FieldType, structType.Name()+p.FieldName)
		if err != nil {
			return Endpoint{}, err
		}
		result.Request.PathParams = append(result.Request.PathParams, Parameter{
			Name:     p.FieldName,
			WireName: p.VariableName,
			Type:     typeRef,
			Required: true,
		})
	}

	for _, q := range requestField.QueryParameterFields {
		typeRef, err := c.parseTypeRef(q.FieldType, structType.Name()+q.FieldName)
		if err != nil {
			return Endpoint{}, err
		}
		result.Request.QueryParams = append(result.Request.QueryParams, Parameter{
			Name:     q.FieldName,
			WireName: q.QueryKey,
			Type:     typeRef,
			Required: false,
		})
	}

	for _, h := range requestField.HeaderParameterFields {
		typeRef, err := c.parseTypeRef(h.FieldType, structType.Name()+h.FieldName)
		if err != nil {
			return Endpoint{}, err
		}
		result.Request.HeaderParams = append(result.Request.HeaderParams, Parameter{
			Name:     h.FieldName,
			WireName: h.HeaderName,
			Type:     typeRef,
			Required: h.Required,
		})
	}

	if method == meta.EndpointMethodPost || method == meta.EndpointMethodPut || method == meta.EndpointMethodPatch {
		bodyRef, err := c.parseRequestBody(structType, requestField)
		if err != nil {
			return Endpoint{}, err
		}
		result.Request.Body = bodyRef
	}

	responseField, ok := structType.FieldByName("Response")
	if !ok {
		return Endpoint{}, wherr.Errorf("struct %q must have a Response field", structType.Name())
	}
	if successStatus == 204 && responseField.Type.Kind() == reflect.Struct && responseField.Type.NumField() == 0 {
		result.Response = Response{Envelope: false}
		return result, nil
	}

	typeRef, err := c.parseTypeRef(responseField.Type, structType.Name()+"Response")
	if err != nil {
		return Endpoint{}, err
	}
	result.Response = Response{
		Envelope: true,
		Body:     &typeRef,
	}

	return result, nil
}

func (c *typeCollector) parseExtraType(value any) error {
	t := reflect.TypeOf(value)
	if t == nil {
		return wherr.Errorf("extra type must not be nil")
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || t.Name() == "" {
		return wherr.Errorf("extra type %s must be a non-pointer named struct", t)
	}
	_, err := c.parseTypeRef(t, t.Name())
	return err
}

func (c *typeCollector) parseRequestBody(structType reflect.Type, request meta.RequestField) (*TypeRef, error) {
	requestType := request.FieldType
	include := make([]int, 0, requestType.NumField())
	pathFieldIndexes := make([]int, 0, len(request.PathParameterFields))
	for _, p := range request.PathParameterFields {
		pathFieldIndexes = append(pathFieldIndexes, p.FieldIndex)
	}
	headerFieldIndexes := make([]int, 0, len(request.HeaderParameterFields))
	for _, h := range request.HeaderParameterFields {
		headerFieldIndexes = append(headerFieldIndexes, h.FieldIndex)
	}

	for i := 0; i < requestType.NumField(); i++ {
		field := requestType.Field(i)
		if !field.IsExported() || field.Anonymous || fieldShouldBeOmitted(field) {
			continue
		}
		if slices.Contains(pathFieldIndexes, i) {
			continue
		}
		if slices.Contains(headerFieldIndexes, i) {
			continue
		}
		include = append(include, i)
	}

	if len(include) == 0 {
		return nil, nil
	}

	bodyName := structType.Name() + "Request"
	if requestType.Name() != "" && len(include) == countSerializableFieldsForBody(requestType) {
		typeRef, err := c.parseTypeRef(requestType, bodyName)
		if err != nil {
			return nil, err
		}
		return &typeRef, nil
	}

	if err := c.ensureSyntheticType(bodyName, requestType, include); err != nil {
		return nil, err
	}
	return &TypeRef{Kind: TypeKindNamed, Name: bodyName}, nil
}

func (c *typeCollector) parseTypeRef(t reflect.Type, nameHint string) (TypeRef, error) {
	nullable := false
	for t.Kind() == reflect.Pointer {
		nullable = true
		t = t.Elem()
	}

	typeRef, err := c.parseNonPointerTypeRef(t, nameHint)
	if err != nil {
		return TypeRef{}, err
	}
	typeRef.Nullable = nullable
	return typeRef, nil
}

func (c *typeCollector) parseNonPointerTypeRef(t reflect.Type, nameHint string) (TypeRef, error) {
	if isUUIDType(t) {
		return TypeRef{Kind: TypeKindUUID}, nil
	}

	switch t.Kind() {
	case reflect.Bool:
		return TypeRef{Kind: TypeKindBool}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return TypeRef{Kind: TypeKindInt}, nil
	case reflect.Float32, reflect.Float64:
		return TypeRef{Kind: TypeKindFloat}, nil
	case reflect.String:
		return TypeRef{Kind: TypeKindString}, nil
	case reflect.Interface:
		return TypeRef{Kind: TypeKindAny}, nil
	case reflect.Slice:
		elem, err := c.parseTypeRef(t.Elem(), nameHint+"Item")
		if err != nil {
			return TypeRef{}, err
		}
		return TypeRef{Kind: TypeKindList, Elem: &elem}, nil
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return TypeRef{}, wherr.Errorf("unsupported map key type %s", t.Key())
		}
		key, err := c.parseTypeRef(t.Key(), nameHint+"Key")
		if err != nil {
			return TypeRef{}, err
		}
		value, err := c.parseTypeRef(t.Elem(), nameHint+"Value")
		if err != nil {
			return TypeRef{}, err
		}
		return TypeRef{
			Kind:  TypeKindMap,
			Key:   &key,
			Value: &value,
		}, nil
	case reflect.Struct:
		name := t.Name()
		if name == "" {
			name = nameHint
		}
		if err := c.ensureStructType(name, t); err != nil {
			return TypeRef{}, err
		}
		return TypeRef{Kind: TypeKindNamed, Name: name}, nil
	default:
		return TypeRef{}, wherr.Errorf("unsupported type %s", t)
	}
}

func (c *typeCollector) ensureStructType(name string, t reflect.Type) error {
	if existing, ok := c.defs[name]; ok && !c.building[name] {
		next, err := c.buildTypeDef(name, t, nil)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(existing, next) {
			return wherr.Errorf("type name %q is defined with conflicting shapes", name)
		}
		return nil
	}
	if c.building[name] {
		return nil
	}

	c.order = append(c.order, name)
	c.defs[name] = TypeDef{Name: name, Kind: TypeKindObject}
	c.building[name] = true

	next, err := c.buildTypeDef(name, t, nil)
	if err != nil {
		delete(c.building, name)
		return err
	}

	c.defs[name] = next
	delete(c.building, name)
	return nil
}

func (c *typeCollector) ensureSyntheticType(name string, t reflect.Type, include []int) error {
	if existing, ok := c.defs[name]; ok && !c.building[name] {
		next, err := c.buildTypeDef(name, t, include)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(existing, next) {
			return wherr.Errorf("type name %q is defined with conflicting shapes", name)
		}
		return nil
	}
	if c.building[name] {
		return nil
	}

	c.order = append(c.order, name)
	c.defs[name] = TypeDef{Name: name, Kind: TypeKindObject}
	c.building[name] = true

	next, err := c.buildTypeDef(name, t, include)
	if err != nil {
		delete(c.building, name)
		return err
	}

	c.defs[name] = next
	delete(c.building, name)
	return nil
}

func (c *typeCollector) buildTypeDef(name string, t reflect.Type, include []int) (TypeDef, error) {
	def := TypeDef{
		Name: name,
		Kind: TypeKindObject,
	}

	includeAll := include == nil
	includeSet := map[int]bool{}
	for _, index := range include {
		includeSet[index] = true
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() || field.Anonymous || fieldShouldBeOmitted(field) {
			continue
		}
		if !includeAll && !includeSet[i] {
			continue
		}

		typeRef, err := c.parseTypeRef(field.Type, name+field.Name)
		if err != nil {
			return TypeDef{}, err
		}

		def.Fields = append(def.Fields, FieldDef{
			Name:      field.Name,
			WireName:  inspect.WireName(field),
			Type:      typeRef,
			Required:  !typeRef.Nullable && !fieldHasOmitEmpty(field),
			Nullable:  typeRef.Nullable,
			OmitEmpty: fieldHasOmitEmpty(field),
		})
	}

	return def, nil
}

func countSerializableFieldsForBody(t reflect.Type) int {
	count := 0
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() || field.Anonymous || fieldShouldBeOmitted(field) {
			continue
		}
		if _, hasHeader := inspect.HeaderName(field); hasHeader {
			continue
		}
		count++
	}
	return count
}

func isUUIDType(t reflect.Type) bool {
	return t.PkgPath() == "github.com/google/uuid" && t.Name() == "UUID"
}

func fieldShouldBeOmitted(field reflect.StructField) bool {
	tag := field.Tag.Get("json")
	return tag == "-"
}

func fieldHasOmitEmpty(field reflect.StructField) bool {
	tag := field.Tag.Get("json")
	if tag == "" || tag == "-" {
		return false
	}
	parts := strings.Split(tag, ",")
	for _, part := range parts[1:] {
		if part == "omitempty" {
			return true
		}
	}
	return false
}
