package testpkgone

import (
	"context"

	"github.com/failer-dev/onedef/onedef_go/internal/meta"
)

type Lookup struct {
	meta.GET `path:"/{id}"`
	Request  struct {
		ID string
	}
	Response struct {
		ID string `json:"id"`
	}
}

func (h *Lookup) Handle(ctx context.Context) error {
	h.Response.ID = h.Request.ID
	return nil
}
