package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/textproto"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/failer-dev/onedef/internal/dartgen"
	"github.com/failer-dev/onedef/internal/inspect"
	"github.com/failer-dev/onedef/internal/meta"
)

type App struct {
	mux              *http.ServeMux
	endpoints        []meta.EndpointStruct
	groupedEndpoints []meta.EndpointStruct
	groups           []*meta.GroupMeta
	definitionRoots  []meta.GroupNode
	definitionsReady bool
	dependencyErrors []error
}

type GenerateSDKOptions struct {
	OutDir      string
	PackageName string
	// Initialisms declares identifier words such as ID, URL, OAuth, or JWT in the IR.
	// SDK generators decide how those words map to their target language casing.
	Initialisms []string
}

type GenerateIROptions struct {
	// Initialisms declares identifier words such as ID, URL, OAuth, or JWT in the IR.
	// SDK generators decide how those words map to their target language casing.
	Initialisms []string
}

var generateDartPackage = dartgen.GeneratePackage

func New(definitionRoots ...meta.GroupNode) *App {
	return &App{
		mux:             http.NewServeMux(),
		definitionRoots: append([]meta.GroupNode(nil), definitionRoots...),
	}
}

func (a *App) Register(endpoints ...any) {
	for _, ep := range endpoints {
		switch n := ep.(type) {
		case meta.EndpointsNode:
			for _, endpoint := range n.Endpoints {
				es := a.inspectEndpoint(endpoint, nil, nil)
				a.registerEndpoint(es, false, nil, dependencyRegistry{}, defaultErrorPolicy())
			}
		case meta.Node:
			panic("onedef: Register(endpoints ...any) does not accept group DSL nodes; use App.Group(...) for grouped registration")
		default:
			es := a.inspectEndpoint(ep, nil, nil)
			a.registerEndpoint(es, false, nil, dependencyRegistry{}, defaultErrorPolicy())
		}
	}
}

func (a *App) Group(path string, children ...meta.Node) meta.GroupRef {
	root := meta.GroupNode{Path: path, Children: children, Exposed: false}
	for _, group := range a.registerGroupNode(root, "", nil, nil, nil, nil, dependencyRegistry{}, defaultErrorPolicy()) {
		a.groups = append(a.groups, group)
	}

	return meta.GroupRef{Path: path}
}

func (a *App) GenerateSDK(opts GenerateSDKOptions) error {
	if opts.OutDir == "" {
		return fmt.Errorf("onedef: GenerateSDK OutDir is required")
	}
	if opts.PackageName == "" {
		return fmt.Errorf("onedef: GenerateSDK PackageName is required")
	}

	specJSON, err := a.GenerateIRJSON(GenerateIROptions{
		Initialisms: opts.Initialisms,
	})
	if err != nil {
		return err
	}

	module, err := currentModuleInfo()
	if err != nil {
		return err
	}

	return generateDartPackage(dartgen.GenerateOptions{
		SpecJSON:     specJSON,
		PackageName:  opts.PackageName,
		OutputDir:    opts.OutDir,
		ModuleDir:    module.Dir,
		CorePathBase: filepathJoin("generators", "dart", "packages", "onedef_core"),
	})
}

func (a *App) GenerateIRJSON(opts GenerateIROptions) ([]byte, error) {
	a.finalizeDefinitions()
	return a.buildGroupedSDKSpecJSON(opts.Initialisms)
}

func (a *App) Run(addr string, opts ...RunOption) error {
	a.finalizeDefinitions()

	if len(a.dependencyErrors) > 0 {
		return a.dependencyErrors[0]
	}
	a.printPathLeafWarnings()
	server := a.newHTTPServer(addr, opts...)
	a.printEndpoints(addr)
	return server.ListenAndServe()
}

func (a *App) finalizeDefinitions() {
	if a.definitionsReady {
		return
	}
	a.definitionsReady = true
	if len(a.definitionRoots) == 0 {
		return
	}

	for _, root := range a.definitionRoots {
		root.Exposed = false
		for _, group := range a.registerGroupNode(root, "", nil, nil, nil, nil, dependencyRegistry{}, defaultErrorPolicy()) {
			a.groups = append(a.groups, group)
		}
	}
}

func (a *App) httpHandler(handler meta.HandlerFunc, policy meta.ErrorPolicyBinding) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := handler(w, r); err != nil {
			status, body := policy.MapError(r, err)
			writeJSON(w, status, body)
		}
	})
}

func (a *App) printEndpoints(addr string) {
	fmt.Println("onedef server starting on", addr)
	fmt.Println()
	for _, es := range a.endpoints {
		fmt.Printf("  %-8s %s  (%s)\n", es.Method, es.Path, es.StructName)
	}
	fmt.Println()
}

func (a *App) printPathLeafWarnings() {
	for _, es := range a.groupedEndpoints {
		warning, ok := pathLeafWarning(es)
		if ok {
			fmt.Fprintln(os.Stderr, warning)
		}
	}
}

func (a *App) inspectEndpoint(endpoint any, fullPath *string, cfg *endpointGroupConfig) meta.EndpointStruct {
	structType := reflect.TypeOf(endpoint)
	if structType == nil {
		panic("onedef: endpoint must not be nil")
	}
	if structType.Kind() == reflect.Pointer {
		structType = structType.Elem()
	}

	method, path, pathParams, successStatus, err := inspect.InspectEndpointMethodMarker(structType)
	if err != nil {
		panic(err)
	}

	request, err := inspect.InspectRequest(structType, method, pathParams)
	if err != nil {
		panic(err)
	}

	dependencies, err := inspect.InspectDependencies(structType)
	if err != nil {
		panic(err)
	}

	handlerType := reflect.TypeFor[meta.Handler]()
	ptrType := reflect.PointerTo(structType)
	if !ptrType.Implements(handlerType) {
		panic("onedef: " + structType.Name() + " must implement Handle(context.Context) error")
	}
	responseField, ok := structType.FieldByName("Response")
	if !ok {
		panic("onedef: " + structType.Name() + " must have a Response field")
	}
	if successStatus == http.StatusNoContent {
		if responseField.Type.Kind() != reflect.Struct || responseField.Type.NumField() != 0 {
			panic(`onedef: ` + structType.Name() + ` with status "204" must have Response of type struct{}`)
		}
	}

	resolvedPath := path
	es := meta.EndpointStruct{
		StructName:          structType.Name(),
		StructPkgPath:       structType.PkgPath(),
		StructQualifiedName: qualifiedStructName(structType),
		Method:              method,
		Path:                resolvedPath,
		LeafPath:            path,
		SuccessStatus:       successStatus,
		Request:             request,
		Dependencies:        dependencies,
		StructType:          structType,
	}

	groupHeaders := []string(nil)
	if cfg != nil {
		if path != "" && !strings.HasPrefix(path, "/") {
			panic(fmt.Errorf("onedef: endpoint leaf path %q must be empty or start with '/'", path))
		}
		resolvedPath = *fullPath
		es.Path = resolvedPath
		es.GroupPath = append([]string(nil), cfg.groupPath...)
		groupHeaders = append([]string(nil), cfg.finalHeaders...)
		es.InheritedRequiredHeaders = append([]string(nil), cfg.finalHeaders...)
	}

	finalHeaders := append([]string(nil), groupHeaders...)
	seenGroupHeaders := make(map[string]string, len(groupHeaders))
	for _, header := range groupHeaders {
		seenGroupHeaders[normalizeHeaderName(header)] = header
	}
	seenEndpointHeaders := make(map[string]string, len(request.HeaderParameterFields))
	for _, h := range request.HeaderParameterFields {
		es.EndpointRequiredHeaders = append(es.EndpointRequiredHeaders, h.HeaderName)

		normalized := normalizeHeaderName(h.HeaderName)
		if existing, ok := seenGroupHeaders[normalized]; ok {
			panic(fmt.Errorf("onedef: duplicate required header %q on endpoint %q; already required by group as %q", h.HeaderName, es.StructName, existing))
		}
		if existing, ok := seenEndpointHeaders[normalized]; ok {
			panic(fmt.Errorf("onedef: duplicate endpoint header %q on %q; already declared as %q", h.HeaderName, es.StructName, existing))
		}
		seenEndpointHeaders[normalized] = h.HeaderName
		if h.Required {
			finalHeaders = append(finalHeaders, h.HeaderName)
		}
	}
	es.FinalRequiredHeaders = finalHeaders
	return es
}

func (a *App) registerEndpoint(
	es meta.EndpointStruct,
	grouped bool,
	middlewares []middlewareEntry,
	dependencies dependencyRegistry,
	errorPolicy meta.ErrorPolicyBinding,
) {
	meta.MustErrorPolicy(errorPolicy)
	es.ErrorBodyType = errorPolicy.ErrorBodyType()
	a.endpoints = append(a.endpoints, es)
	if grouped {
		a.groupedEndpoints = append(a.groupedEndpoints, es)
	}
	resolvedDependencies, err := dependencies.snapshotDependencies(es)
	if err != nil {
		a.dependencyErrors = append(a.dependencyErrors, err)
	}
	handler := applyMiddlewareEntries(MakeHandlerFunc(es, resolvedDependencies), middlewares)
	a.mux.Handle(string(es.Method)+" "+es.Path, a.httpHandler(handler, errorPolicy))
}

type endpointGroupConfig struct {
	groupPath    []string
	finalHeaders []string
}

func (a *App) registerGroupNode(
	node meta.GroupNode,
	parentPrefix string,
	inheritedHeaders []string,
	visiblePath []string,
	boundHeaders []string,
	inheritedMiddlewares []middlewareEntry,
	inheritedDependencies dependencyRegistry,
	inheritedErrorPolicy meta.ErrorPolicyBinding,
) []*meta.GroupMeta {
	if !strings.HasPrefix(node.Path, "/") {
		panic(fmt.Errorf("onedef: group path %q must start with '/'", node.Path))
	}

	fullPrefix := joinPaths(parentPrefix, node.Path)
	localHeaders := make([]string, 0)
	seenHeaders := make(map[string]string, len(inheritedHeaders))
	for _, header := range inheritedHeaders {
		seenHeaders[normalizeHeaderName(header)] = header
	}
	omitAuthorization := false
	for _, child := range node.Children {
		switch n := child.MetaNode().(type) {
		case meta.RequireHeaderNode:
			key := normalizeHeaderName(n.Name)
			if existing, ok := seenHeaders[key]; ok {
				panic(fmt.Errorf("onedef: duplicate required header %q; already declared as %q", n.Name, existing))
			}
			seenHeaders[key] = n.Name
			localHeaders = append(localHeaders, n.Name)
		case meta.OmitHeaderNode:
			if normalizeHeaderName(n.Name) != normalizeHeaderName("Authorization") {
				panic(fmt.Errorf("onedef: OmitHeader(%q) is not supported; only OmitHeader(\"Authorization\") is allowed", n.Name))
			}
			if _, ok := seenHeaders[normalizeHeaderName("Authorization")]; !ok {
				panic(fmt.Errorf("onedef: OmitHeader(\"Authorization\") used but \"Authorization\" is not required by any parent group"))
			}
			omitAuthorization = true
		case meta.SkipMiddlewareNode:
			// Middleware declarations are handled below in declaration order.
		case meta.UseNode:
			// Middleware declarations are handled below in declaration order.
		}
	}

	finalHeaders := make([]string, 0, len(inheritedHeaders)+len(localHeaders))
	for _, header := range inheritedHeaders {
		if omitAuthorization && normalizeHeaderName(header) == normalizeHeaderName("Authorization") {
			continue
		}
		finalHeaders = append(finalHeaders, header)
	}
	finalHeaders = append(finalHeaders, localHeaders...)
	nextVisiblePath := append([]string(nil), visiblePath...)
	if node.Exposed {
		nextVisiblePath = append(nextVisiblePath, lastPathSegment(node.Path))
	}
	providerHeaders := subtractHeaders(finalHeaders, boundHeaders)
	nextBoundHeaders := append([]string(nil), boundHeaders...)
	if node.Exposed {
		nextBoundHeaders = append([]string(nil), finalHeaders...)
	}

	finalMiddlewares := cloneMiddlewareEntries(inheritedMiddlewares)
	for _, child := range node.Children {
		switch n := child.MetaNode().(type) {
		case meta.SkipMiddlewareNode:
			finalMiddlewares = skipMiddlewareEntries(finalMiddlewares, n.Names)
		case meta.UseNode:
			finalMiddlewares = appendMiddlewareEntries(finalMiddlewares, n.Middlewares)
		}
	}

	finalDependencies := inheritedDependencies.clone()
	localDependencyTypes := make(map[reflect.Type]struct{})
	for _, child := range node.Children {
		if binding, ok := child.MetaNode().(meta.DependencyBinding); ok {
			if err := finalDependencies.addScopedBinding(binding, localDependencyTypes); err != nil {
				panic(err)
			}
		}
	}

	finalErrorPolicy := inheritedErrorPolicy
	localErrorPolicyDeclared := false
	for _, child := range node.Children {
		if policy, ok := child.MetaNode().(meta.ErrorPolicyBinding); ok {
			if localErrorPolicyDeclared {
				panic("onedef: error policy already declared in this group")
			}
			meta.MustErrorPolicy(policy)
			localErrorPolicyDeclared = true
			finalErrorPolicy = policy
		}
	}

	var groupMeta *meta.GroupMeta
	if node.Exposed {
		groupMeta = &meta.GroupMeta{
			ID:                      strings.Join(nextVisiblePath, "."),
			Name:                    lastPathSegment(node.Path),
			PathPrefix:              fullPrefix,
			PathSegments:            append([]string(nil), nextVisiblePath...),
			LocalRequiredHeaders:    append([]string(nil), localHeaders...),
			ProviderRequiredHeaders: append([]string(nil), providerHeaders...),
			FinalRequiredHeaders:    append([]string(nil), finalHeaders...),
		}
	}

	result := make([]*meta.GroupMeta, 0)
	for _, child := range node.Children {
		switch n := child.MetaNode().(type) {
		case meta.EndpointNode:
			fullPath := joinPaths(fullPrefix, endpointLeafPath(n.Endpoint))
			es := a.inspectEndpoint(n.Endpoint, &fullPath, &endpointGroupConfig{
				groupPath:    nextVisiblePath,
				finalHeaders: finalHeaders,
			})
			es.SDKName = n.SDKName
			endpointMiddlewares := skipMiddlewareEntries(finalMiddlewares, n.SkipMiddlewareNames)
			endpointMiddlewares = appendMiddlewareEntries(endpointMiddlewares, n.Middlewares)
			endpointDependencies := finalDependencies.clone()
			localEndpointDependencyTypes := make(map[reflect.Type]struct{})
			for _, binding := range n.Dependencies {
				if err := endpointDependencies.addScopedBinding(binding, localEndpointDependencyTypes); err != nil {
					panic(err)
				}
			}
			endpointErrorPolicy := finalErrorPolicy
			if n.ErrorPolicy != nil {
				endpointErrorPolicy = n.ErrorPolicy
			}
			a.registerEndpoint(es, true, endpointMiddlewares, endpointDependencies, endpointErrorPolicy)
			es.ErrorBodyType = endpointErrorPolicy.ErrorBodyType()
			if groupMeta != nil {
				groupMeta.Endpoints = append(groupMeta.Endpoints, es)
			}
		case meta.EndpointsNode:
			for _, endpoint := range n.Endpoints {
				fullPath := joinPaths(fullPrefix, endpointLeafPath(endpoint))
				es := a.inspectEndpoint(endpoint, &fullPath, &endpointGroupConfig{
					groupPath:    nextVisiblePath,
					finalHeaders: finalHeaders,
				})
				a.registerEndpoint(es, true, finalMiddlewares, finalDependencies, finalErrorPolicy)
				es.ErrorBodyType = finalErrorPolicy.ErrorBodyType()
				if groupMeta != nil {
					groupMeta.Endpoints = append(groupMeta.Endpoints, es)
				}
			}
		case meta.GroupNode:
			children := a.registerGroupNode(n, fullPrefix, finalHeaders, nextVisiblePath, nextBoundHeaders, finalMiddlewares, finalDependencies, finalErrorPolicy)
			if groupMeta != nil {
				groupMeta.Children = append(groupMeta.Children, children...)
			} else {
				result = append(result, children...)
			}
		}
	}

	if groupMeta != nil {
		return append(result, groupMeta)
	}
	return result
}

func endpointLeafPath(endpoint any) string {
	structType := reflect.TypeOf(endpoint)
	if structType.Kind() == reflect.Pointer {
		structType = structType.Elem()
	}
	_, path, _, _, err := inspect.InspectEndpointMethodMarker(structType)
	if err != nil {
		panic(err)
	}
	return path
}

func joinPaths(parts ...string) string {
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.Trim(part, "/")
		if trimmed == "" {
			continue
		}
		segments = append(segments, trimmed)
	}
	if len(segments) == 0 {
		return "/"
	}
	return "/" + strings.Join(segments, "/")
}

func lastPathSegment(path string) string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return "root"
	}
	parts := strings.Split(trimmed, "/")
	return parts[len(parts)-1]
}

func pathLeafWarning(es meta.EndpointStruct) (string, bool) {
	if len(es.GroupPath) == 0 {
		return "", false
	}
	groupSegment := es.GroupPath[len(es.GroupPath)-1]
	leafSegment, ok := firstStaticPathSegment(es.LeafPath)
	if !ok || !similarPathSegments(groupSegment, leafSegment) {
		return "", false
	}
	return fmt.Sprintf(
		"onedef warning: endpoint %s under group %q uses leaf path %q; full path becomes %q. Did you mean path:\"\"?",
		es.StructName,
		groupSegment,
		es.LeafPath,
		es.Path,
	), true
}

func firstStaticPathSegment(path string) (string, bool) {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return "", false
	}
	segment := strings.Split(trimmed, "/")[0]
	if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
		return "", false
	}
	return segment, true
}

func similarPathSegments(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if a == "" || b == "" {
		return false
	}
	return a == b || singularizePathSegment(a) == singularizePathSegment(b)
}

func singularizePathSegment(segment string) string {
	switch {
	case strings.HasSuffix(segment, "ies") && len(segment) > 3:
		return segment[:len(segment)-3] + "y"
	case strings.HasSuffix(segment, "s") && !strings.HasSuffix(segment, "ss") && len(segment) > 1:
		return segment[:len(segment)-1]
	default:
		return segment
	}
}

func normalizeHeaderName(name string) string {
	return textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(name))
}

func subtractHeaders(headers []string, subtract []string) []string {
	if len(headers) == 0 {
		return nil
	}
	if len(subtract) == 0 {
		return append([]string(nil), headers...)
	}

	blocked := make(map[string]struct{}, len(subtract))
	for _, header := range subtract {
		blocked[normalizeHeaderName(header)] = struct{}{}
	}

	result := make([]string, 0, len(headers))
	for _, header := range headers {
		if _, ok := blocked[normalizeHeaderName(header)]; ok {
			continue
		}
		result = append(result, header)
	}
	return result
}

func qualifiedStructName(structType reflect.Type) string {
	if structType.PkgPath() == "" {
		return structType.Name()
	}
	return structType.PkgPath() + "." + structType.Name()
}

func filepathJoin(parts ...string) string {
	return strings.Join(parts, "/")
}

type sdkSpec struct {
	Version   string        `json:"version"`
	Naming    *sdkNaming    `json:"naming,omitempty"`
	Endpoints []sdkEndpoint `json:"endpoints,omitempty"`
	Groups    []sdkGroup    `json:"groups,omitempty"`
	Types     []sdkTypeDef  `json:"types"`
}

type sdkNaming struct {
	Initialisms []string `json:"initialisms,omitempty"`
}

type sdkGroup struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	PathSegments    []string      `json:"pathSegments,omitempty"`
	RequiredHeaders []string      `json:"requiredHeaders,omitempty"`
	Endpoints       []sdkEndpoint `json:"endpoints,omitempty"`
	Groups          []sdkGroup    `json:"groups,omitempty"`
}

type sdkEndpoint struct {
	Name            string          `json:"name"`
	SDKName         string          `json:"sdkName,omitempty"`
	Method          string          `json:"method"`
	Path            string          `json:"path"`
	SuccessStatus   int             `json:"successStatus"`
	Group           string          `json:"group,omitempty"`
	RequiredHeaders []string        `json:"requiredHeaders,omitempty"`
	Request         sdkRequestSpec  `json:"request"`
	Response        sdkResponseSpec `json:"response"`
	Error           sdkErrorSpec    `json:"error"`
}

type sdkRequestSpec struct {
	PathParams   []sdkParameterSpec `json:"pathParams,omitempty"`
	QueryParams  []sdkParameterSpec `json:"queryParams,omitempty"`
	HeaderParams []sdkParameterSpec `json:"headerParams,omitempty"`
	Body         *sdkTypeRef        `json:"body,omitempty"`
}

type sdkResponseSpec struct {
	Envelope bool        `json:"envelope"`
	Body     *sdkTypeRef `json:"body,omitempty"`
}

type sdkErrorSpec struct {
	Body sdkTypeRef `json:"body"`
}

type sdkParameterSpec struct {
	Name     string     `json:"name"`
	WireName string     `json:"wireName"`
	Type     sdkTypeRef `json:"type"`
	Required bool       `json:"required"`
}

type sdkTypeDef struct {
	Name   string        `json:"name"`
	Kind   string        `json:"kind"`
	Fields []sdkFieldDef `json:"fields,omitempty"`
}

type sdkFieldDef struct {
	Name      string     `json:"name"`
	WireName  string     `json:"wireName"`
	Type      sdkTypeRef `json:"type"`
	Required  bool       `json:"required"`
	Nullable  bool       `json:"nullable,omitempty"`
	OmitEmpty bool       `json:"omitEmpty,omitempty"`
}

type sdkTypeRef struct {
	Kind     string      `json:"kind"`
	Name     string      `json:"name,omitempty"`
	Nullable bool        `json:"nullable,omitempty"`
	Elem     *sdkTypeRef `json:"elem,omitempty"`
	Key      *sdkTypeRef `json:"key,omitempty"`
	Value    *sdkTypeRef `json:"value,omitempty"`
}

type parsedIRSpec struct {
	Version   string             `json:"version"`
	Naming    *sdkNaming         `json:"naming,omitempty"`
	Endpoints []parsedIREndpoint `json:"endpoints"`
	Types     []sdkTypeDef       `json:"types"`
}

type parsedIREndpoint struct {
	ID       string      `json:"id"`
	Endpoint sdkEndpoint `json:"endpoint"`
}

func (a *App) buildGroupedSDKSpecJSON(initialisms ...[]string) ([]byte, error) {
	namingInitialisms := []string(nil)
	if len(initialisms) > 0 {
		namingInitialisms = initialisms[0]
	}
	parsedJSON, err := a.generateGroupedIRJSON(namingInitialisms)
	if err != nil {
		return nil, err
	}

	var parsed parsedIRSpec
	if err := json.Unmarshal(parsedJSON, &parsed); err != nil {
		return nil, fmt.Errorf("onedef: failed to decode grouped IR JSON: %w", err)
	}

	parsedByID := make(map[string]sdkEndpoint, len(parsed.Endpoints))
	for _, endpoint := range parsed.Endpoints {
		parsedByID[endpoint.ID] = endpoint.Endpoint
	}

	groups, err := buildSDKGroups(a.groups, parsedByID)
	if err != nil {
		return nil, err
	}

	spec := sdkSpec{
		Version: "v1",
		Naming:  parsed.Naming,
		Groups:  groups,
		Types:   parsed.Types,
	}

	encoded, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("onedef: failed to encode grouped sdk spec: %w", err)
	}
	return encoded, nil
}

func (a *App) generateGroupedIRJSON(initialisms []string) ([]byte, error) {
	module, err := currentModuleInfo()
	if err != nil {
		return nil, err
	}
	irImportPath := onedefIRImportPath()

	helperDir, err := os.MkdirTemp(module.Dir, ".onedef-grouped-ir-*")
	if err != nil {
		return nil, fmt.Errorf("onedef: failed to create temp grouped helper dir: %w", err)
	}
	defer os.RemoveAll(helperDir)

	imports := make(map[string]string)
	constructors := make([]string, 0, len(a.groupedEndpoints))
	endpointIDs := make([]string, 0, len(a.groupedEndpoints))
	for _, endpoint := range a.groupedEndpoints {
		pkgPath := endpoint.StructType.PkgPath()
		alias, ok := imports[pkgPath]
		if !ok {
			alias = fmt.Sprintf("endpointpkg%d", len(imports))
			imports[pkgPath] = alias
		}
		constructors = append(constructors, fmt.Sprintf("&%s.%s{}", alias, endpoint.StructType.Name()))
		endpointIDs = append(endpointIDs, endpoint.StructQualifiedName)
	}
	errorTypeConstructors := make([]string, 0)
	seenErrorTypes := make(map[reflect.Type]struct{})
	defaultErrorType := reflect.TypeOf(meta.DefaultError{})
	for _, endpoint := range a.groupedEndpoints {
		errorType := endpoint.ErrorBodyType
		if errorType == nil || errorType == defaultErrorType {
			continue
		}
		for errorType.Kind() == reflect.Pointer {
			errorType = errorType.Elem()
		}
		if _, ok := seenErrorTypes[errorType]; ok {
			continue
		}
		seenErrorTypes[errorType] = struct{}{}
		pkgPath := errorType.PkgPath()
		alias, ok := imports[pkgPath]
		if !ok {
			alias = fmt.Sprintf("endpointpkg%d", len(imports))
			imports[pkgPath] = alias
		}
		errorTypeConstructors = append(errorTypeConstructors, fmt.Sprintf("%s.%s{}", alias, errorType.Name()))
	}

	var helper bytes.Buffer
	helper.WriteString("package main\n\nimport (\n")
	helper.WriteString("  \"encoding/json\"\n")
	helper.WriteString("  \"fmt\"\n")
	helper.WriteString("  \"os\"\n")
	helper.WriteString(fmt.Sprintf("  onedefir \"%s\"\n", irImportPath))
	for pkgPath, alias := range imports {
		helper.WriteString(fmt.Sprintf("  %s \"%s\"\n", alias, pkgPath))
	}
	helper.WriteString(")\n\ntype parsedEndpoint struct {\n")
	helper.WriteString("  ID string `json:\"id\"`\n")
	helper.WriteString("  Endpoint onedefir.Endpoint `json:\"endpoint\"`\n")
	helper.WriteString("}\n\n")
	helper.WriteString("type parsedSpec struct {\n")
	helper.WriteString("  Version string `json:\"version\"`\n")
	helper.WriteString("  Naming *onedefir.Naming `json:\"naming,omitempty\"`\n")
	helper.WriteString("  Endpoints []parsedEndpoint `json:\"endpoints\"`\n")
	helper.WriteString("  Types []onedefir.TypeDef `json:\"types\"`\n")
	helper.WriteString("}\n\nfunc main() {\n")
	helper.WriteString("  spec, err := onedefir.ParseWithOptions(onedefir.ParseOptions{\n")
	helper.WriteString("    Initialisms: []string{\n")
	for _, initialism := range initialisms {
		helper.WriteString(fmt.Sprintf("      %q,\n", initialism))
	}
	helper.WriteString("    },\n")
	if len(errorTypeConstructors) > 0 {
		helper.WriteString("    ExtraTypes: []any{\n")
		for _, constructor := range errorTypeConstructors {
			helper.WriteString(fmt.Sprintf("      %s,\n", constructor))
		}
		helper.WriteString("    },\n")
	}
	helper.WriteString("  },\n")
	for _, constructor := range constructors {
		helper.WriteString(fmt.Sprintf("    %s,\n", constructor))
	}
	helper.WriteString("  )\n")
	helper.WriteString("  if err != nil {\n")
	helper.WriteString("    fmt.Fprintln(os.Stderr, err)\n")
	helper.WriteString("    os.Exit(1)\n")
	helper.WriteString("  }\n")
	helper.WriteString("  endpointIDs := []string{\n")
	for _, endpointID := range endpointIDs {
		helper.WriteString(fmt.Sprintf("    %q,\n", endpointID))
	}
	helper.WriteString("  }\n")
	helper.WriteString("  if len(spec.Endpoints) != len(endpointIDs) {\n")
	helper.WriteString("    fmt.Fprintln(os.Stderr, \"onedef: grouped IR helper produced mismatched endpoint count\")\n")
	helper.WriteString("    os.Exit(1)\n")
	helper.WriteString("  }\n")
	helper.WriteString("  out := parsedSpec{Version: spec.Version, Naming: spec.Naming, Types: spec.Types}\n")
	helper.WriteString("  for i, endpoint := range spec.Endpoints {\n")
	helper.WriteString("    out.Endpoints = append(out.Endpoints, parsedEndpoint{ID: endpointIDs[i], Endpoint: endpoint})\n")
	helper.WriteString("  }\n")
	helper.WriteString("  encoder := json.NewEncoder(os.Stdout)\n")
	helper.WriteString("  encoder.SetIndent(\"\", \"  \")\n")
	helper.WriteString("  if err := encoder.Encode(out); err != nil {\n")
	helper.WriteString("    fmt.Fprintln(os.Stderr, err)\n")
	helper.WriteString("    os.Exit(1)\n")
	helper.WriteString("  }\n")
	helper.WriteString("}\n")

	helperPath := filepath.Join(helperDir, "main.go")
	if err := os.WriteFile(helperPath, helper.Bytes(), 0o600); err != nil {
		return nil, fmt.Errorf("onedef: failed to write grouped helper: %w", err)
	}

	cmd := exec.Command("go", "run", helperPath)
	cmd.Dir = module.Dir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("onedef: failed to generate grouped IR JSON: %s", msg)
	}

	return stdout.Bytes(), nil
}

func onedefIRImportPath() string {
	pkgPath := reflect.TypeOf(App{}).PkgPath()
	const suffix = "/internal/app"
	if strings.HasSuffix(pkgPath, suffix) {
		return strings.TrimSuffix(pkgPath, suffix) + "/ir"
	}
	return "github.com/failer-dev/onedef/ir"
}

func buildSDKGroups(groups []*meta.GroupMeta, parsedByID map[string]sdkEndpoint) ([]sdkGroup, error) {
	result := make([]sdkGroup, 0, len(groups))
	for _, group := range groups {
		entry := sdkGroup{
			ID:              group.ID,
			Name:            group.Name,
			PathSegments:    append([]string(nil), group.PathSegments...),
			RequiredHeaders: append([]string(nil), group.ProviderRequiredHeaders...),
		}
		for _, endpoint := range group.Endpoints {
			converted, err := buildSDKEndpoint(endpoint, parsedByID)
			if err != nil {
				return nil, err
			}
			entry.Endpoints = append(entry.Endpoints, converted)
		}
		children, err := buildSDKGroups(group.Children, parsedByID)
		if err != nil {
			return nil, err
		}
		entry.Groups = children
		result = append(result, entry)
	}
	return result, nil
}

func buildSDKEndpoint(endpoint meta.EndpointStruct, parsedByID map[string]sdkEndpoint) (sdkEndpoint, error) {
	parsed, ok := parsedByID[endpoint.StructQualifiedName]
	if !ok {
		return sdkEndpoint{}, fmt.Errorf("onedef: parsed endpoint %q not found for grouped sdk generation", endpoint.StructQualifiedName)
	}
	sdkName := parsed.SDKName
	if endpoint.SDKName != "" {
		sdkName = endpoint.SDKName
	}
	return sdkEndpoint{
		Name:            parsed.Name,
		SDKName:         sdkName,
		Method:          parsed.Method,
		Path:            endpoint.Path,
		SuccessStatus:   parsed.SuccessStatus,
		RequiredHeaders: append([]string(nil), endpoint.FinalRequiredHeaders...),
		Request:         parsed.Request,
		Response:        parsed.Response,
		Error:           sdkErrorSpec{Body: sdkTypeRefForReflectType(endpoint.ErrorBodyType)},
	}, nil
}

func sdkTypeRefForReflectType(t reflect.Type) sdkTypeRef {
	if t == nil {
		return sdkTypeRef{Kind: "named", Name: "DefaultError"}
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return sdkTypeRef{Kind: "named", Name: t.Name()}
}
