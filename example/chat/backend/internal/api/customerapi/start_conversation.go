package customerapi

import (
	"context"
	"strings"
	"time"

	"github.com/failer-dev/onedef/example/chat/backend/internal/business"
	"github.com/failer-dev/onedef/onedef_go"
)

type StartConversation struct {
	onedef.POST `path:"/conversations" status:"201"`
	Request     struct {
		StoreID string `json:"storeId"`
		Message string `json:"message"`
	}
	Provide struct {
		Actor         business.Actor
		Conversations business.ConversationStore
	}
	Response business.ConversationView
}

func (h *StartConversation) Handle(ctx context.Context) error {
	storeID := strings.TrimSpace(h.Request.StoreID)
	text := strings.TrimSpace(h.Request.Message)
	if storeID == "" {
		return onedef.BadRequest("missing_store", "storeId is required", nil)
	}
	if text == "" {
		return onedef.BadRequest("empty_message", "message must not be empty", nil)
	}

	now := time.Now().UTC()
	conversation := business.Conversation{
		StoreID:    storeID,
		CustomerID: h.Provide.Actor.ID,
		CreatedAt:  now,
		UpdatedAt:  now,
		Messages: []business.Message{
			{
				SenderKind: h.Provide.Actor.Kind,
				SenderID:   h.Provide.Actor.ID,
				Text:       text,
				SentAt:     now,
			},
		},
	}
	created, err := h.Provide.Conversations.Create(ctx, conversation)
	if err != nil {
		return err
	}
	h.Response = business.NewConversationView(created)
	return nil
}
