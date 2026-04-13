package service

import (
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
)

type messageRepository interface {
	Create(message *model.Message) error
	ListByConversationKey(conversationKey string, beforeID uint, limit int) ([]*model.Message, error)
}

type messageUserFinder interface {
	GetByUUID(uuid string) (*model.User, error)
}

type friendshipChecker interface {
	AreFriends(userUUID, friendUUID string) (bool, error)
}

type MessageService struct {
	repo          messageRepository
	userFinder    messageUserFinder
	friendChecker friendshipChecker
}

func NewMessageService(repo messageRepository, userFinder messageUserFinder, friendChecker friendshipChecker) *MessageService {
	return &MessageService{
		repo:          repo,
		userFinder:    userFinder,
		friendChecker: friendChecker,
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

func (s *MessageService) PersistedDirectMessage(senderUUID, targetUUID, content string) (*model.Message, error) {
	return s.SendDirectMessage(senderUUID, targetUUID, content)
}

func (s *MessageService) ensureDirectFriendship(userUUID, targetUUID string) error {
	if s.friendChecker == nil {
		return nil
	}

	areFriends, err := s.friendChecker.AreFriends(strings.TrimSpace(userUUID), strings.TrimSpace(targetUUID))
	if err != nil {
		return fmt.Errorf("check friendship in direct message: %w", err)
	}
	if !areFriends {
		return ErrMessageFriendRequired
	}

	return nil
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
