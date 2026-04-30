package api

import (
	"context"

	"github.com/failer-dev/onedef/example/chat/backend/internal/business"
)

type AuditAfterHandle struct {
	Provide struct {
		Actor       business.Actor
		AuditLogger business.AuditLogger
	}
	Response business.ConversationView
}

func (h *AuditAfterHandle) AfterHandle(ctx context.Context) error {
	auditLogger := h.Provide.AuditLogger
	auditLogger.Record(ctx, business.AuditEvent{
		ActorKind:      h.Provide.Actor.Kind,
		ActorID:        h.Provide.Actor.ID,
		StoreID:        h.Response.StoreID,
		ConversationID: h.Response.ID,
		MessageCount:   len(h.Response.Messages),
	})
	return nil
}
