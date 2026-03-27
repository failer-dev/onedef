package dart

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/failer-dev/onedef/internal/inspect"
	"github.com/failer-dev/onedef/internal/meta"
)

func generateClient(endpoints []meta.EndpointStruct, className string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("class %s {\n", className))
	sb.WriteString("  final String baseUrl;\n")
	sb.WriteString("  final http.Client _client;\n")
	sb.WriteString(fmt.Sprintf("\n  %s({required this.baseUrl, http.Client? client})\n", className))
	sb.WriteString("      : _client = client ?? http.Client();\n")

	for _, ep := range endpoints {
		sb.WriteString("\n")
		sb.WriteString(generateMethod(ep))
	}

	sb.WriteString("}\n")
	return sb.String()
}

func generateMethod(ep meta.EndpointStruct) string {
	var sb strings.Builder

	methodName := inspect.ToCamelCase(ep.StructName)
	returnType := dartReturnType(ep)
	params := dartMethodParams(ep)

	// Comment
	sb.WriteString(fmt.Sprintf("  // %s %s\n", ep.Method, ep.Path))

	// Signature
	sb.WriteString(fmt.Sprintf("  Future<%s> %s(%s) async {\n", returnType, methodName, params))

	// URL construction
	urlPath := dartUrlPath(ep.Path)
	hasQueryParams := len(ep.Request.QueryParameterFields) > 0

	if hasQueryParams {
		sb.WriteString("    final queryParams = <String, String>{};\n")
		for _, q := range ep.Request.QueryParameterFields {
			paramName := inspect.ToCamelCase(q.FieldName)
			sb.WriteString(fmt.Sprintf("    if (%s != null) queryParams['%s'] = %s.toString();\n",
				paramName, q.QueryKey, paramName))
		}
		sb.WriteString(fmt.Sprintf("    final uri = Uri.parse('$baseUrl%s').replace(queryParameters: queryParams.isNotEmpty ? queryParams : null);\n", urlPath))
	} else {
		sb.WriteString(fmt.Sprintf("    final uri = Uri.parse('$baseUrl%s');\n", urlPath))
	}

	// HTTP call
	httpMethod := strings.ToLower(string(ep.Method))
	hasBody := ep.Method == meta.EndpointMethodPost ||
		ep.Method == meta.EndpointMethodPut ||
		ep.Method == meta.EndpointMethodPatch

	if hasBody {
		sb.WriteString(fmt.Sprintf("    final resp = await _client.%s(\n", httpMethod))
		sb.WriteString("      uri,\n")
		sb.WriteString("      headers: {'Content-Type': 'application/json'},\n")
		sb.WriteString("      body: jsonEncode(body.toJson()),\n")
		sb.WriteString("    );\n")
	} else {
		sb.WriteString(fmt.Sprintf("    final resp = await _client.%s(uri);\n", httpMethod))
	}

	// Error handling
	sb.WriteString("    if (resp.statusCode >= 400) {\n")
	sb.WriteString("      throw Exception('HTTP ${resp.statusCode}: ${resp.body}');\n")
	sb.WriteString("    }\n")

	// Response parsing
	if returnType == "void" {
		// no parsing
	} else {
		sb.WriteString(fmt.Sprintf("    return %s.fromJson(jsonDecode(resp.body) as Map<String, dynamic>);\n", returnType))
	}

	sb.WriteString("  }\n")
	return sb.String()
}

func dartReturnType(ep meta.EndpointStruct) string {
	responseField, ok := ep.StructType.FieldByName("Response")
	if !ok {
		return "void"
	}
	t := responseField.Type
	// Empty struct → void
	if t.Kind() == reflect.Struct && t.NumField() == 0 {
		return "void"
	}
	return goDartType(t)
}

func dartMethodParams(ep meta.EndpointStruct) string {
	var params []string

	// Path params
	for _, p := range ep.Request.PathParameterFields {
		dartType := goDartType(p.FieldType)
		paramName := inspect.ToCamelCase(p.FieldName)
		params = append(params, fmt.Sprintf("required %s %s", dartType, paramName))
	}

	// Body param (POST/PUT/PATCH)
	hasBody := ep.Method == meta.EndpointMethodPost ||
		ep.Method == meta.EndpointMethodPut ||
		ep.Method == meta.EndpointMethodPatch
	if hasBody {
		requestField, ok := ep.StructType.FieldByName("Request")
		if ok {
			reqType := requestField.Type
			if reqType.Kind() == reflect.Struct && reqType.NumField() > 0 {
				dartType := requestBodyDartType(reqType)
				params = append(params, fmt.Sprintf("required %s body", dartType))
			}
		}
	}

	// Query params (optional)
	for _, q := range ep.Request.QueryParameterFields {
		dartType := goDartType(q.FieldType)
		paramName := inspect.ToCamelCase(q.FieldName)
		params = append(params, fmt.Sprintf("%s? %s", dartType, paramName))
	}

	if len(params) == 0 {
		return ""
	}
	return "{" + strings.Join(params, ", ") + "}"
}

func requestBodyDartType(reqType reflect.Type) string {
	if reqType.Name() != "" {
		return reqType.Name()
	}
	if name, ok := anonNames[reqType]; ok {
		return name
	}
	return "dynamic"
}

func dartUrlPath(path string) string {
	// /users/{id} → /users/$id
	result := path
	for {
		start := strings.Index(result, "{")
		if start == -1 {
			break
		}
		end := strings.Index(result, "}")
		paramName := result[start+1 : end]
		dartParam := inspect.ToCamelCase(paramName)
		result = result[:start] + "$" + dartParam + result[end+1:]
	}
	return result
}
