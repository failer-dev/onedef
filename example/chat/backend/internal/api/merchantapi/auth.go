package merchantapi

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
	onedef.Description("Merchant authorization header."),
	onedef.Example("Merchant merchant_456 store_main"),
	onedef.ParseHeader(parseAuth),
)

func parseAuth(raw string) (auth, error) {
	const prefix = "Merchant "
	if !strings.HasPrefix(raw, prefix) {
		return "", fmt.Errorf("merchant authorization must use %q", "Merchant <merchant_id> <store_id>")
	}
	parts := strings.Fields(strings.TrimPrefix(raw, prefix))
	if len(parts) != 2 {
		return "", fmt.Errorf("merchant authorization must use %q", "Merchant <merchant_id> <store_id>")
	}
	return auth(parts[0] + "|" + parts[1]), nil
}

func (a auth) actor() business.Actor {
	merchantID, storeID, _ := strings.Cut(string(a), "|")
	return business.Actor{Kind: "merchant", ID: merchantID, StoreID: storeID}
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
	h.Provide.Actor = h.Request.Authorization.actor()
	return nil
}
