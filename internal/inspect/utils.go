package inspect

import (
	"reflect"
	"regexp"
	"strings"
	"unicode"
)

var pathParamPattern = regexp.MustCompile(`\{(\w+)\}`)

func extractPathParams(path string) map[string]string {
	matches := pathParamPattern.FindAllStringSubmatch(path, -1)
	result := map[string]string{}

	for _, m := range matches {
		result[normalizePathParam(m[1])] = m[1]
	}

	return result
}

func normalizePathParam(v string) string {
	return strings.ReplaceAll(strings.ToLower(v), "_", "")
}

// wireName은 struct 필드의 외부 전송 이름을 결정한다.
// json 태그가 있으면 태그 값, 없으면 toCamelCase(필드명).
func WireName(sf reflect.StructField) string {
	tag := sf.Tag.Get("json")
	if tag != "" && tag != "-" {
		if i := strings.Index(tag, ","); i != -1 {
			tag = tag[:i]
		}
		return tag
	}
	return ToCamelCase(sf.Name)
}

func ToCamelCase(s string) string {
	if len(s) == 0 {
		return s
	}
	runes := []rune(s)

	i := 0
	for i < len(runes) && unicode.IsUpper(runes[i]) {
		i++
	}
	if i == 0 {
		return s
	}

	// 전부 대문자 (ID, URL) → 전체 소문자
	if i == len(runes) {
		for j := range runes {
			runes[j] = unicode.ToLower(runes[j])
		}
		return string(runes)
	}

	// 연속 대문자 2개 이상 → 마지막 하나는 다음 단어 시작이므로 유지
	if i > 1 {
		for j := 0; j < i-1; j++ {
			runes[j] = unicode.ToLower(runes[j])
		}
	} else {
		runes[0] = unicode.ToLower(runes[0])
	}
	return string(runes)
}
