package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
)

var (
	ErrMessageTargetRequired    = errors.New("message target is required")
	ErrMessageContentRequired   = errors.New("message content is required")
	ErrMessageContentTooLong    = errors.New("message content is too long")
	ErrMessageTargetUnavailable = errors.New("message target is unavailable")
	ErrMessageTargetNotFound    = errors.New("message target not found")
	ErrMessageFriendRequired    = errors.New("direct message requires friendship")
	ErrMessageGroupForbidden    = errors.New("group message requires membership")
)

type messageRepository interface {
	Create(message *model.Message) error
	ListByConversationKey(conversationKey string, beforeID uint, limit int) ([]*model.Message, error)
}

type messageUserFinder interface {
	GetByUUID(uuid string) (*model.User, error)
}

type friendshipChecker interface {
	CanSendDirectMessage(userUUID, friendUUID string) (bool, error)
}

type groupMessageChecker interface {
	GetByUUID(groupUUID string) (*model.Group, error)
	GetMember(groupUUID, userUUID string) (*model.GroupMember, error)
	ListMembers(groupUUID string) ([]*model.GroupMember, error)
}

type MessageService struct {
	repo          messageRepository
	userFinder    messageUserFinder
	friendChecker friendshipChecker
	groupChecker  groupMessageChecker
	events        eventPublisher
}

func NewMessageService(repo messageRepository, userFinder messageUserFinder, friendChecker friendshipChecker, groupChecker groupMessageChecker, events eventPublisher) *MessageService {
	return &MessageService{
		repo:          repo,
		userFinder:    userFinder,
		friendChecker: friendChecker,
		groupChecker:  groupChecker,
		events:        events,
	}
}

func (s *MessageService) SendDirectMessage(senderUUID, targetUUID, content string) (*model.Message, error) {
	targetUUID = strings.TrimSpace(targetUUID)
	content = strings.TrimSpace(content)
	if targetUUID == "" {
		return nil, ErrMessageTargetRequired
	}
	if content == "" {
		return nil, ErrMessageContentRequired
	}
	if len([]rune(content)) > 1000 {
		return nil, ErrMessageContentTooLong
	}

	targetUser, err := s.userFinder.GetByUUID(targetUUID)
	if err != nil {
		return nil, fmt.Errorf("find target user in send direct message: %w", err)
	}
	if targetUser == nil || targetUser.Status == model.UserStatusDisabled {
		return nil, ErrMessageTargetUnavailable
	}
	if err := s.ensureDirectFriendship(senderUUID, targetUUID); err != nil {
		return nil, err
	}

	message := &model.Message{
		UUID:            generateMessageUUID(),
		ConversationKey: model.DirectConversationKey(senderUUID, targetUUID),
		SenderUUID:      strings.TrimSpace(senderUUID),
		TargetType:      model.MessageTargetDirect,
		TargetUUID:      targetUUID,
		MessageType:     model.MessageTypeText,
		Content:         content,
		SentAt:          time.Now().UTC(),
	}

	if err := s.repo.Create(message); err != nil {
		return nil, fmt.Errorf("persist direct message: %w", err)
	}
	s.publishMessageCreated("message.direct.created", message)

	return message, nil
}

func (s *MessageService) ListDirectMessages(currentUserUUID, targetUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	targetUUID = strings.TrimSpace(targetUUID)
	if targetUUID == "" {
		return nil, ErrMessageTargetRequired
	}

	targetUser, err := s.userFinder.GetByUUID(targetUUID)
	if err != nil {
		return nil, fmt.Errorf("find target user in list direct messages: %w", err)
	}
	if targetUser == nil {
		return nil, ErrMessageTargetNotFound
	}
	if err := s.ensureDirectFriendship(currentUserUUID, targetUUID); err != nil {
		return nil, err
	}

	messages, err := s.repo.ListByConversationKey(
		model.DirectConversationKey(currentUserUUID, targetUUID),
		beforeID,
		normalizeMessageListLimit(limit),
	)
	if err != nil {
		return nil, fmt.Errorf("list direct messages: %w", err)
	}

	return messages, nil
}

func (s *MessageService) SendGroupMessage(senderUUID, groupUUID, content string) (*model.Message, []string, error) {
	groupUUID = strings.TrimSpace(groupUUID)
	content = strings.TrimSpace(content)
	if groupUUID == "" {
		return nil, nil, ErrMessageTargetRequired
	}
	if content == "" {
		return nil, nil, ErrMessageContentRequired
	}
	if len([]rune(content)) > 1000 {
		return nil, nil, ErrMessageContentTooLong
	}

	if err := s.ensureGroupMessagePermission(senderUUID, groupUUID); err != nil {
		return nil, nil, err
	}

	message := &model.Message{
		UUID:            generateMessageUUID(),
		ConversationKey: model.GroupConversationKey(groupUUID),
		SenderUUID:      strings.TrimSpace(senderUUID),
		TargetType:      model.MessageTargetGroup,
		TargetUUID:      groupUUID,
		MessageType:     model.MessageTypeText,
		Content:         content,
		SentAt:          time.Now().UTC(),
	}

	if err := s.repo.Create(message); err != nil {
		return nil, nil, fmt.Errorf("persist group message: %w", err)
	}
	s.publishMessageCreated("message.group.created", message)

	recipientUUIDs, err := s.listGroupMemberUUIDs(groupUUID)
	if err != nil {
		return nil, nil, err
	}

	return message, recipientUUIDs, nil
}

func (s *MessageService) ListGroupMessages(currentUserUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	groupUUID = strings.TrimSpace(groupUUID)
	if groupUUID == "" {
		return nil, ErrMessageTargetRequired
	}
	if err := s.ensureGroupMessagePermission(currentUserUUID, groupUUID); err != nil {
		return nil, err
	}

	messages, err := s.repo.ListByConversationKey(
		model.GroupConversationKey(groupUUID),
		beforeID,
		normalizeMessageListLimit(limit),
	)
	if err != nil {
		return nil, fmt.Errorf("list group messages: %w", err)
	}

	return messages, nil
}

func (s *MessageService) publishMessageCreated(topic string, message *model.Message) {
	if s.events == nil || message == nil {
		return
	}

	payload := map[string]any{
		"message_id":       message.UUID,
		"conversation_key": message.ConversationKey,
		"sender_uuid":      message.SenderUUID,
		"target_uuid":      message.TargetUUID,
		"target_type":      message.TargetType,
		"message_type":     message.MessageType,
		"content":          message.Content,
		"sent_at":          message.SentAt,
	}
	_ = s.events.PublishJSON(context.Background(), topic, message.UUID, payload, nil)
}

func (s *MessageService) PersistedDirectMessage(senderUUID, targetUUID, content string) (*model.Message, error) {
	return s.SendDirectMessage(senderUUID, targetUUID, content)
}

func (s *MessageService) ensureDirectFriendship(userUUID, targetUUID string) error {
	if s.friendChecker == nil {
		return nil
	}

	areFriends, err := s.friendChecker.CanSendDirectMessage(strings.TrimSpace(userUUID), strings.TrimSpace(targetUUID))
	if err != nil {
		return fmt.Errorf("check friendship in direct message: %w", err)
	}
	if !areFriends {
		return ErrMessageFriendRequired
	}

	return nil
}

func (s *MessageService) ensureGroupMessagePermission(userUUID, groupUUID string) error {
	if s.groupChecker == nil {
		return ErrMessageTargetNotFound
	}

	group, err := s.groupChecker.GetByUUID(strings.TrimSpace(groupUUID))
	if err != nil {
		return fmt.Errorf("check group in message permission: %w", err)
	}
	if group == nil || group.Status != model.GroupStatusNormal {
		return ErrMessageTargetNotFound
	}

	member, err := s.groupChecker.GetMember(group.UUID, strings.TrimSpace(userUUID))
	if err != nil {
		return fmt.Errorf("check group member in message permission: %w", err)
	}
	if member == nil {
		return ErrMessageGroupForbidden
	}

	return nil
}

func (s *MessageService) listGroupMemberUUIDs(groupUUID string) ([]string, error) {
	members, err := s.groupChecker.ListMembers(strings.TrimSpace(groupUUID))
	if err != nil {
		return nil, fmt.Errorf("list group members in message service: %w", err)
	}

	memberUUIDs := make([]string, 0, len(members))
	for _, member := range members {
		if member == nil {
			continue
		}
		memberUUIDs = append(memberUUIDs, member.UserUUID)
	}

	return memberUUIDs, nil
}

func normalizeMessageListLimit(limit int) int {
	switch {
	case limit <= 0:
		return 20
	case limit > 50:
		return 50
	default:
		return limit
	}
}

func generateMessageUUID() string {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("generate message uuid: %w", err))
	}

	return "M" + strings.ToUpper(hex.EncodeToString(buf))
}
