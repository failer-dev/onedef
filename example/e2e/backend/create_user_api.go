package main

import (
	"context"

	"github.com/failer-dev/onedef"
)

type CreateUser struct {
	onedef.POST `path:"/users"`
	Request     struct {
		Name string `json:"name"`
	}
	Response User
}

func (h *CreateUser) Handle(ctx context.Context) error {
	h.Response = User{
		ID:   "new-id",
		Name: h.Request.Name,
	}
	return nil
}
