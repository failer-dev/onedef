package meta

import "context"

type Handler interface {
	Handle(context.Context) error
}
