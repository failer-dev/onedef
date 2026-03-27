package dart

import (
	"fmt"
	"strings"
)

func generateModels(types []structMeta) string {
	var sb strings.Builder

	for i, sm := range types {
		if i > 0 {
			sb.WriteString("\n")
		}

		// class declaration
		sb.WriteString(fmt.Sprintf("class %s {\n", sm.Name))

		// fields
		for _, f := range sm.Fields {
			sb.WriteString(fmt.Sprintf("  final %s %s;\n", f.DartType, f.DartName))
		}

		// constructor
		sb.WriteString(fmt.Sprintf("\n  %s({\n", sm.Name))
		for _, f := range sm.Fields {
			if f.Nullable {
				sb.WriteString(fmt.Sprintf("    this.%s,\n", f.DartName))
			} else {
				sb.WriteString(fmt.Sprintf("    required this.%s,\n", f.DartName))
			}
		}
		sb.WriteString("  });\n")

		// fromJson
		sb.WriteString(fmt.Sprintf("\n  factory %s.fromJson(Map<String, dynamic> json) => %s(\n", sm.Name, sm.Name))
		for _, f := range sm.Fields {
			sb.WriteString(fmt.Sprintf("    %s: %s,\n", f.DartName, dartFromJson(f)))
		}
		sb.WriteString("  );\n")

		// toJson
		sb.WriteString("\n  Map<String, dynamic> toJson() => {\n")
		for _, f := range sm.Fields {
			sb.WriteString(fmt.Sprintf("    '%s': %s,\n", f.DartName, dartToJson(f)))
		}
		sb.WriteString("  };\n")

		sb.WriteString("}\n")
	}

	return sb.String()
}

func dartFromJson(f fieldMeta) string {
	dartType := f.DartType

	// Nullable
	if f.Nullable {
		inner := strings.TrimSuffix(dartType, "?")
		if isJsonPrimitive(inner) {
			return fmt.Sprintf("json['%s'] as %s", f.DartName, dartType)
		}
		return fmt.Sprintf("json['%s'] != null ? %s.fromJson(json['%s'] as Map<String, dynamic>) : null",
			f.DartName, inner, f.DartName)
	}

	// List<T>
	if strings.HasPrefix(dartType, "List<") {
		elemType := dartType[5 : len(dartType)-1]
		if isJsonPrimitive(elemType) {
			return fmt.Sprintf("(json['%s'] as List<dynamic>).cast<%s>()", f.DartName, elemType)
		}
		return fmt.Sprintf("(json['%s'] as List<dynamic>).map((e) => %s.fromJson(e as Map<String, dynamic>)).toList()",
			f.DartName, elemType)
	}

	// Map
	if strings.HasPrefix(dartType, "Map<") {
		return fmt.Sprintf("json['%s'] as Map<String, dynamic>", f.DartName)
	}

	// Primitive
	if isJsonPrimitive(dartType) {
		return fmt.Sprintf("json['%s'] as %s", f.DartName, dartType)
	}

	// Nested struct
	return fmt.Sprintf("%s.fromJson(json['%s'] as Map<String, dynamic>)", dartType, f.DartName)
}

func dartToJson(f fieldMeta) string {
	dartType := f.DartType

	// Nullable struct
	if f.Nullable && !isJsonPrimitive(strings.TrimSuffix(dartType, "?")) {
		return fmt.Sprintf("%s?.toJson()", f.DartName)
	}

	// List<T> where T is struct
	if strings.HasPrefix(dartType, "List<") {
		elemType := dartType[5 : len(dartType)-1]
		if !isJsonPrimitive(elemType) {
			return fmt.Sprintf("%s.map((e) => e.toJson()).toList()", f.DartName)
		}
	}

	// Primitive, nullable primitive, List<primitive>, Map
	return f.DartName
}

func isJsonPrimitive(dartType string) bool {
	switch dartType {
	case "String", "int", "double", "bool", "dynamic":
		return true
	}
	return false
}
