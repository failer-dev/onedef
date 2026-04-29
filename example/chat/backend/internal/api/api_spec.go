package api

import (
	"context"
	"log"

	"github.com/failer-dev/onedef/example/chat/backend/internal/api/customerapi"
	"github.com/failer-dev/onedef/example/chat/backend/internal/api/merchantapi"
	"github.com/failer-dev/onedef/example/chat/backend/internal/business"
	"github.com/failer-dev/onedef/example/chat/backend/internal/infra"
	"github.com/failer-dev/onedef/onedef_go"
)

func APISpec() *onedef.Spec {
	conversationStore := infra.NewMemoryConversationStore()
	auditLogger := infra.NewStdoutAuditLogger()

	return onedef.Group(
		"/api",
		onedef.Provide[business.ConversationStore](conversationStore),
		onedef.Provide[business.AuditLogger](auditLogger),
		onedef.AfterHandle(&AuditAfterHandle{}),
		onedef.Observe(onedef.ObserverFunc(logOutcome)),
		onedef.Group(
			"/customer",
			onedef.RequireHeader(customerapi.AuthorizationHeader),
			onedef.BeforeHandle(&customerapi.AuthBeforeHandle{}),
			onedef.Endpoints(
				&customerapi.StartConversation{},
				&customerapi.GetConversation{},
				&customerapi.AddMessage{},
			),
		),
		onedef.Group(
			"/merchant",
			onedef.RequireHeader(merchantapi.AuthorizationHeader),
			onedef.BeforeHandle(&merchantapi.AuthBeforeHandle{}),
			onedef.Endpoints(
				&merchantapi.GetConversation{},
				&merchantapi.ReplyToConversation{},
			),
		),
	)
}

func logOutcome(_ context.Context, outcome onedef.Outcome) {
	log.Printf(
		"outcome method=%s path=%s status=%d duration=%s endpoint=%s",
		outcome.Method,
		outcome.Path,
		outcome.Status,
		outcome.Duration,
		outcome.Endpoint,
	)
}
