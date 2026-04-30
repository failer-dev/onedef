package merchantapi

import (
	"context"

	"github.com/failer-dev/onedef/example/chat/backend/internal/business"
	"github.com/failer-dev/onedef/onedef_go"
)

type GetConversation struct {
	onedef.GET `path:"/conversations/{id}"`
	Request    struct {
		ID string
	}
	Provide struct {
		Actor         business.Actor
		Conversations business.ConversationStore
	}
	Response business.ConversationView
}

func (h *GetConversation) Handle(ctx context.Context) error {
	conversation, ok := h.Provide.Conversations.Get(ctx, h.Request.ID)
	if !ok {
		return onedef.NotFound("conversation_not_found", "conversation was not found", map[string]any{
			"id": h.Request.ID,
		})
	}
	if conversation.StoreID != h.Provide.Actor.StoreID {
		return onedef.Forbidden("wrong_store", "merchant cannot read another store's conversation", map[string]any{
			"conversationStoreId": conversation.StoreID,
			"merchantStoreId":     h.Provide.Actor.StoreID,
		})
	}
	h.Response = business.NewConversationView(conversation)
	return nil
}
