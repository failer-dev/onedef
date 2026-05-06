package customerapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/failer-dev/onedef/example/chat/backend/internal/business"
	"github.com/failer-dev/onedef/onedef_go"
)

type auth string

var AuthorizationHeader = onedef.Header[auth](
	"Authorization",
	onedef.Description("Customer authorization header."),
	onedef.Example("Customer customer_123"),
	onedef.ParseHeader(parseAuth),
)

func parseAuth(raw string) (auth, error) {
	const prefix = "Customer "
	if !strings.HasPrefix(raw, prefix) {
		return "", fmt.Errorf("customer authorization must use %q", "Customer <customer_id>")
	}
	id := strings.TrimSpace(strings.TrimPrefix(raw, prefix))
	if id == "" {
		return "", fmt.Errorf("customer authorization is missing customer id")
	}
	return auth(id), nil
}

type AuthBeforeHandle struct {
	Request struct {
		Authorization auth `header:"Authorization"`
	}
	Provide struct {
		Actor business.Actor
	}
}

func (h *AuthBeforeHandle) BeforeHandle(context.Context) error {
	h.Provide.Actor = business.Actor{Kind: "customer", ID: string(h.Request.Authorization)}
	return nil
}
