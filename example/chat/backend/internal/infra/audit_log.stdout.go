package infra

import (
	"context"
	"log"

	"github.com/failer-dev/onedef/example/chat/backend/internal/business"
)

type StdoutAuditLogger struct{}

func NewStdoutAuditLogger() *StdoutAuditLogger {
	return &StdoutAuditLogger{}
}

func (StdoutAuditLogger) Record(_ context.Context, event business.AuditEvent) {
	log.Printf(
		"audit actor=%s:%s store=%s conversation=%s messages=%d",
		event.ActorKind,
		event.ActorID,
		event.StoreID,
		event.ConversationID,
		event.MessageCount,
	)
}
