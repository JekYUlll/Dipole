package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/model"
)

var (
	ErrConversationTargetRequired = errors.New("conversation target is required")
	ErrConversationTargetNotFound = errors.New("conversation target not found")
)

type conversationRepository interface {
	UpsertDirectMessage(userUUID, targetUUID string, message *model.Message, unreadIncrement int) error
	ListByUserUUID(userUUID string, limit int) ([]*model.Conversation, error)
	ClearUnreadByConversationKey(userUUID, conversationKey string) error
}

type conversationUserFinder interface {
	GetByUUID(uuid string) (*model.User, error)
	ListByUUIDs(uuids []string) ([]*model.User, error)
}

type ConversationView struct {
	Conversation *model.Conversation
	TargetUser   *model.User
}

type ConversationService struct {
	repo       conversationRepository
	userFinder conversationUserFinder
}

func NewConversationService(repo conversationRepository, userFinder conversationUserFinder) *ConversationService {
	return &ConversationService{
		repo:       repo,
		userFinder: userFinder,
	}
}

func (s *ConversationService) UpdateDirectConversations(message *model.Message) error {
	if message == nil || message.TargetType != model.MessageTargetDirect {
		return nil
	}

	if err := s.repo.UpsertDirectMessage(message.SenderUUID, message.TargetUUID, message, 0); err != nil {
		return fmt.Errorf("upsert sender direct conversation: %w", err)
	}
	if err := s.repo.UpsertDirectMessage(message.TargetUUID, message.SenderUUID, message, 1); err != nil {
		return fmt.Errorf("upsert target direct conversation: %w", err)
	}

	return nil
}

func (s *ConversationService) ListForUser(userUUID string, limit int) ([]*ConversationView, error) {
	conversations, err := s.repo.ListByUserUUID(userUUID, normalizeConversationListLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list conversations for user: %w", err)
	}

	targetUUIDs := collectDirectConversationTargets(conversations)
	targetUsers, err := s.userFinder.ListByUUIDs(targetUUIDs)
	if err != nil {
		return nil, fmt.Errorf("list target users for conversations: %w", err)
	}

	targetUserByUUID := make(map[string]*model.User, len(targetUsers))
	for _, user := range targetUsers {
		targetUserByUUID[user.UUID] = user
	}

	result := make([]*ConversationView, 0, len(conversations))
	for _, conversation := range conversations {
		result = append(result, &ConversationView{
			Conversation: conversation,
			TargetUser:   targetUserByUUID[conversation.TargetUUID],
		})
	}

	return result, nil
}

func (s *ConversationService) MarkDirectConversationRead(userUUID, targetUUID string) error {
	targetUUID = strings.TrimSpace(targetUUID)
	if targetUUID == "" {
		return ErrConversationTargetRequired
	}

	targetUser, err := s.userFinder.GetByUUID(targetUUID)
	if err != nil {
		return fmt.Errorf("get target user in mark direct conversation read: %w", err)
	}
	if targetUser == nil {
		return ErrConversationTargetNotFound
	}

	if err := s.repo.ClearUnreadByConversationKey(userUUID, model.DirectConversationKey(userUUID, targetUUID)); err != nil {
		return fmt.Errorf("mark direct conversation read: %w", err)
	}

	return nil
}

func collectDirectConversationTargets(conversations []*model.Conversation) []string {
	seen := make(map[string]struct{}, len(conversations))
	targetUUIDs := make([]string, 0, len(conversations))
	for _, conversation := range conversations {
		if conversation.TargetType != model.MessageTargetDirect {
			continue
		}
		if _, ok := seen[conversation.TargetUUID]; ok {
			continue
		}
		seen[conversation.TargetUUID] = struct{}{}
		targetUUIDs = append(targetUUIDs, conversation.TargetUUID)
	}

	return targetUUIDs
}

func normalizeConversationListLimit(limit int) int {
	switch {
	case limit <= 0:
		return 20
	case limit > 50:
		return 50
	default:
		return limit
	}
}
