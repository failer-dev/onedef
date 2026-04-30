package meta

// EndpointMethodMarker is embedded anonymously in endpoint structs to declare
// their HTTP method and path tags. The interface is sealed so the inspector can
// treat the method marker set as exhaustive.
type EndpointMethodMarker interface {
	// Unexported to seal this interface: only types defined within this package
	// are permitted to satisfy EndpointMethodMarker, making the set of valid
	// HTTP method markers closed and exhaustive by construction.
	//
	// Note: even if an external package defines a method with the same name,
	// unexported identifiers are scoped to their declaring package, so that
	// method can never satisfy this interface from outside.
	value() EndpointMethod
}

// GET marks an endpoint struct as handling HTTP GET.
type GET struct{}

// POST marks an endpoint struct as handling HTTP POST.
type POST struct{}

// PUT marks an endpoint struct as handling HTTP PUT.
type PUT struct{}

// PATCH marks an endpoint struct as handling HTTP PATCH.
type PATCH struct{}

// DELETE marks an endpoint struct as handling HTTP DELETE.
type DELETE struct{}

// HEAD marks an endpoint struct as handling HTTP HEAD.
type HEAD struct{}

// OPTIONS marks an endpoint struct as handling HTTP OPTIONS.
type OPTIONS struct{}

func (GET) value() EndpointMethod     { return EndpointMethodGet }
func (POST) value() EndpointMethod    { return EndpointMethodPost }
func (PUT) value() EndpointMethod     { return EndpointMethodPut }
func (PATCH) value() EndpointMethod   { return EndpointMethodPatch }
func (DELETE) value() EndpointMethod  { return EndpointMethodDelete }
func (HEAD) value() EndpointMethod    { return EndpointMethodHead }
func (OPTIONS) value() EndpointMethod { return EndpointMethodOptions }
