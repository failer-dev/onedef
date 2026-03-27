package dart

import (
	"reflect"
	"strings"
	"unicode"

	"github.com/failer-dev/onedef/internal/inspect"
	"github.com/failer-dev/onedef/internal/meta"
)

type structMeta struct {
	Name   string
	Fields []fieldMeta
}

type fieldMeta struct {
	DartName string
	DartType string
	Nullable bool
}

func goDartType(t reflect.Type) string {
	if t.Kind() == reflect.Ptr {
		return goDartType(t.Elem()) + "?"
	}
	if t.Kind() == reflect.Slice {
		return "List<" + goDartType(t.Elem()) + ">"
	}
	if t.Kind() == reflect.Map {
		return "Map<String, dynamic>"
	}
	switch t.Kind() {
	case reflect.String:
		return "String"
	case reflect.Bool:
		return "bool"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "int"
	case reflect.Float32, reflect.Float64:
		return "double"
	case reflect.Struct:
		if t.PkgPath() == "github.com/google/uuid" && t.Name() == "UUID" {
			return "String"
		}
		if t.Name() == "" {
			if name, ok := anonNames[t]; ok {
				return name
			}
			return "dynamic"
		}
		return t.Name()
	}
	return "dynamic"
}

// anonNames maps anonymous reflect.Type → generated name (e.g. "GetUserResponse").
// Used by goDartType to resolve anonymous struct references.
var anonNames map[reflect.Type]string

func collectTypes(endpoints []meta.EndpointStruct) []structMeta {
	seen := map[reflect.Type]bool{}
	anonNames = map[reflect.Type]string{}
	var result []structMeta

	var walk func(t reflect.Type, nameHint string)
	walk = func(t reflect.Type, nameHint string) {
		// Unwrap pointer/slice
		for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct {
			return
		}
		// Skip uuid.UUID
		if t.PkgPath() == "github.com/google/uuid" && t.Name() == "UUID" {
			return
		}
		if seen[t] {
			return
		}
		seen[t] = true

		name := t.Name()
		if name == "" {
			name = nameHint
			anonNames[t] = name
		}

		sm := structMeta{
			Name: name,
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			if f.Anonymous {
				continue
			}
			sm.Fields = append(sm.Fields, fieldMeta{
				DartName: inspect.WireName(f),
				DartType: goDartType(f.Type),
				Nullable: f.Type.Kind() == reflect.Ptr,
			})
			walk(f.Type, name+f.Name)
		}
		result = append(result, sm)
	}

	for _, ep := range endpoints {
		if responseField, ok := ep.StructType.FieldByName("Response"); ok {
			walk(responseField.Type, ep.StructName+"Response")
		}
		hasBody := ep.Method == meta.EndpointMethodPost ||
			ep.Method == meta.EndpointMethodPut ||
			ep.Method == meta.EndpointMethodPatch
		if hasBody {
			if requestField, ok := ep.StructType.FieldByName("Request"); ok {
				walk(requestField.Type, ep.StructName+"Request")
			}
		}
	}

	return result
}

func snakeToPascal(s string) string {
	parts := strings.Split(s, "_")
	var sb strings.Builder
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		runes := []rune(p)
		runes[0] = unicode.ToUpper(runes[0])
		sb.WriteString(string(runes))
	}
	return sb.String()
}
