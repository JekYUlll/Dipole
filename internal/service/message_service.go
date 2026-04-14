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
	mysqlDriver "github.com/go-sql-driver/mysql"
	sqlite3 "github.com/mattn/go-sqlite3"
	"gorm.io/gorm"
)

var (
	ErrMessageTargetRequired    = errors.New("message target is required")
	ErrMessageContentRequired   = errors.New("message content is required")
	ErrMessageContentTooLong    = errors.New("message content is too long")
	ErrMessageTargetUnavailable = errors.New("message target is unavailable")
	ErrMessageTargetNotFound    = errors.New("message target not found")
	ErrMessageFriendRequired    = errors.New("direct message requires friendship")
	ErrMessageGroupForbidden    = errors.New("group message requires membership")
	ErrMessageFileRequired      = errors.New("message file is required")
	ErrMessageFileUnavailable   = errors.New("message file is unavailable")
)

type messageRepository interface {
	Create(message *model.Message) error
	GetByUUID(uuid string) (*model.Message, error)
	ListByConversationKey(conversationKey string, beforeID uint, limit int) ([]*model.Message, error)
	ListOfflineByUserUUID(userUUID string, afterID uint, limit int) ([]*model.Message, error)
}

type messageFileFinder interface {
	GetOwnedFile(uploaderUUID, fileUUID string) (*model.UploadedFile, error)
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
	fileFinder    messageFileFinder
}

type MessageEventPayload struct {
	MessageID       string     `json:"message_id"`
	ConversationKey string     `json:"conversation_key"`
	SenderUUID      string     `json:"sender_uuid"`
	TargetUUID      string     `json:"target_uuid"`
	TargetType      int8       `json:"target_type"`
	MessageType     int8       `json:"message_type"`
	Content         string     `json:"content"`
	FileID          string     `json:"file_id,omitempty"`
	FileName        string     `json:"file_name,omitempty"`
	FileSize        int64      `json:"file_size,omitempty"`
	FileURL         string     `json:"file_url,omitempty"`
	FileContentType string     `json:"file_content_type,omitempty"`
	FileExpiresAt   *time.Time `json:"file_expires_at,omitempty"`
	SentAt          time.Time  `json:"sent_at"`
	RecipientUUIDs  []string   `json:"recipient_uuids,omitempty"`
}

func NewMessageService(repo messageRepository, userFinder messageUserFinder, friendChecker friendshipChecker, groupChecker groupMessageChecker, fileFinder messageFileFinder, events eventPublisher) *MessageService {
	return &MessageService{
		repo:          repo,
		userFinder:    userFinder,
		friendChecker: friendChecker,
		groupChecker:  groupChecker,
		events:        events,
		fileFinder:    fileFinder,
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
	if !targetUser.IsAssistant() {
		if err := s.ensureDirectFriendship(senderUUID, targetUUID); err != nil {
			return nil, err
		}
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

	if s.events == nil {
		if err := s.repo.Create(message); err != nil {
			return nil, fmt.Errorf("persist direct message: %w", err)
		}
		return message, nil
	}

	if err := s.publishMessageRequested("message.direct.send_requested", message, nil); err != nil {
		return nil, err
	}

	return message, nil
}

func (s *MessageService) SendAssistantTextMessage(assistantUUID, targetUUID, content string) (*model.Message, error) {
	assistantUUID = strings.TrimSpace(assistantUUID)
	targetUUID = strings.TrimSpace(targetUUID)
	content = strings.TrimSpace(content)
	if assistantUUID == "" || targetUUID == "" {
		return nil, ErrMessageTargetRequired
	}
	if content == "" {
		return nil, ErrMessageContentRequired
	}

	assistantUser, err := s.userFinder.GetByUUID(assistantUUID)
	if err != nil {
		return nil, fmt.Errorf("find assistant user in send assistant message: %w", err)
	}
	if assistantUser == nil || !assistantUser.IsAssistant() || assistantUser.Status == model.UserStatusDisabled {
		return nil, ErrMessageTargetUnavailable
	}

	targetUser, err := s.userFinder.GetByUUID(targetUUID)
	if err != nil {
		return nil, fmt.Errorf("find target user in send assistant message: %w", err)
	}
	if targetUser == nil || targetUser.Status == model.UserStatusDisabled {
		return nil, ErrMessageTargetUnavailable
	}

	message := &model.Message{
		UUID:            generateMessageUUID(),
		ConversationKey: model.DirectConversationKey(assistantUUID, targetUUID),
		SenderUUID:      assistantUUID,
		TargetType:      model.MessageTargetDirect,
		TargetUUID:      targetUUID,
		MessageType:     model.MessageTypeAIText,
		Content:         content,
		SentAt:          time.Now().UTC(),
	}

	if s.events == nil {
		if err := s.repo.Create(message); err != nil {
			return nil, fmt.Errorf("persist assistant message: %w", err)
		}
		return message, nil
	}

	if err := s.publishMessageRequested("message.direct.send_requested", message, nil); err != nil {
		return nil, err
	}

	return message, nil
}

func (s *MessageService) SendSystemDirectMessage(senderUUID, targetUUID, content string) (*model.Message, error) {
	senderUUID = strings.TrimSpace(senderUUID)
	targetUUID = strings.TrimSpace(targetUUID)
	content = strings.TrimSpace(content)
	if senderUUID == "" || targetUUID == "" {
		return nil, ErrMessageTargetRequired
	}
	if content == "" {
		return nil, ErrMessageContentRequired
	}

	senderUser, err := s.userFinder.GetByUUID(senderUUID)
	if err != nil {
		return nil, fmt.Errorf("find sender user in send system message: %w", err)
	}
	if senderUser == nil || senderUser.Status == model.UserStatusDisabled {
		return nil, ErrMessageTargetUnavailable
	}

	targetUser, err := s.userFinder.GetByUUID(targetUUID)
	if err != nil {
		return nil, fmt.Errorf("find target user in send system message: %w", err)
	}
	if targetUser == nil || targetUser.Status == model.UserStatusDisabled {
		return nil, ErrMessageTargetUnavailable
	}

	message := &model.Message{
		UUID:            generateMessageUUID(),
		ConversationKey: model.DirectConversationKey(senderUUID, targetUUID),
		SenderUUID:      senderUUID,
		TargetType:      model.MessageTargetDirect,
		TargetUUID:      targetUUID,
		MessageType:     model.MessageTypeSystem,
		Content:         content,
		SentAt:          time.Now().UTC(),
	}

	if s.events == nil {
		if err := s.repo.Create(message); err != nil {
			return nil, fmt.Errorf("persist system message: %w", err)
		}
		return message, nil
	}

	if err := s.publishMessageRequested("message.direct.send_requested", message, nil); err != nil {
		return nil, err
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
	if !targetUser.IsAssistant() {
		if err := s.ensureDirectFriendship(currentUserUUID, targetUUID); err != nil {
			return nil, err
		}
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

	recipientUUIDs, err := s.listGroupMemberUUIDs(groupUUID)
	if err != nil {
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

	if s.events == nil {
		if err := s.repo.Create(message); err != nil {
			return nil, nil, fmt.Errorf("persist group message: %w", err)
		}
		return message, recipientUUIDs, nil
	}

	if err := s.publishMessageRequested("message.group.send_requested", message, recipientUUIDs); err != nil {
		return nil, nil, err
	}

	return message, recipientUUIDs, nil
}

func (s *MessageService) SendDirectFileMessage(senderUUID, targetUUID, fileUUID string) (*model.Message, error) {
	targetUUID = strings.TrimSpace(targetUUID)
	fileUUID = strings.TrimSpace(fileUUID)
	if targetUUID == "" {
		return nil, ErrMessageTargetRequired
	}
	if fileUUID == "" {
		return nil, ErrMessageFileRequired
	}

	targetUser, err := s.userFinder.GetByUUID(targetUUID)
	if err != nil {
		return nil, fmt.Errorf("find target user in send direct file message: %w", err)
	}
	if targetUser == nil || targetUser.Status == model.UserStatusDisabled {
		return nil, ErrMessageTargetUnavailable
	}
	if !targetUser.IsAssistant() {
		if err := s.ensureDirectFriendship(senderUUID, targetUUID); err != nil {
			return nil, err
		}
	}

	message, err := s.newFileMessage(senderUUID, targetUUID, model.MessageTargetDirect, fileUUID)
	if err != nil {
		return nil, err
	}
	if s.events == nil {
		if err := s.repo.Create(message); err != nil {
			return nil, fmt.Errorf("persist direct file message: %w", err)
		}
		return message, nil
	}
	if err := s.publishMessageRequested("message.direct.send_requested", message, nil); err != nil {
		return nil, err
	}

	return message, nil
}

func (s *MessageService) SendGroupFileMessage(senderUUID, groupUUID, fileUUID string) (*model.Message, []string, error) {
	groupUUID = strings.TrimSpace(groupUUID)
	fileUUID = strings.TrimSpace(fileUUID)
	if groupUUID == "" {
		return nil, nil, ErrMessageTargetRequired
	}
	if fileUUID == "" {
		return nil, nil, ErrMessageFileRequired
	}
	if err := s.ensureGroupMessagePermission(senderUUID, groupUUID); err != nil {
		return nil, nil, err
	}

	recipientUUIDs, err := s.listGroupMemberUUIDs(groupUUID)
	if err != nil {
		return nil, nil, err
	}

	message, err := s.newFileMessage(senderUUID, groupUUID, model.MessageTargetGroup, fileUUID)
	if err != nil {
		return nil, nil, err
	}
	if s.events == nil {
		if err := s.repo.Create(message); err != nil {
			return nil, nil, fmt.Errorf("persist group file message: %w", err)
		}
		return message, recipientUUIDs, nil
	}
	if err := s.publishMessageRequested("message.group.send_requested", message, recipientUUIDs); err != nil {
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

func (s *MessageService) ListOfflineMessages(currentUserUUID string, afterID uint, limit int) ([]*model.Message, error) {
	messages, err := s.repo.ListOfflineByUserUUID(strings.TrimSpace(currentUserUUID), afterID, normalizeMessageListLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list offline messages: %w", err)
	}

	return messages, nil
}

func (s *MessageService) PersistRequestedMessage(payload MessageEventPayload) (*model.Message, error) {
	message := payloadToMessage(payload)
	if message == nil {
		return nil, fmt.Errorf("message payload is nil")
	}

	if err := s.repo.Create(message); err != nil {
		if !isDuplicateMessageError(err) {
			return nil, fmt.Errorf("persist requested message: %w", err)
		}

		existing, findErr := s.repo.GetByUUID(message.UUID)
		if findErr != nil {
			return nil, fmt.Errorf("find duplicate message by uuid: %w", findErr)
		}
		if existing == nil {
			return nil, fmt.Errorf("duplicate message %s not found after conflict", message.UUID)
		}
		message = existing
	}

	if err := s.publishMessageCreated(createdTopicForTargetType(message.TargetType), message, payload.RecipientUUIDs); err != nil {
		return nil, err
	}

	return message, nil
}

func (s *MessageService) PersistedDirectMessage(senderUUID, targetUUID, content string) (*model.Message, error) {
	return s.SendDirectMessage(senderUUID, targetUUID, content)
}

func (s *MessageService) publishMessageRequested(topic string, message *model.Message, recipientUUIDs []string) error {
	if s.events == nil || message == nil {
		return nil
	}

	payload := messageToEventPayload(message, recipientUUIDs)
	if err := s.events.PublishEvent(context.Background(), topic, message.UUID, topic, payload, nil); err != nil {
		return fmt.Errorf("publish requested message event: %w", err)
	}

	return nil
}

func (s *MessageService) publishMessageCreated(topic string, message *model.Message, recipientUUIDs []string) error {
	if s.events == nil || message == nil {
		return nil
	}

	payload := messageToEventPayload(message, recipientUUIDs)
	if err := s.events.PublishEvent(context.Background(), topic, message.UUID, topic, payload, nil); err != nil {
		return fmt.Errorf("publish created message event: %w", err)
	}

	return nil
}

func messageToEventPayload(message *model.Message, recipientUUIDs []string) MessageEventPayload {
	return MessageEventPayload{
		MessageID:       message.UUID,
		ConversationKey: message.ConversationKey,
		SenderUUID:      message.SenderUUID,
		TargetUUID:      message.TargetUUID,
		TargetType:      message.TargetType,
		MessageType:     message.MessageType,
		Content:         message.Content,
		FileID:          message.FileID,
		FileName:        message.FileName,
		FileSize:        message.FileSize,
		FileURL:         message.FileURL,
		FileContentType: message.FileContentType,
		FileExpiresAt:   message.FileExpiresAt,
		SentAt:          message.SentAt,
		RecipientUUIDs:  recipientUUIDs,
	}
}

func payloadToMessage(payload MessageEventPayload) *model.Message {
	return &model.Message{
		UUID:            strings.TrimSpace(payload.MessageID),
		ConversationKey: strings.TrimSpace(payload.ConversationKey),
		SenderUUID:      strings.TrimSpace(payload.SenderUUID),
		TargetUUID:      strings.TrimSpace(payload.TargetUUID),
		TargetType:      payload.TargetType,
		MessageType:     payload.MessageType,
		Content:         payload.Content,
		FileID:          strings.TrimSpace(payload.FileID),
		FileName:        strings.TrimSpace(payload.FileName),
		FileSize:        payload.FileSize,
		FileURL:         strings.TrimSpace(payload.FileURL),
		FileContentType: strings.TrimSpace(payload.FileContentType),
		FileExpiresAt:   payload.FileExpiresAt,
		SentAt:          payload.SentAt,
	}
}

func createdTopicForTargetType(targetType int8) string {
	if targetType == model.MessageTargetGroup {
		return "message.group.created"
	}

	return "message.direct.created"
}

func isDuplicateMessageError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}

	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return true
	}

	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) && sqliteErr.Code == sqlite3.ErrConstraint {
		return true
	}

	return false
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

func (s *MessageService) newFileMessage(senderUUID, targetUUID string, targetType int8, fileUUID string) (*model.Message, error) {
	if s.fileFinder == nil {
		return nil, ErrFileStorageUnavailable
	}

	uploadedFile, err := s.fileFinder.GetOwnedFile(strings.TrimSpace(senderUUID), fileUUID)
	if err != nil {
		switch {
		case errors.Is(err, ErrFileNotFound), errors.Is(err, ErrFilePermissionDenied):
			return nil, ErrMessageFileUnavailable
		default:
			return nil, fmt.Errorf("get uploaded file in message service: %w", err)
		}
	}

	conversationKey := model.DirectConversationKey(senderUUID, targetUUID)
	if targetType == model.MessageTargetGroup {
		conversationKey = model.GroupConversationKey(targetUUID)
	}

	expiresAt := time.Now().UTC().Add(7 * 24 * time.Hour)
	return &model.Message{
		UUID:            generateMessageUUID(),
		ConversationKey: conversationKey,
		SenderUUID:      strings.TrimSpace(senderUUID),
		TargetType:      targetType,
		TargetUUID:      strings.TrimSpace(targetUUID),
		MessageType:     model.MessageTypeFile,
		Content:         uploadedFile.FileName,
		FileID:          uploadedFile.UUID,
		FileName:        uploadedFile.FileName,
		FileSize:        uploadedFile.FileSize,
		FileURL:         uploadedFile.URL,
		FileContentType: uploadedFile.ContentType,
		FileExpiresAt:   &expiresAt,
		SentAt:          time.Now().UTC(),
	}, nil
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
