package irbuild

import (
	"reflect"
	"slices"
	"strings"

	"github.com/failer-dev/onedef/onedef_go/internal/inspect"
	ir "github.com/failer-dev/onedef/onedef_go/internal/irspec"
	"github.com/failer-dev/onedef/onedef_go/internal/meta"
	"github.com/failer-dev/wherr"
)

type Options struct {
	Initialisms []string
	Endpoints   []meta.EndpointStruct
	Groups      []*meta.GroupMeta
}

func BuildDocument(opts Options) (*ir.Document, error) {
	collector := typeCollector{
		defs:     map[string]ir.TypeDef{},
		building: map[string]bool{},
	}

	doc := &ir.Document{
		Version: ir.VersionV1,
	}
	if initialisms := ir.NormalizeInitialisms(opts.Initialisms); len(initialisms) > 0 {
		doc.Naming = &ir.Naming{Initialisms: initialisms}
	}

	for _, endpoint := range opts.Endpoints {
		parsed, err := collector.parseEndpoint(endpoint)
		if err != nil {
			return nil, err
		}
		doc.Endpoints = append(doc.Endpoints, parsed)
	}
	doc.Endpoints = assignGroups(doc.Endpoints)

	for _, group := range opts.Groups {
		parsed, err := collector.parseGroup(group)
		if err != nil {
			return nil, err
		}
		doc.Groups = append(doc.Groups, parsed)
	}

	for _, name := range collector.order {
		doc.Types = append(doc.Types, collector.defs[name])
	}

	ir.Normalize(doc)
	return doc, nil
}

type typeCollector struct {
	defs     map[string]ir.TypeDef
	building map[string]bool
	order    []string
}

func (c *typeCollector) parseGroup(group *meta.GroupMeta) (ir.Group, error) {
	if group == nil {
		return ir.Group{}, wherr.Errorf("onedef: IR group must not be nil")
	}
	result := ir.Group{
		ID:              group.ID,
		Name:            group.Name,
		PathSegments:    append([]string(nil), group.PathSegments...),
		RequiredHeaders: headerWireNames(group.ProviderRequiredHeaders),
	}
	for _, header := range group.ProviderRequiredHeaders {
		param, err := c.parseHeaderContract(header, header.Name)
		if err != nil {
			return ir.Group{}, err
		}
		result.ProviderHeaders = append(result.ProviderHeaders, param)
	}
	for _, endpoint := range group.Endpoints {
		parsed, err := c.parseEndpoint(endpoint)
		if err != nil {
			return ir.Group{}, err
		}
		result.Endpoints = append(result.Endpoints, parsed)
	}
	for _, child := range group.Children {
		parsed, err := c.parseGroup(child)
		if err != nil {
			return ir.Group{}, err
		}
		result.Groups = append(result.Groups, parsed)
	}
	return result, nil
}

func (c *typeCollector) parseEndpoint(endpoint meta.EndpointStruct) (ir.Endpoint, error) {
	structType := endpoint.StructType
	if structType == nil {
		return ir.Endpoint{}, wherr.Errorf("onedef: endpoint %q has nil struct type", endpoint.StructName)
	}

	result := ir.Endpoint{
		Name:            endpoint.StructName,
		SDKName:         endpoint.SDKName,
		Method:          string(endpoint.Method),
		Path:            endpoint.Path,
		SuccessStatus:   endpoint.SuccessStatus,
		RequiredHeaders: headerWireNames(endpoint.FinalRequiredHeaders),
		Error: ir.Error{
			Body: ir.TypeRef{Kind: ir.TypeKindNamed, Name: ir.BuiltinDefaultError},
		},
	}

	for _, p := range endpoint.Request.PathParameterFields {
		typeRef, err := c.parseTypeRef(p.FieldType, structType.Name()+p.FieldName)
		if err != nil {
			return ir.Endpoint{}, err
		}
		result.Request.PathParams = append(result.Request.PathParams, ir.Parameter{
			Name:     p.FieldName,
			WireName: p.VariableName,
			Type:     typeRef,
			Required: true,
		})
	}

	for _, q := range endpoint.Request.QueryParameterFields {
		typeRef, err := c.parseTypeRef(q.FieldType, structType.Name()+q.FieldName)
		if err != nil {
			return ir.Endpoint{}, err
		}
		result.Request.QueryParams = append(result.Request.QueryParams, ir.Parameter{
			Name:     q.FieldName,
			WireName: q.QueryKey,
			Type:     typeRef,
			Required: false,
		})
	}

	for _, h := range endpoint.Request.HeaderParameterFields {
		if !h.MethodParameter {
			continue
		}
		typeRef, err := c.parseTypeRef(h.FieldType, structType.Name()+h.FieldName)
		if err != nil {
			return ir.Endpoint{}, err
		}
		result.Request.HeaderParams = append(result.Request.HeaderParams, ir.Parameter{
			Name:        h.FieldName,
			WireName:    h.Header.WireName,
			Type:        typeRef,
			Required:    h.Required,
			Description: h.Header.Description,
			Examples:    append([]string(nil), h.Header.Examples...),
		})
	}

	if endpoint.Method == meta.EndpointMethodPost || endpoint.Method == meta.EndpointMethodPut || endpoint.Method == meta.EndpointMethodPatch {
		bodyRef, err := c.parseRequestBody(structType, endpoint.Request)
		if err != nil {
			return ir.Endpoint{}, err
		}
		result.Request.Body = bodyRef
	}

	responseField, ok := structType.FieldByName("Response")
	if !ok {
		return ir.Endpoint{}, wherr.Errorf("struct %q must have a Response field", structType.Name())
	}
	if endpoint.SuccessStatus == 204 && responseField.Type.Kind() == reflect.Struct && responseField.Type.NumField() == 0 {
		result.Response = ir.Response{Envelope: false}
	} else {
		typeRef, err := c.parseTypeRef(responseField.Type, structType.Name()+"Response")
		if err != nil {
			return ir.Endpoint{}, err
		}
		result.Response = ir.Response{
			Envelope: true,
			Body:     &typeRef,
		}
	}

	errorRef, err := c.errorBodyTypeRef(endpoint.ErrorBodyType)
	if err != nil {
		return ir.Endpoint{}, err
	}
	result.Error = ir.Error{Body: errorRef}
	return result, nil
}

func (c *typeCollector) parseHeaderContract(header meta.HeaderContract, nameHint string) (ir.Parameter, error) {
	typeRef, err := c.parseTypeRef(header.Type, nameHint)
	if err != nil {
		return ir.Parameter{}, err
	}
	return ir.Parameter{
		Name:        header.Name,
		WireName:    header.WireName,
		Type:        typeRef,
		Required:    true,
		Description: header.Description,
		Examples:    append([]string(nil), header.Examples...),
	}, nil
}

func headerWireNames(headers []meta.HeaderContract) []string {
	if len(headers) == 0 {
		return nil
	}
	result := make([]string, 0, len(headers))
	for _, header := range headers {
		result = append(result, header.WireName)
	}
	return result
}

func (c *typeCollector) errorBodyTypeRef(t reflect.Type) (ir.TypeRef, error) {
	if t == nil {
		return ir.TypeRef{Kind: ir.TypeKindNamed, Name: ir.BuiltinDefaultError}, nil
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	defaultErrorType := reflect.TypeOf(meta.DefaultError{})
	if t == defaultErrorType {
		return ir.TypeRef{Kind: ir.TypeKindNamed, Name: ir.BuiltinDefaultError}, nil
	}
	if t.Kind() != reflect.Struct || t.Name() == "" {
		return ir.TypeRef{}, wherr.Errorf("onedef: error body type %s must be a named struct", t)
	}
	return c.parseTypeRef(t, t.Name())
}

func (c *typeCollector) parseRequestBody(structType reflect.Type, request meta.RequestField) (*ir.TypeRef, error) {
	requestType := request.FieldType
	include := make([]int, 0, requestType.NumField())
	pathFieldIndexes := make([]int, 0, len(request.PathParameterFields))
	for _, p := range request.PathParameterFields {
		pathFieldIndexes = append(pathFieldIndexes, p.FieldIndex)
	}
	headerFieldIndexes := make([]int, 0, len(request.HeaderParameterFields))
	for _, h := range request.HeaderParameterFields {
		if h.FieldIndex >= 0 {
			headerFieldIndexes = append(headerFieldIndexes, h.FieldIndex)
		}
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
	return &ir.TypeRef{Kind: ir.TypeKindNamed, Name: bodyName}, nil
}

func (c *typeCollector) parseTypeRef(t reflect.Type, nameHint string) (ir.TypeRef, error) {
	nullable := false
	for t.Kind() == reflect.Pointer {
		nullable = true
		t = t.Elem()
	}

	typeRef, err := c.parseNonPointerTypeRef(t, nameHint)
	if err != nil {
		return ir.TypeRef{}, err
	}
	typeRef.Nullable = nullable
	return typeRef, nil
}

func (c *typeCollector) parseNonPointerTypeRef(t reflect.Type, nameHint string) (ir.TypeRef, error) {
	if isUUIDType(t) {
		return ir.TypeRef{Kind: ir.TypeKindUUID}, nil
	}

	switch t.Kind() {
	case reflect.Bool:
		return ir.TypeRef{Kind: ir.TypeKindBool}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return ir.TypeRef{Kind: ir.TypeKindInt}, nil
	case reflect.Float32, reflect.Float64:
		return ir.TypeRef{Kind: ir.TypeKindFloat}, nil
	case reflect.String:
		return ir.TypeRef{Kind: ir.TypeKindString}, nil
	case reflect.Interface:
		return ir.TypeRef{Kind: ir.TypeKindAny}, nil
	case reflect.Slice:
		elem, err := c.parseTypeRef(t.Elem(), nameHint+"Item")
		if err != nil {
			return ir.TypeRef{}, err
		}
		return ir.TypeRef{Kind: ir.TypeKindList, Elem: &elem}, nil
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return ir.TypeRef{}, wherr.Errorf("unsupported map key type %s", t.Key())
		}
		key, err := c.parseTypeRef(t.Key(), nameHint+"Key")
		if err != nil {
			return ir.TypeRef{}, err
		}
		value, err := c.parseTypeRef(t.Elem(), nameHint+"Value")
		if err != nil {
			return ir.TypeRef{}, err
		}
		return ir.TypeRef{
			Kind:  ir.TypeKindMap,
			Key:   &key,
			Value: &value,
		}, nil
	case reflect.Struct:
		name := t.Name()
		if name == "" {
			name = nameHint
		}
		if err := c.ensureStructType(name, t); err != nil {
			return ir.TypeRef{}, err
		}
		return ir.TypeRef{Kind: ir.TypeKindNamed, Name: name}, nil
	default:
		return ir.TypeRef{}, wherr.Errorf("unsupported type %s", t)
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
	c.defs[name] = ir.TypeDef{Name: name, Kind: ir.TypeKindObject}
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
	c.defs[name] = ir.TypeDef{Name: name, Kind: ir.TypeKindObject}
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

func (c *typeCollector) buildTypeDef(name string, t reflect.Type, include []int) (ir.TypeDef, error) {
	def := ir.TypeDef{
		Name: name,
		Kind: ir.TypeKindObject,
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
			return ir.TypeDef{}, err
		}

		def.Fields = append(def.Fields, ir.FieldDef{
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
