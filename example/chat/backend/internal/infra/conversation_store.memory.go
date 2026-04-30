package infra

import (
	"context"
	"fmt"
	"sync"

	"github.com/failer-dev/onedef/example/chat/backend/internal/business"
)

type MemoryConversationStore struct {
	mu            sync.Mutex
	nextID        int
	conversations map[string]business.Conversation
}

func NewMemoryConversationStore() *MemoryConversationStore {
	return &MemoryConversationStore{
		conversations: make(map[string]business.Conversation),
	}
}

func (s *MemoryConversationStore) Create(_ context.Context, conversation business.Conversation) (business.Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	conversation.ID = fmt.Sprintf("conv_%d", s.nextID)
	s.conversations[conversation.ID] = conversation
	return conversation, nil
}

func (s *MemoryConversationStore) Get(_ context.Context, id string) (business.Conversation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	conversation, ok := s.conversations[id]
	return conversation, ok
}

func (s *MemoryConversationStore) Update(_ context.Context, conversation business.Conversation) (business.Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.conversations[conversation.ID]; !ok {
		return business.Conversation{}, fmt.Errorf("conversation %q was not found", conversation.ID)
	}
	s.conversations[conversation.ID] = conversation
	return conversation, nil
}
