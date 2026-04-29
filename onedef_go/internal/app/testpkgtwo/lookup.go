package testpkgtwo

import (
	"context"

	"github.com/failer-dev/onedef/onedef_go/internal/meta"
)

type Lookup struct {
	meta.GET `path:"/{slug}"`
	Request  struct {
		Slug string
	}
	Response struct {
		ID string `json:"id"`
	}
}

func (h *Lookup) Handle(ctx context.Context) error {
	h.Response.ID = h.Request.Slug
	return nil
}
