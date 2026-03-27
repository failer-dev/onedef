package meta

type EndpointMethodMarker interface {
	// Unexported to seal this interface — only types defined within this package
	// are permitted to satisfy EndpointMethodMarker, making the set of valid
	// HTTP method markers closed and exhaustive by construction.
	//
	// Note: even if an external package defines a method with the same name,
	// unexported identifiers are scoped to their declaring package, so such a
	// method can never satisfy this interface from outside.
	value() EndpointMethod
}

type GET struct{}
type POST struct{}
type PUT struct{}
type PATCH struct{}
type DELETE struct{}
type HEAD struct{}
type OPTIONS struct{}

func (GET) value() EndpointMethod     { return EndpointMethodGet }
func (POST) value() EndpointMethod    { return EndpointMethodPost }
func (PUT) value() EndpointMethod     { return EndpointMethodPut }
func (PATCH) value() EndpointMethod   { return EndpointMethodPatch }
func (DELETE) value() EndpointMethod  { return EndpointMethodDelete }
func (HEAD) value() EndpointMethod    { return EndpointMethodHead }
func (OPTIONS) value() EndpointMethod { return EndpointMethodOptions }
