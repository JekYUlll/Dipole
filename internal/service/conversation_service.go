package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
)

var (
	ErrConversationTargetRequired   = errors.New("conversation target is required")
	ErrConversationTargetNotFound   = errors.New("conversation target not found")
	ErrConversationPermissionDenied = errors.New("conversation permission denied")
	ErrConversationRemarkTooLong    = errors.New("conversation remark is too long")
)

type conversationRepository interface {
	UpsertDirectMessage(userUUID, targetUUID string, message *model.Message, unreadIncrement int) error
	UpsertGroupMessage(userUUID, groupUUID string, message *model.Message, unreadIncrement int) error
	InitGroupConversation(userUUID, groupUUID, conversationKey string, createdAt time.Time) error
	ListByUserUUID(userUUID string, limit int) ([]*model.Conversation, error)
	GetByUserAndConversationKey(userUUID, conversationKey string) (*model.Conversation, error)
	ClearUnreadByConversationKey(userUUID, conversationKey string) error
	UpdateRemarkByConversationKey(userUUID, conversationKey, remark string) error
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
	groupRepo  conversationGroupRepository
	notifier   conversationNotifier
	events     eventPublisher
}

type conversationGroupRepository interface {
	GetByUUID(groupUUID string) (*model.Group, error)
	ListMembers(groupUUID string) ([]*model.GroupMember, error)
	GetMember(groupUUID, userUUID string) (*model.GroupMember, error)
}

type conversationNotifier interface {
	NotifyDirectRead(receipt ConversationReadReceipt)
}

type ConversationReadReceipt struct {
	ReaderUUID          string    `json:"reader_uuid"`
	TargetUUID          string    `json:"target_uuid"`
	TargetType          int8      `json:"target_type"`
	ConversationKey     string    `json:"conversation_key"`
	LastReadMessageUUID string    `json:"last_read_message_uuid"`
	ReadAt              time.Time `json:"read_at"`
}

func NewConversationService(repo conversationRepository, userFinder conversationUserFinder, groupRepo conversationGroupRepository, notifier conversationNotifier, events eventPublisher) *ConversationService {
	return &ConversationService{
		repo:       repo,
		userFinder: userFinder,
		groupRepo:  groupRepo,
		notifier:   notifier,
		events:     events,
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

// InitGroupConversations seeds an empty conversation row for every member of a newly created group.
// Called from the Kafka group.created consumer so each member sees the group in their conversation list
// even before the first message is sent.
func (s *ConversationService) InitGroupConversations(groupUUID string, memberUUIDs []string, createdAt time.Time) error {
	conversationKey := "group:" + groupUUID
	for _, userUUID := range memberUUIDs {
		if err := s.repo.InitGroupConversation(userUUID, groupUUID, conversationKey, createdAt); err != nil {
			return fmt.Errorf("init group conversation for user %s: %w", userUUID, err)
		}
	}
	return nil
}

func (s *ConversationService) UpdateGroupConversations(message *model.Message) error {
	if message == nil || message.TargetType != model.MessageTargetGroup || s.groupRepo == nil {
		return nil
	}

	members, err := s.groupRepo.ListMembers(message.TargetUUID)
	if err != nil {
		return fmt.Errorf("list group members for conversation update: %w", err)
	}
	for _, member := range members {
		if member == nil {
			continue
		}
		unreadIncrement := 1
		if member.UserUUID == message.SenderUUID {
			unreadIncrement = 0
		}
		if err := s.repo.UpsertGroupMessage(member.UserUUID, message.TargetUUID, message, unreadIncrement); err != nil {
			return fmt.Errorf("upsert group conversation for user %s: %w", member.UserUUID, err)
		}
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

func (s *ConversationService) MarkDirectConversationRead(userUUID, targetUUID string) (*ConversationReadReceipt, error) {
	targetUUID = strings.TrimSpace(targetUUID)
	if targetUUID == "" {
		return nil, ErrConversationTargetRequired
	}

	targetUser, err := s.userFinder.GetByUUID(targetUUID)
	if err != nil {
		return nil, fmt.Errorf("get target user in mark direct conversation read: %w", err)
	}
	if targetUser == nil {
		return nil, ErrConversationTargetNotFound
	}

	conversationKey := model.DirectConversationKey(userUUID, targetUUID)
	conversation, err := s.repo.GetByUserAndConversationKey(userUUID, conversationKey)
	if err != nil {
		return nil, fmt.Errorf("get direct conversation before mark read: %w", err)
	}
	if err := s.repo.ClearUnreadByConversationKey(userUUID, conversationKey); err != nil {
		return nil, fmt.Errorf("mark direct conversation read: %w", err)
	}

	receipt := buildConversationReadReceipt(userUUID, targetUUID, model.MessageTargetDirect, conversation)
	if err := s.dispatchDirectReadReceipt(receipt); err != nil {
		return nil, err
	}

	return receipt, nil
}

func (s *ConversationService) MarkGroupConversationRead(userUUID, groupUUID string) error {
	groupUUID = strings.TrimSpace(groupUUID)
	if groupUUID == "" {
		return ErrConversationTargetRequired
	}

	group, err := s.groupRepo.GetByUUID(groupUUID)
	if err != nil {
		return fmt.Errorf("get group in mark group conversation read: %w", err)
	}
	if group == nil || group.Status != model.GroupStatusNormal {
		return ErrConversationTargetNotFound
	}
	member, err := s.groupRepo.GetMember(groupUUID, userUUID)
	if err != nil {
		return fmt.Errorf("get group member in mark group conversation read: %w", err)
	}
	if member == nil {
		return ErrConversationPermissionDenied
	}

	if err := s.repo.ClearUnreadByConversationKey(userUUID, model.GroupConversationKey(groupUUID)); err != nil {
		return fmt.Errorf("mark group conversation read: %w", err)
	}

	return nil
}

func (s *ConversationService) UpdateGroupRemark(userUUID, groupUUID, remark string) (*model.Conversation, error) {
	groupUUID = strings.TrimSpace(groupUUID)
	if groupUUID == "" {
		return nil, ErrConversationTargetRequired
	}
	remark = strings.TrimSpace(remark)
	if len([]rune(remark)) > 50 {
		return nil, ErrConversationRemarkTooLong
	}

	group, err := s.groupRepo.GetByUUID(groupUUID)
	if err != nil {
		return nil, fmt.Errorf("get group in update group remark: %w", err)
	}
	if group == nil || group.Status != model.GroupStatusNormal {
		return nil, ErrConversationTargetNotFound
	}
	member, err := s.groupRepo.GetMember(groupUUID, userUUID)
	if err != nil {
		return nil, fmt.Errorf("get group member in update group remark: %w", err)
	}
	if member == nil {
		return nil, ErrConversationPermissionDenied
	}

	conversationKey := model.GroupConversationKey(groupUUID)
	if err := s.repo.UpdateRemarkByConversationKey(userUUID, conversationKey, remark); err != nil {
		return nil, fmt.Errorf("update group conversation remark: %w", err)
	}

	conversation, err := s.repo.GetByUserAndConversationKey(userUUID, conversationKey)
	if err != nil {
		return nil, fmt.Errorf("get group conversation after update remark: %w", err)
	}
	return conversation, nil
}

func buildConversationReadReceipt(readerUUID, targetUUID string, targetType int8, conversation *model.Conversation) *ConversationReadReceipt {
	if conversation == nil || conversation.LastMessageUUID == "" {
		return nil
	}

	return &ConversationReadReceipt{
		ReaderUUID:          strings.TrimSpace(readerUUID),
		TargetUUID:          strings.TrimSpace(targetUUID),
		TargetType:          targetType,
		ConversationKey:     conversation.ConversationKey,
		LastReadMessageUUID: conversation.LastMessageUUID,
		ReadAt:              time.Now().UTC(),
	}
}

func (s *ConversationService) dispatchDirectReadReceipt(receipt *ConversationReadReceipt) error {
	if receipt == nil {
		return nil
	}

	if s.events != nil {
		if err := s.events.PublishEvent(
			context.Background(),
			"conversation.direct.read",
			receipt.TargetUUID,
			"conversation.direct.read",
			receipt,
			nil,
		); err != nil {
			return fmt.Errorf("publish direct conversation read receipt: %w", err)
		}
		return nil
	}

	if s.notifier != nil {
		s.notifier.NotifyDirectRead(*receipt)
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
