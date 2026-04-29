package app

import (
	"fmt"

	"github.com/failer-dev/onedef/internal/meta"
)

type middlewareEntry struct {
	name       string
	middleware meta.Middleware
}

func appendMiddlewareEntries(entries []middlewareEntry, middlewares []meta.Middleware) []middlewareEntry {
	result := cloneMiddlewareEntries(entries)
	for _, middleware := range middlewares {
		meta.MustMiddleware(middleware)
		entry := middlewareEntry{middleware: middleware}
		if name, ok := meta.MiddlewareName(middleware); ok {
			if hasMiddlewareName(result, name) {
				panic(fmt.Errorf("onedef: duplicate middleware name %q", name))
			}
			entry.name = name
		}
		result = append(result, entry)
	}
	return result
}

func skipMiddlewareEntries(entries []middlewareEntry, names []string) []middlewareEntry {
	if len(names) == 0 {
		return cloneMiddlewareEntries(entries)
	}

	result := cloneMiddlewareEntries(entries)
	for _, name := range names {
		index := middlewareIndexByName(result, name)
		if index < 0 {
			panic(fmt.Errorf("onedef: middleware %q is not active", name))
		}
		result = append(result[:index], result[index+1:]...)
	}
	return result
}

func applyMiddlewareEntries(handler meta.HandlerFunc, entries []middlewareEntry) meta.HandlerFunc {
	for i := len(entries) - 1; i >= 0; i-- {
		wrapped := entries[i].middleware.Wrap(handler)
		if wrapped == nil {
			if entries[i].name != "" {
				panic(fmt.Errorf("onedef: middleware %q returned nil handler", entries[i].name))
			}
			panic("onedef: middleware returned nil handler")
		}
		handler = wrapped
	}
	return handler
}

func cloneMiddlewareEntries(entries []middlewareEntry) []middlewareEntry {
	if len(entries) == 0 {
		return nil
	}
	return append([]middlewareEntry(nil), entries...)
}

func hasMiddlewareName(entries []middlewareEntry, name string) bool {
	return middlewareIndexByName(entries, name) >= 0
}

func middlewareIndexByName(entries []middlewareEntry, name string) int {
	for i, entry := range entries {
		if entry.name == name {
			return i
		}
	}
	return -1
}
