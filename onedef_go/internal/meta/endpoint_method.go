package meta

import (
	"github.com/failer-dev/wherr"
)

// EndpointMethod is the closed set of HTTP methods understood by onedef's
// endpoint marker types.
type EndpointMethod string

const (
	// EndpointMethodGet is the method selected by an embedded GET marker.
	EndpointMethodGet EndpointMethod = "GET"
	// EndpointMethodPost is the method selected by an embedded POST marker.
	EndpointMethodPost EndpointMethod = "POST"
	// EndpointMethodPut is the method selected by an embedded PUT marker.
	EndpointMethodPut EndpointMethod = "PUT"
	// EndpointMethodPatch is the method selected by an embedded PATCH marker.
	EndpointMethodPatch EndpointMethod = "PATCH"
	// EndpointMethodDelete is the method selected by an embedded DELETE marker.
	EndpointMethodDelete EndpointMethod = "DELETE"
	// EndpointMethodHead is the method selected by an embedded HEAD marker.
	EndpointMethodHead EndpointMethod = "HEAD"
	// EndpointMethodOptions is the method selected by an embedded OPTIONS marker.
	EndpointMethodOptions EndpointMethod = "OPTIONS"
)

// Invariant reports whether e is one of the methods produced by onedef's
// endpoint markers. It is useful when an EndpointMethod has been reconstructed
// from data rather than obtained from a marker.
func (e EndpointMethod) Invariant() error {
	switch e {
	case
		EndpointMethodGet, EndpointMethodPost, EndpointMethodPut,
		EndpointMethodPatch, EndpointMethodDelete, EndpointMethodHead,
		EndpointMethodOptions:
		return nil
	default:
		return wherr.Errorf("invalid EndpointMehtod: %q", e)
	}
}

// String returns the HTTP method token for e.
func (e EndpointMethod) String() string {
	return string(e)
}
