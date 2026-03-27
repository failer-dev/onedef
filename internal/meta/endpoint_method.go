package meta

import (
	"github.com/failer-dev/wherr"
)

type EndpointMethod string

const (
	EndpointMethodGet     EndpointMethod = "GET"
	EndpointMethodPost    EndpointMethod = "POST"
	EndpointMethodPut     EndpointMethod = "PUT"
	EndpointMethodPatch   EndpointMethod = "PATCH"
	EndpointMethodDelete  EndpointMethod = "DELETE"
	EndpointMethodHead    EndpointMethod = "HEAD"
	EndpointMethodOptions EndpointMethod = "OPTIONS"
)

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

func (e EndpointMethod) String() string {
	return string(e)
}
