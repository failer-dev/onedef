package business

import (
	"context"
	"time"
)

type Conversation struct {
	ID         string
	StoreID    string
	CustomerID string
	Messages   []Message
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Message struct {
	SenderKind string
	SenderID   string
	Text       string
	SentAt     time.Time
}

type ConversationView struct {
	ID         string        `json:"id"`
	StoreID    string        `json:"storeId"`
	CustomerID string        `json:"customerId"`
	Messages   []MessageView `json:"messages"`
}

type MessageView struct {
	SenderKind string `json:"senderKind"`
	SenderID   string `json:"senderId"`
	Text       string `json:"text"`
	SentAt     string `json:"sentAt"`
}

func NewConversationView(conversation Conversation) ConversationView {
	messages := make([]MessageView, 0, len(conversation.Messages))
	for _, message := range conversation.Messages {
		messages = append(messages, MessageView{
			SenderKind: message.SenderKind,
			SenderID:   message.SenderID,
			Text:       message.Text,
			SentAt:     message.SentAt.UTC().Format(time.RFC3339),
		})
	}
	return ConversationView{
		ID:         conversation.ID,
		StoreID:    conversation.StoreID,
		CustomerID: conversation.CustomerID,
		Messages:   messages,
	}
}

type ConversationStore interface {
	Create(context.Context, Conversation) (Conversation, error)
	Get(context.Context, string) (Conversation, bool)
	Update(context.Context, Conversation) (Conversation, error)
}
