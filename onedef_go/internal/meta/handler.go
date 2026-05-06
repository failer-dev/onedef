package meta

import (
	"context"
	"net/http"
)

// HandlerFunc is the internal HTTP handler shape used by App error projection.
type HandlerFunc func(http.ResponseWriter, *http.Request) error

// Handler is implemented by endpoint structs after the runtime has populated
// Request and Provide. Implementations should write their successful result into
// Response and return an error to let the active error policy format failures.
type Handler interface {
	Handle(context.Context) error
}
