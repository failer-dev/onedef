package ir

import "strings"

func assignGroups(endpoints []Endpoint) []Endpoint {
	hasDeeperSibling := map[string]bool{}
	for _, ep := range endpoints {
		segments := pathSegments(ep.Path)
		if len(segments) >= 2 && !isPathParamSegment(segments[0]) {
			hasDeeperSibling[segments[0]] = true
		}
	}

	result := make([]Endpoint, len(endpoints))
	for i, ep := range endpoints {
		result[i] = ep
		segments := pathSegments(ep.Path)
		if shouldGenerateAtRoot(segments, hasDeeperSibling) {
			continue
		}
		first := segments[0]
		result[i].Group = segmentToSnakeCase(singularizeSegment(first))
	}

	return result
}

func pathSegments(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func shouldGenerateAtRoot(segments []string, hasDeeperSibling map[string]bool) bool {
	if len(segments) == 0 {
		return true
	}
	first := segments[0]
	if isPathParamSegment(first) {
		return true
	}
	if len(segments) == 1 && !hasDeeperSibling[first] && singularizeSegment(first) == first {
		return true
	}
	return false
}

func isPathParamSegment(segment string) bool {
	return strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}")
}

func singularizeSegment(segment string) string {
	switch {
	case strings.HasSuffix(segment, "ies") && len(segment) > 3:
		return segment[:len(segment)-3] + "y"
	case strings.HasSuffix(segment, "s") && !strings.HasSuffix(segment, "ss") && len(segment) > 1:
		return segment[:len(segment)-1]
	default:
		return segment
	}
}

func segmentToSnakeCase(segment string) string {
	var sb strings.Builder
	lastWasUnderscore := false

	for _, r := range strings.ToLower(segment) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			sb.WriteRune(r)
			lastWasUnderscore = false
		case !lastWasUnderscore && sb.Len() > 0:
			sb.WriteByte('_')
			lastWasUnderscore = true
		}
	}

	return strings.Trim(sb.String(), "_")
}
