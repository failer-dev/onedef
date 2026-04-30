package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/textproto"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/failer-dev/onedef/onedef_go/internal/inspect"
	"github.com/failer-dev/onedef/onedef_go/internal/irbuild"
	"github.com/failer-dev/onedef/onedef_go/internal/meta"
)

type App struct {
	mux              *http.ServeMux
	endpoints        []meta.EndpointStruct
	groupedEndpoints []meta.EndpointStruct
	groups           []*meta.GroupMeta
	routeHeaders     []meta.HeaderContract
	definitionRoots  []meta.GroupNode
	definitionsReady bool
}

type GenerateIROptions struct {
	// Initialisms declares identifier words such as ID, URL, OAuth, or JWT in the IR.
	// SDK generators decide how those words map to their target language casing.
	Initialisms []string
}

func New(definitionRoots ...meta.GroupNode) *App {
	return &App{
		mux:             http.NewServeMux(),
		definitionRoots: append([]meta.GroupNode(nil), definitionRoots...),
	}
}

func (a *App) GenerateIRJSON(opts GenerateIROptions) ([]byte, error) {
	a.finalizeDefinitions()
	return a.buildIRJSON(opts.Initialisms)
}

func (a *App) Run(addr string, opts ...RunOption) error {
	a.finalizeDefinitions()
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
		for _, group := range a.registerGroupNode(root, "", nil, nil, nil, nil, nil, nil, provideRegistry{}, defaultErrorPolicy()) {
			a.groups = append(a.groups, group)
		}
	}
}

func (a *App) httpHandler(handler meta.HandlerFunc, policy meta.ErrorPolicyBinding, endpoint meta.EndpointStruct) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		tracker := &statusTrackingResponseWriter{ResponseWriter: w}
		err := handler(tracker, r)
		if err != nil {
			status, body := policy.MapError(r, err)
			writeJSON(tracker, status, body)
		}
		outcome := meta.Outcome{
			Method:   string(endpoint.Method),
			Path:     endpoint.Path,
			Endpoint: endpoint.StructName,
			Status:   tracker.statusOrDefault(),
			Duration: time.Since(started),
			Error:    err,
		}
		for _, observer := range endpoint.Observers {
			observer.Observe(r.Context(), outcome)
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

	provide, err := inspect.InspectProvide(structType)
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
		Provide:             provide,
		StructType:          structType,
	}

	groupHeaders := []meta.HeaderContract(nil)
	endpointHeaders := []meta.HeaderContract(nil)
	beforeHandlers := []any(nil)
	afterHandlers := []any(nil)
	observers := []meta.Observer(nil)
	if cfg != nil {
		if path != "" && !strings.HasPrefix(path, "/") {
			panic(fmt.Errorf("onedef: endpoint leaf path %q must be empty or start with '/'", path))
		}
		resolvedPath = *fullPath
		es.Path = resolvedPath
		es.GroupPath = append([]string(nil), cfg.groupPath...)
		groupHeaders = cloneHeaderContracts(cfg.finalHeaders)
		endpointHeaders = cloneHeaderContracts(cfg.endpointHeaders)
		beforeHandlers = append([]any(nil), cfg.beforeHandlers...)
		afterHandlers = append([]any(nil), cfg.afterHandlers...)
		observers = append([]meta.Observer(nil), cfg.observers...)
		es.InheritedRequiredHeaders = cloneHeaderContracts(cfg.finalHeaders)
	}

	finalHeaders := cloneHeaderContracts(groupHeaders)
	seenGroupHeaders := headerContractMap(groupHeaders)

	seenEndpointHeaders := make(map[string]meta.HeaderContract, len(endpointHeaders))
	for _, header := range endpointHeaders {
		normalized := normalizeHeaderName(header.WireName)
		if header.WireName == "" || normalized == "" {
			panic(fmt.Errorf("onedef: required header on endpoint %q must not be empty", es.StructName))
		}
		if existing, ok := seenEndpointHeaders[normalized]; ok {
			panic(fmt.Errorf("onedef: duplicate required header %q on endpoint %q; already declared as %q", header.WireName, es.StructName, existing.WireName))
		}
		if existing, ok := seenGroupHeaders[normalized]; ok {
			panic(fmt.Errorf("onedef: duplicate required header %q on endpoint %q; already required by group as %q", header.WireName, es.StructName, existing.WireName))
		}
		seenEndpointHeaders[normalized] = header
		es.EndpointRequiredHeaders = append(es.EndpointRequiredHeaders, header)
		finalHeaders = append(finalHeaders, header)
	}

	finalHeaderMap := headerContractMap(finalHeaders)
	seenStructHeaders := make(map[string]string, len(request.HeaderParameterFields))
	boundEndpointHeaders := make(map[string]struct{}, len(request.HeaderParameterFields))
	for i := range request.HeaderParameterFields {
		h := request.HeaderParameterFields[i]
		normalized := normalizeHeaderName(h.Header.WireName)
		if existing, ok := seenStructHeaders[normalized]; ok {
			panic(fmt.Errorf("onedef: duplicate header binding %q on endpoint %q; already bound by %s", h.Header.WireName, es.StructName, existing))
		}
		seenStructHeaders[normalized] = h.FieldName

		header, ok := finalHeaderMap[normalized]
		if !ok {
			panic(fmt.Errorf("onedef: header %q on endpoint %q Request.%s must be declared by RequireHeader in the endpoint or an ancestor group", h.Header.WireName, es.StructName, h.FieldName))
		}
		if err := validateHeaderBinding(header, h.FieldType); err != nil {
			panic(fmt.Errorf("onedef: header %q on endpoint %q Request.%s: %w", h.Header.WireName, es.StructName, h.FieldName, err))
		}

		request.HeaderParameterFields[i].Header = header
		request.HeaderParameterFields[i].Required = true
		if _, ok := seenEndpointHeaders[normalized]; ok {
			request.HeaderParameterFields[i].MethodParameter = true
			boundEndpointHeaders[normalized] = struct{}{}
		}
	}

	for _, header := range endpointHeaders {
		normalized := normalizeHeaderName(header.WireName)
		if _, ok := boundEndpointHeaders[normalized]; ok {
			continue
		}
		request.HeaderParameterFields = append(request.HeaderParameterFields, meta.HeaderParameterField{
			FieldName:       header.Name,
			FieldIndex:      -1,
			FieldType:       header.Type,
			Header:          header,
			Required:        true,
			MethodParameter: true,
		})
	}

	beforeStructs := make([]meta.BeforeHandleStruct, 0, len(beforeHandlers))
	for _, handler := range beforeHandlers {
		beforeStruct, err := inspect.InspectBeforeHandle(handler, pathParams, finalHeaders)
		if err != nil {
			panic(err)
		}
		beforeStructs = append(beforeStructs, beforeStruct)
	}

	afterStructs := make([]meta.AfterHandleStruct, 0, len(afterHandlers))
	for _, handler := range afterHandlers {
		afterStruct, err := inspect.InspectAfterHandle(handler, pathParams, finalHeaders, responseField.Type)
		if err != nil {
			panic(err)
		}
		afterStructs = append(afterStructs, afterStruct)
	}

	es.Request = request
	es.BeforeHandlers = beforeStructs
	es.AfterHandlers = afterStructs
	es.Observers = observers
	es.FinalRequiredHeaders = finalHeaders
	return es
}

func validateHeaderBinding(header meta.HeaderContract, fieldType reflect.Type) error {
	value := reflect.Zero(header.Type)
	_, err := meta.AssignHeaderValue(value, fieldType)
	return err
}

func (a *App) registerEndpoint(
	es meta.EndpointStruct,
	grouped bool,
	provides provideRegistry,
	errorPolicy meta.ErrorPolicyBinding,
) {
	meta.MustErrorPolicy(errorPolicy)
	es.ErrorBodyType = errorPolicy.ErrorBodyType()
	a.endpoints = append(a.endpoints, es)
	if grouped {
		a.groupedEndpoints = append(a.groupedEndpoints, es)
	}
	handler := MakeHandlerFunc(es, provides)
	a.mux.Handle(string(es.Method)+" "+es.Path, a.httpHandler(handler, errorPolicy, es))
}

type endpointGroupConfig struct {
	groupPath       []string
	finalHeaders    []meta.HeaderContract
	endpointHeaders []meta.HeaderContract
	beforeHandlers  []any
	afterHandlers   []any
	observers       []meta.Observer
}

func (a *App) registerGroupNode(
	node meta.GroupNode,
	parentPrefix string,
	inheritedHeaders []meta.HeaderContract,
	visiblePath []string,
	boundHeaders []meta.HeaderContract,
	inheritedBeforeHandlers []any,
	inheritedAfterHandlers []any,
	inheritedObservers []meta.Observer,
	inheritedProvides provideRegistry,
	inheritedErrorPolicy meta.ErrorPolicyBinding,
) []*meta.GroupMeta {
	if !strings.HasPrefix(node.Path, "/") {
		panic(fmt.Errorf("onedef: group path %q must start with '/'", node.Path))
	}

	fullPrefix := joinPaths(parentPrefix, node.Path)
	finalHeaders := cloneHeaderContracts(inheritedHeaders)
	localHeaders := make([]meta.HeaderContract, 0)
	localHeaderKeys := make(map[string]meta.HeaderContract)
	for _, child := range node.Children {
		switch n := child.MetaNode().(type) {
		case meta.RequireHeaderNode:
			header := n.Header
			key := normalizeHeaderName(header.WireName)
			if header.WireName == "" || key == "" {
				panic(fmt.Errorf("onedef: required header in group %q must not be empty", fullPrefix))
			}
			if existing, ok := localHeaderKeys[key]; ok {
				panic(fmt.Errorf("onedef: duplicate required header %q in group %q; already declared as %q", header.WireName, fullPrefix, existing.WireName))
			}
			localHeaderKeys[key] = header
			finalHeaders = upsertHeaderContract(finalHeaders, header)
			localHeaders = append(localHeaders, header)
		case meta.OmitHeaderNode:
			updated, removed := removeHeaderContract(finalHeaders, n.Header)
			if !removed {
				panic(fmt.Errorf("onedef: OmitHeader(%q) used in group %q but that header is not required by any parent group", n.Header.WireName, fullPrefix))
			}
			finalHeaders = updated
		}
	}

	nextVisiblePath := append([]string(nil), visiblePath...)
	if node.Exposed {
		nextVisiblePath = append(nextVisiblePath, lastPathSegment(node.Path))
	}
	providerHeaders := subtractHeaderContracts(finalHeaders, boundHeaders)
	if !node.Exposed && len(visiblePath) == 0 {
		for _, header := range providerHeaders {
			a.routeHeaders = upsertHeaderContract(a.routeHeaders, header)
		}
	}
	nextBoundHeaders := cloneHeaderContracts(boundHeaders)
	if node.Exposed {
		nextBoundHeaders = cloneHeaderContracts(finalHeaders)
	}

	finalBeforeHandlers := append([]any(nil), inheritedBeforeHandlers...)
	for _, child := range node.Children {
		if n, ok := child.MetaNode().(meta.BeforeHandleNode); ok {
			finalBeforeHandlers = append(finalBeforeHandlers, n.Handler)
		}
	}

	localAfterHandlers := make([]any, 0)
	for _, child := range node.Children {
		if n, ok := child.MetaNode().(meta.AfterHandleNode); ok {
			localAfterHandlers = append(localAfterHandlers, n.Handler)
		}
	}
	finalAfterHandlers := append([]any(nil), localAfterHandlers...)
	finalAfterHandlers = append(finalAfterHandlers, inheritedAfterHandlers...)

	finalObservers := append([]meta.Observer(nil), inheritedObservers...)
	for _, child := range node.Children {
		if n, ok := child.MetaNode().(meta.ObserveNode); ok {
			finalObservers = append(finalObservers, n.Observer)
		}
	}

	finalProvides := inheritedProvides.clone()
	for _, child := range node.Children {
		if binding, ok := child.MetaNode().(meta.ProvideBinding); ok {
			if err := finalProvides.addScopedBinding(binding); err != nil {
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
			LocalRequiredHeaders:    cloneHeaderContracts(localHeaders),
			ProviderRequiredHeaders: cloneHeaderContracts(providerHeaders),
			FinalRequiredHeaders:    cloneHeaderContracts(finalHeaders),
		}
	}

	result := make([]*meta.GroupMeta, 0)
	for _, child := range node.Children {
		switch n := child.MetaNode().(type) {
		case meta.EndpointNode:
			fullPath := joinPaths(fullPrefix, endpointLeafPath(n.Endpoint))
			endpointBeforeHandlers := append([]any(nil), finalBeforeHandlers...)
			endpointBeforeHandlers = append(endpointBeforeHandlers, n.BeforeHandlers...)
			endpointAfterHandlers := append([]any(nil), n.AfterHandlers...)
			endpointAfterHandlers = append(endpointAfterHandlers, finalAfterHandlers...)
			endpointObservers := append([]meta.Observer(nil), finalObservers...)
			endpointObservers = append(endpointObservers, n.Observers...)
			es := a.inspectEndpoint(n.Endpoint, &fullPath, &endpointGroupConfig{
				groupPath:       nextVisiblePath,
				finalHeaders:    finalHeaders,
				endpointHeaders: n.RequiredHeaders,
				beforeHandlers:  endpointBeforeHandlers,
				afterHandlers:   endpointAfterHandlers,
				observers:       endpointObservers,
			})
			es.SDKName = n.SDKName
			endpointProvides := finalProvides.clone()
			for _, binding := range n.Provides {
				if err := endpointProvides.addScopedBinding(binding); err != nil {
					panic(err)
				}
			}
			endpointErrorPolicy := finalErrorPolicy
			if n.ErrorPolicy != nil {
				endpointErrorPolicy = n.ErrorPolicy
			}
			a.registerEndpoint(es, true, endpointProvides, endpointErrorPolicy)
			es.ErrorBodyType = endpointErrorPolicy.ErrorBodyType()
			if groupMeta != nil {
				groupMeta.Endpoints = append(groupMeta.Endpoints, es)
			}
		case meta.EndpointsNode:
			for _, endpoint := range n.Endpoints {
				fullPath := joinPaths(fullPrefix, endpointLeafPath(endpoint))
				es := a.inspectEndpoint(endpoint, &fullPath, &endpointGroupConfig{
					groupPath:      nextVisiblePath,
					finalHeaders:   finalHeaders,
					beforeHandlers: finalBeforeHandlers,
					afterHandlers:  finalAfterHandlers,
					observers:      finalObservers,
				})
				a.registerEndpoint(es, true, finalProvides, finalErrorPolicy)
				es.ErrorBodyType = finalErrorPolicy.ErrorBodyType()
				if groupMeta != nil {
					groupMeta.Endpoints = append(groupMeta.Endpoints, es)
				}
			}
		case meta.GroupNode:
			children := a.registerGroupNode(n, fullPrefix, finalHeaders, nextVisiblePath, nextBoundHeaders, finalBeforeHandlers, finalAfterHandlers, finalObservers, finalProvides, finalErrorPolicy)
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

func cloneHeaderContracts(headers []meta.HeaderContract) []meta.HeaderContract {
	if len(headers) == 0 {
		return nil
	}
	return append([]meta.HeaderContract(nil), headers...)
}

func headerContractMap(headers []meta.HeaderContract) map[string]meta.HeaderContract {
	result := make(map[string]meta.HeaderContract, len(headers))
	for _, header := range headers {
		result[normalizeHeaderName(header.WireName)] = header
	}
	return result
}

func upsertHeaderContract(headers []meta.HeaderContract, header meta.HeaderContract) []meta.HeaderContract {
	key := normalizeHeaderName(header.WireName)
	for i, existing := range headers {
		if normalizeHeaderName(existing.WireName) == key {
			result := cloneHeaderContracts(headers)
			result[i] = header
			return result
		}
	}
	return append(cloneHeaderContracts(headers), header)
}

func removeHeaderContract(headers []meta.HeaderContract, header meta.HeaderContract) ([]meta.HeaderContract, bool) {
	key := normalizeHeaderName(header.WireName)
	result := make([]meta.HeaderContract, 0, len(headers))
	removed := false
	for _, existing := range headers {
		if normalizeHeaderName(existing.WireName) == key {
			removed = true
			continue
		}
		result = append(result, existing)
	}
	return result, removed
}

func subtractHeaderContracts(headers []meta.HeaderContract, subtract []meta.HeaderContract) []meta.HeaderContract {
	if len(headers) == 0 {
		return nil
	}
	if len(subtract) == 0 {
		return cloneHeaderContracts(headers)
	}

	result := make([]meta.HeaderContract, 0, len(headers))
	for _, header := range headers {
		if containsEquivalentHeader(subtract, header) {
			continue
		}
		result = append(result, header)
	}
	return result
}

func containsEquivalentHeader(headers []meta.HeaderContract, target meta.HeaderContract) bool {
	for _, header := range headers {
		if normalizeHeaderName(header.WireName) != normalizeHeaderName(target.WireName) {
			continue
		}
		if header.Name == target.Name && header.Type == target.Type && header.Description == target.Description {
			return true
		}
	}
	return false
}

func qualifiedStructName(structType reflect.Type) string {
	if structType.PkgPath() == "" {
		return structType.Name()
	}
	return structType.PkgPath() + "." + structType.Name()
}

func (a *App) buildIRJSON(initialisms []string) ([]byte, error) {
	ungrouped := make([]meta.EndpointStruct, 0, len(a.endpoints))
	for _, endpoint := range a.endpoints {
		if len(endpoint.GroupPath) == 0 {
			ungrouped = append(ungrouped, endpoint)
		}
	}

	doc, err := irbuild.BuildDocument(irbuild.Options{
		Initialisms: initialisms,
		Headers:     a.routeHeaders,
		Endpoints:   ungrouped,
		Groups:      a.groups,
	})
	if err != nil {
		return nil, err
	}

	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(doc); err != nil {
		return nil, fmt.Errorf("onedef: failed to encode IR JSON: %w", err)
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}
