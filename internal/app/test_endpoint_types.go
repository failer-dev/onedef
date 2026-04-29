package app

import (
	"context"

	"github.com/failer-dev/onedef/internal/meta"
)

type GroupedSpecTestEndpoint struct {
	meta.GET `path:"/{id}"`
	Request  struct{ ID string }
	Response GroupedSpecTestUser
}

type GroupedSpecTestUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type GroupedSpecTestError struct {
	Source string `json:"source"`
}

type GroupedSpecEndpointError struct {
	Source string `json:"source"`
}

func (h *GroupedSpecTestEndpoint) Handle(ctx context.Context) error {
	h.Response = GroupedSpecTestUser{ID: h.Request.ID, Name: "Grouped"}
	return nil
}

type CreateOrderSpecTestEndpoint struct {
	meta.POST `path:"/orders" status:"201"`
	Request   struct {
		IdempotencyKey string  `header:"Idempotency-Key"`
		RequestID      *string `header:"X-Request-Id"`
		Name           string  `json:"name"`
	}
	Response struct {
		ID string `json:"id"`
	}
}

func (h *CreateOrderSpecTestEndpoint) Handle(ctx context.Context) error {
	h.Response.ID = h.Request.IdempotencyKey
	if h.Request.RequestID != nil {
		h.Response.ID = *h.Request.RequestID
	}
	return nil
}

type DuplicateAuthorizationHeaderSpecTestEndpoint struct {
	meta.GET `path:"/secure"`
	Request  struct {
		Authorization string `header:"Authorization"`
	}
	Response struct{}
}

func (h *DuplicateAuthorizationHeaderSpecTestEndpoint) Handle(ctx context.Context) error {
	return nil
}
