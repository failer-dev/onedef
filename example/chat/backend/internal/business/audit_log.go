package business

import "context"

type AuditLogger interface {
	Record(context.Context, AuditEvent)
}

type AuditEvent struct {
	ActorKind      string
	ActorID        string
	StoreID        string
	ConversationID string
	MessageCount   int
}
