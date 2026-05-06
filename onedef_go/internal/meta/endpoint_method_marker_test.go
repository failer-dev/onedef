package meta

import "testing"

type invalidMarker struct{ v EndpointMethod }

func (m invalidMarker) value() EndpointMethod { return m.v }

func Test_EndpointMethodMarker_Valid(t *testing.T) {
	cases := []struct {
		marker EndpointMethodMarker
		want   EndpointMethod
	}{
		{GET{}, EndpointMethodGet},
		{POST{}, EndpointMethodPost},
		{PUT{}, EndpointMethodPut},
		{PATCH{}, EndpointMethodPatch},
		{DELETE{}, EndpointMethodDelete},
		{HEAD{}, EndpointMethodHead},
		{OPTIONS{}, EndpointMethodOptions},
	}

	for _, c := range cases {
		if c.want != c.marker.value() {
			t.Errorf("got %q, want %q", c.marker.value(), c.want)
		}
	}
}

func Test_EndpointMethodMarker_Invalid(t *testing.T) {
	cases := []EndpointMethodMarker{
		invalidMarker{""},
		invalidMarker{"get"},
		invalidMarker{"post"},
		invalidMarker{"CONNECT"},
		invalidMarker{"TRACE"},
	}

	for _, c := range cases {
		if err := c.value().Invariant(); err == nil {
			t.Errorf("expected invariant violation for %q, but got nil", c.value())
		}
	}
}
