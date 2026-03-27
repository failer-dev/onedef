package main

import (
	"context"

	"github.com/failer-dev/onedef"
)

type GetUser struct {
	onedef.GET `path:"/users/{id}"`
	Request    struct{ ID string }
	Response   User
}

func (h *GetUser) Handle(ctx context.Context) error {
	h.Response = User{
		ID:   h.Request.ID,
		Name: "Alice",
	}
	return nil
}
