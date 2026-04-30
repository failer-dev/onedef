package merchantapi

import (
	"context"
	"strings"
	"time"

	"github.com/failer-dev/onedef/example/chat/backend/internal/business"
	"github.com/failer-dev/onedef/onedef_go"
)

type ReplyToConversation struct {
	onedef.POST `path:"/conversations/{id}/messages" status:"201"`
	Request     struct {
		ID      string
		Message string `json:"message"`
	}
	Provide struct {
		Actor         business.Actor
		Conversations business.ConversationStore
	}
	Response business.ConversationView
}

func (h *ReplyToConversation) Handle(ctx context.Context) error {
	text := strings.TrimSpace(h.Request.Message)
	if text == "" {
		return onedef.BadRequest("empty_message", "message must not be empty", nil)
	}

	conversation, ok := h.Provide.Conversations.Get(ctx, h.Request.ID)
	if !ok {
		return onedef.NotFound("conversation_not_found", "conversation was not found", map[string]any{
			"id": h.Request.ID,
		})
	}
	if conversation.StoreID != h.Provide.Actor.StoreID {
		return onedef.Forbidden("wrong_store", "merchant cannot reply to another store's conversation", map[string]any{
			"conversationStoreId": conversation.StoreID,
			"merchantStoreId":     h.Provide.Actor.StoreID,
		})
	}

	now := time.Now().UTC()
	conversation.Messages = append(conversation.Messages, business.Message{
		SenderKind: h.Provide.Actor.Kind,
		SenderID:   h.Provide.Actor.ID,
		Text:       text,
		SentAt:     now,
	})
	conversation.UpdatedAt = now
	updated, err := h.Provide.Conversations.Update(ctx, conversation)
	if err != nil {
		return err
	}
	h.Response = business.NewConversationView(updated)
	return nil
}
