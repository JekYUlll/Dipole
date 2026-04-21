package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
	platformCache "github.com/JekYUlll/Dipole/internal/platform/cache"
	platformHotGroup "github.com/JekYUlll/Dipole/internal/platform/hotgroup"
	"github.com/JekYUlll/Dipole/internal/platform/idgen"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"golang.org/x/sync/singleflight"
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
	StoreWithOutbox(message *model.Message, event *model.OutboxEvent) error
	EnsureOutbox(event *model.OutboxEvent) error
	GetByUUID(uuid string) (*model.Message, error)
	GetBySenderAndClientMessageID(senderUUID, clientMessageID string) (*model.Message, error)
	HasConversationMessages(conversationKey string) (bool, error)
	ListByConversationKey(conversationKey string, beforeID uint, limit int) ([]*model.Message, error)
	ListByConversationKeyAfter(conversationKey string, afterID uint, limit int) ([]*model.Message, error)
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

type hotGroupObserver interface {
	ObserveMessage(groupUUID string, memberCount int) (platformHotGroup.Status, error)
}

type MessageService struct {
	repo          messageRepository
	userFinder    messageUserFinder
	friendChecker friendshipChecker
	groupChecker  groupMessageChecker
	events        eventPublisher
	fileFinder    messageFileFinder
	hotGroups     hotGroupObserver
	// 热群改成 notify + pull 后，同一台节点上会出现很多相同的
	// group_uuid/after_id/limit 增量拉取请求。singleflight 在 service 层
	// 合并这些回源，避免瞬时把同一页消息重复打到 MySQL。
	groupPulls singleflight.Group
}

type MessageEventPayload struct {
	MessageID       string     `json:"message_id"`
	ClientMessageID string     `json:"client_message_id,omitempty"`
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

func NewMessageService(repo messageRepository, userFinder messageUserFinder, friendChecker friendshipChecker, groupChecker groupMessageChecker, fileFinder messageFileFinder, events eventPublisher, hotGroups hotGroupObserver) *MessageService {
	return &MessageService{
		repo:          repo,
		userFinder:    userFinder,
		friendChecker: friendChecker,
		groupChecker:  groupChecker,
		events:        events,
		fileFinder:    fileFinder,
		hotGroups:     hotGroups,
	}
}

func (s *MessageService) SendDirectMessage(senderUUID, targetUUID, content, clientMessageID string) (*model.Message, error) {
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

	return s.buildAndDispatchDirect(senderUUID, targetUUID, content, clientMessageID, model.MessageTypeText)
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

	return s.buildAndDispatchDirect(assistantUUID, targetUUID, content, generateClientMessageID(), model.MessageTypeAIText)
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

	return s.buildAndDispatchDirect(senderUUID, targetUUID, content, generateClientMessageID(), model.MessageTypeSystem)
}

// buildAndDispatchDirect constructs a direct message and either persists it
// synchronously (no events publisher) or publishes a send_requested event.
func (s *MessageService) buildAndDispatchDirect(senderUUID, targetUUID, content, clientMessageID string, msgType int8) (*model.Message, error) {
	message := &model.Message{
		UUID:            generateMessageUUID(),
		ClientMessageID: normalizeClientMessageID(clientMessageID),
		ConversationKey: model.DirectConversationKey(senderUUID, targetUUID),
		SenderUUID:      senderUUID,
		TargetType:      model.MessageTargetDirect,
		TargetUUID:      targetUUID,
		MessageType:     msgType,
		Content:         content,
		SentAt:          time.Now().UTC(),
	}

	if s.events == nil {
		return s.persistLocalMessage(message, "persist direct message")
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
	conversationKey := model.DirectConversationKey(currentUserUUID, targetUUID)

	targetUser, err := s.userFinder.GetByUUID(targetUUID)
	if err != nil {
		return nil, fmt.Errorf("find target user in list direct messages: %w", err)
	}
	if targetUser == nil {
		return nil, ErrMessageTargetNotFound
	}
	if !targetUser.IsAssistant() {
		areFriends, err := s.friendChecker.CanSendDirectMessage(strings.TrimSpace(currentUserUUID), targetUUID)
		if err != nil {
			return nil, fmt.Errorf("check friendship in list direct messages: %w", err)
		}
		if !areFriends {
			hasHistory, err := s.repo.HasConversationMessages(conversationKey)
			if err != nil {
				return nil, fmt.Errorf("check direct conversation history: %w", err)
			}
			if !hasHistory {
				return nil, ErrMessageFriendRequired
			}
		}
	}

	messages, err := s.repo.ListByConversationKey(
		conversationKey,
		beforeID,
		normalizeMessageListLimit(limit),
	)
	if err != nil {
		return nil, fmt.Errorf("list direct messages: %w", err)
	}

	return messages, nil
}

func (s *MessageService) SendGroupMessage(senderUUID, groupUUID, content, clientMessageID string) (*model.Message, []string, error) {
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

	if err := s.ensureWritableGroupMessagePermission(senderUUID, groupUUID); err != nil {
		return nil, nil, err
	}

	recipientUUIDs, err := s.listGroupMemberUUIDs(groupUUID)
	if err != nil {
		return nil, nil, err
	}

	message := &model.Message{
		UUID:            generateMessageUUID(),
		ClientMessageID: normalizeClientMessageID(clientMessageID),
		ConversationKey: model.GroupConversationKey(groupUUID),
		SenderUUID:      strings.TrimSpace(senderUUID),
		TargetType:      model.MessageTargetGroup,
		TargetUUID:      groupUUID,
		MessageType:     model.MessageTypeText,
		Content:         content,
		SentAt:          time.Now().UTC(),
	}

	if s.events == nil {
		persisted, persistErr := s.persistLocalMessage(message, "persist group message")
		if persistErr != nil {
			return nil, nil, persistErr
		}
		s.observeGroupHeat(groupUUID, len(recipientUUIDs))
		return persisted, recipientUUIDs, nil
	}

	if err := s.publishMessageRequested("message.group.send_requested", message, recipientUUIDs); err != nil {
		return nil, nil, err
	}
	s.observeGroupHeat(groupUUID, len(recipientUUIDs))

	return message, recipientUUIDs, nil
}

func (s *MessageService) SendDirectFileMessage(senderUUID, targetUUID, fileUUID, clientMessageID string) (*model.Message, error) {
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

	message, err := s.newFileMessage(senderUUID, targetUUID, model.MessageTargetDirect, fileUUID, clientMessageID)
	if err != nil {
		return nil, err
	}
	if s.events == nil {
		return s.persistLocalMessage(message, "persist direct file message")
	}
	if err := s.publishMessageRequested("message.direct.send_requested", message, nil); err != nil {
		return nil, err
	}

	return message, nil
}

func (s *MessageService) SendGroupFileMessage(senderUUID, groupUUID, fileUUID, clientMessageID string) (*model.Message, []string, error) {
	groupUUID = strings.TrimSpace(groupUUID)
	fileUUID = strings.TrimSpace(fileUUID)
	if groupUUID == "" {
		return nil, nil, ErrMessageTargetRequired
	}
	if fileUUID == "" {
		return nil, nil, ErrMessageFileRequired
	}
	if err := s.ensureWritableGroupMessagePermission(senderUUID, groupUUID); err != nil {
		return nil, nil, err
	}

	recipientUUIDs, err := s.listGroupMemberUUIDs(groupUUID)
	if err != nil {
		return nil, nil, err
	}

	message, err := s.newFileMessage(senderUUID, groupUUID, model.MessageTargetGroup, fileUUID, clientMessageID)
	if err != nil {
		return nil, nil, err
	}
	if s.events == nil {
		persisted, persistErr := s.persistLocalMessage(message, "persist group file message")
		if persistErr != nil {
			return nil, nil, persistErr
		}
		s.observeGroupHeat(groupUUID, len(recipientUUIDs))
		return persisted, recipientUUIDs, nil
	}
	if err := s.publishMessageRequested("message.group.send_requested", message, recipientUUIDs); err != nil {
		return nil, nil, err
	}
	s.observeGroupHeat(groupUUID, len(recipientUUIDs))

	return message, recipientUUIDs, nil
}

func (s *MessageService) ListGroupMessages(currentUserUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	groupUUID = strings.TrimSpace(groupUUID)
	if groupUUID == "" {
		return nil, ErrMessageTargetRequired
	}
	if err := s.ensureReadableGroupMessagePermission(currentUserUUID, groupUUID); err != nil {
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

func (s *MessageService) ListGroupMessagesAfter(currentUserUUID, groupUUID string, afterID uint, limit int) ([]*model.Message, error) {
	groupUUID = strings.TrimSpace(groupUUID)
	if groupUUID == "" {
		return nil, ErrMessageTargetRequired
	}
	if err := s.ensureReadableGroupMessagePermission(currentUserUUID, groupUUID); err != nil {
		return nil, err
	}

	normalizedLimit := normalizeMessageListLimit(limit)
	cacheKey := platformCache.HotGroupMessagesKey(groupUUID, afterID, normalizedLimit)
	if cached := s.loadCachedGroupMessagesPage(cacheKey); len(cached) > 0 {
		return cached, nil
	}
	sfKey := fmt.Sprintf("group_pull:%s:%d:%d", groupUUID, afterID, normalizedLimit)
	// 热群 notify 会让同一批成员在极短时间内拉取同一个 after_id 页面。
	// 这里先查短 TTL Redis 缓存，再用 singleflight 合并本机内 miss，
	// 可以把“同页重复补拉”的压力同时挡在 Redis 和单机内存层。
	value, err, _ := s.groupPulls.Do(sfKey, func() (any, error) {
		if cached := s.loadCachedGroupMessagesPage(cacheKey); len(cached) > 0 {
			return cached, nil
		}

		messages, listErr := s.repo.ListByConversationKeyAfter(
			model.GroupConversationKey(groupUUID),
			afterID,
			normalizedLimit,
		)
		if listErr != nil {
			return nil, fmt.Errorf("list group messages after: %w", listErr)
		}

		cloned := cloneMessageSlice(messages)
		s.storeCachedGroupMessagesPage(cacheKey, cloned)
		return cloned, nil
	})
	if err != nil {
		return nil, err
	}

	messages, ok := value.([]*model.Message)
	if !ok {
		return nil, fmt.Errorf("list group messages after: unexpected singleflight result %T", value)
	}

	return cloneMessageSlice(messages), nil
}

func (s *MessageService) loadCachedGroupMessagesPage(cacheKey string) []*model.Message {
	ctx, cancel := platformCache.NewContext()
	defer cancel()

	var messages []*model.Message
	hit, err := platformCache.GetJSON(ctx, cacheKey, &messages)
	if err != nil || !hit {
		return nil
	}

	return cloneMessageSlice(messages)
}

func (s *MessageService) storeCachedGroupMessagesPage(cacheKey string, messages []*model.Message) {
	ctx, cancel := platformCache.NewContext()
	defer cancel()

	ttl := platformCache.HotGroupMessagesTTL
	if len(messages) == 0 {
		ttl = platformCache.HotGroupEmptyTTL
	}

	_ = platformCache.SetJSON(ctx, cacheKey, cloneMessageSlice(messages), ttl)
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

	outboxEvent, err := buildMessageCreatedOutboxEvent(message, payload.RecipientUUIDs)
	if err != nil {
		return nil, fmt.Errorf("build message created outbox event: %w", err)
	}

	if err := s.repo.StoreWithOutbox(message, outboxEvent); err != nil {
		if !isDuplicateMessageError(err) {
			return nil, fmt.Errorf("persist requested message: %w", err)
		}

		existing, findErr := s.findExistingMessageForDuplicate(message)
		if findErr != nil {
			return nil, fmt.Errorf("find duplicate message: %w", findErr)
		}
		if existing == nil {
			return nil, fmt.Errorf("duplicate message %s/%s not found after conflict", message.UUID, message.ClientMessageID)
		}
		message = existing
		outboxEvent, err = buildMessageCreatedOutboxEvent(message, payload.RecipientUUIDs)
		if err != nil {
			return nil, fmt.Errorf("rebuild message created outbox event: %w", err)
		}
		// Redelivery after a partial failure should still recreate the outbox row when it is missing.
		// The unique key on (aggregate_type, aggregate_id, event_type) keeps this idempotent.
		if err := s.repo.EnsureOutbox(outboxEvent); err != nil {
			return nil, fmt.Errorf("ensure outbox for duplicate message: %w", err)
		}
	}

	return message, nil
}

func (s *MessageService) persistLocalMessage(message *model.Message, action string) (*model.Message, error) {
	if err := s.repo.Create(message); err != nil {
		if !isDuplicateMessageError(err) {
			return nil, fmt.Errorf("%s: %w", action, err)
		}

		existing, findErr := s.findExistingMessageForDuplicate(message)
		if findErr != nil {
			return nil, fmt.Errorf("%s: find duplicate message: %w", action, findErr)
		}
		if existing == nil {
			return nil, fmt.Errorf("%s: duplicate message %s/%s not found after conflict", action, message.UUID, message.ClientMessageID)
		}
		return existing, nil
	}

	return message, nil
}

func (s *MessageService) findExistingMessageForDuplicate(message *model.Message) (*model.Message, error) {
	if message == nil {
		return nil, nil
	}
	if message.ClientMessageID != "" {
		existing, err := s.repo.GetBySenderAndClientMessageID(message.SenderUUID, message.ClientMessageID)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
	}

	return s.repo.GetByUUID(message.UUID)
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
		ClientMessageID: message.ClientMessageID,
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

func buildMessageCreatedOutboxEvent(message *model.Message, recipientUUIDs []string) (*model.OutboxEvent, error) {
	if message == nil {
		return nil, nil
	}

	topic := createdTopicForTargetType(message.TargetType)
	eventType := topic
	payload := messageToEventPayload(message, recipientUUIDs)
	envelope, err := platformKafka.NewEnvelope(eventType, payload)
	if err != nil {
		return nil, fmt.Errorf("create message created envelope: %w", err)
	}

	value, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshal message created envelope: %w", err)
	}

	headers, err := json.Marshal(map[string]string{
		"event_type": envelope.EventType,
		"version":    envelope.Version,
		"source":     envelope.Source,
		"event_id":   envelope.EventID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal outbox headers: %w", err)
	}

	now := time.Now().UTC()
	return &model.OutboxEvent{
		AggregateType: "message",
		AggregateID:   message.UUID,
		EventType:     eventType,
		Topic:         topic,
		MessageKey:    message.UUID,
		Value:         value,
		HeadersJSON:   headers,
		Status:        model.OutboxStatusPending,
		NextRetryAt:   &now,
	}, nil
}

func payloadToMessage(payload MessageEventPayload) *model.Message {
	return &model.Message{
		UUID:            strings.TrimSpace(payload.MessageID),
		ClientMessageID: normalizeClientMessageID(payload.ClientMessageID),
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

	if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") {
		return true
	}

	return false
}

func (s *MessageService) observeGroupHeat(groupUUID string, memberCount int) {
	if s == nil || s.hotGroups == nil {
		return
	}

	// 热度统计只作为投递策略信号，不阻塞主消息链路。
	// Redis 短暂异常时，消息仍然按普通群路径继续走。
	_, _ = s.hotGroups.ObserveMessage(groupUUID, memberCount)
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

func (s *MessageService) newFileMessage(senderUUID, targetUUID string, targetType int8, fileUUID string, clientMessageID string) (*model.Message, error) {
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
		ClientMessageID: normalizeClientMessageID(clientMessageID),
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

func (s *MessageService) ensureReadableGroupMessagePermission(userUUID, groupUUID string) error {
	if s.groupChecker == nil {
		return ErrMessageTargetNotFound
	}

	group, err := s.groupChecker.GetByUUID(strings.TrimSpace(groupUUID))
	if err != nil {
		return fmt.Errorf("check group in message permission: %w", err)
	}
	if group == nil {
		return ErrMessageTargetNotFound
	}
	if group.Status != model.GroupStatusNormal && group.Status != model.GroupStatusDismissed {
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

func (s *MessageService) ensureWritableGroupMessagePermission(userUUID, groupUUID string) error {
	groupUUID = strings.TrimSpace(groupUUID)
	if s.groupChecker == nil {
		return ErrMessageTargetNotFound
	}

	group, err := s.groupChecker.GetByUUID(groupUUID)
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

func cloneMessageSlice(messages []*model.Message) []*model.Message {
	if len(messages) == 0 {
		return nil
	}

	cloned := make([]*model.Message, len(messages))
	copy(cloned, messages)
	return cloned
}

func generateMessageUUID() string {
	return idgen.MessageID()
}

func normalizeClientMessageID(clientMessageID string) string {
	normalized := strings.TrimSpace(clientMessageID)
	if normalized != "" {
		return normalized
	}

	return generateClientMessageID()
}

func generateClientMessageID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("generate client message id: %w", err))
	}

	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80

	raw := hex.EncodeToString(buf)
	return fmt.Sprintf("%s-%s-%s-%s-%s", raw[:8], raw[8:12], raw[12:16], raw[16:20], raw[20:])
}
