package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/JekYUlll/Dipole/internal/model"
)

type stubMessageRepository struct {
	createErr           error
	listErr             error
	createdMessages     []*model.Message
	listMessages        []*model.Message
	lastConversationKey string
	lastBeforeID        uint
	lastLimit           int
}

func (r *stubMessageRepository) Create(message *model.Message) error {
	if r.createErr != nil {
		return r.createErr
	}

	r.createdMessages = append(r.createdMessages, message)
	return nil
}

func (r *stubMessageRepository) ListByConversationKey(conversationKey string, beforeID uint, limit int) ([]*model.Message, error) {
	r.lastConversationKey = conversationKey
	r.lastBeforeID = beforeID
	r.lastLimit = limit
	if r.listErr != nil {
		return nil, r.listErr
	}

	return r.listMessages, nil
}

type stubFriendshipChecker struct {
	friendships map[string]map[string]bool
	err         error
}

func (c *stubFriendshipChecker) CanSendDirectMessage(userUUID, friendUUID string) (bool, error) {
	if c.err != nil {
		return false, c.err
	}
	if c.friendships == nil {
		return false, nil
	}

	return c.friendships[userUUID][friendUUID], nil
}

type stubMessageUserFinder struct {
	users map[string]*model.User
	err   error
}

func (f *stubMessageUserFinder) GetByUUID(uuid string) (*model.User, error) {
	if f.err != nil {
		return nil, f.err
	}

	return f.users[uuid], nil
}

type stubGroupMessageChecker struct {
	groups  map[string]*model.Group
	members map[string]map[string]*model.GroupMember
	err     error
}

func (c *stubGroupMessageChecker) GetByUUID(groupUUID string) (*model.Group, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.groups[groupUUID], nil
}

func (c *stubGroupMessageChecker) GetMember(groupUUID, userUUID string) (*model.GroupMember, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.members[groupUUID][userUUID], nil
}

func (c *stubGroupMessageChecker) ListMembers(groupUUID string) ([]*model.GroupMember, error) {
	if c.err != nil {
		return nil, c.err
	}
	members := make([]*model.GroupMember, 0, len(c.members[groupUUID]))
	for _, member := range c.members[groupUUID] {
		members = append(members, member)
	}
	return members, nil
}

type stubEventPublisher struct {
	topics []string
	keys   []string
}

func (p *stubEventPublisher) PublishJSON(_ context.Context, topic string, key string, payload any, headers map[string]string) error {
	p.topics = append(p.topics, topic)
	p.keys = append(p.keys, key)
	_ = payload
	_ = headers
	return nil
}

func TestMessageServiceSendDirectMessageSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubMessageRepository{}
	userFinder := &stubMessageUserFinder{
		users: map[string]*model.User{
			"U200": {UUID: "U200", Status: model.UserStatusNormal},
		},
	}
	service := NewMessageService(repo, userFinder, &stubFriendshipChecker{
		friendships: map[string]map[string]bool{
			"U100": {"U200": true},
		},
	}, nil, nil)

	message, err := service.SendDirectMessage("U100", " U200 ", " hello world ")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(repo.createdMessages) != 1 {
		t.Fatalf("expected one persisted message, got %d", len(repo.createdMessages))
	}
	if message.UUID == "" || !strings.HasPrefix(message.UUID, "M") {
		t.Fatalf("expected generated message uuid, got %s", message.UUID)
	}
	if message.ConversationKey != model.DirectConversationKey("U100", "U200") {
		t.Fatalf("unexpected conversation key: %s", message.ConversationKey)
	}
	if message.SenderUUID != "U100" {
		t.Fatalf("expected sender U100, got %s", message.SenderUUID)
	}
	if message.TargetUUID != "U200" {
		t.Fatalf("expected target U200, got %s", message.TargetUUID)
	}
	if message.Content != "hello world" {
		t.Fatalf("expected trimmed content, got %q", message.Content)
	}
	if message.MessageType != model.MessageTypeText {
		t.Fatalf("expected text message type, got %d", message.MessageType)
	}
}

func TestMessageServiceSendDirectMessageRejectsUnavailableTarget(t *testing.T) {
	t.Parallel()

	repo := &stubMessageRepository{}
	userFinder := &stubMessageUserFinder{
		users: map[string]*model.User{
			"U200": {UUID: "U200", Status: model.UserStatusDisabled},
		},
	}
	service := NewMessageService(repo, userFinder, &stubFriendshipChecker{
		friendships: map[string]map[string]bool{
			"U100": {"U200": true},
		},
	}, nil, nil)

	_, err := service.SendDirectMessage("U100", "U200", "hello")
	if !errors.Is(err, ErrMessageTargetUnavailable) {
		t.Fatalf("expected ErrMessageTargetUnavailable, got %v", err)
	}
	if len(repo.createdMessages) != 0 {
		t.Fatalf("expected no persisted message, got %d", len(repo.createdMessages))
	}
}

func TestMessageServiceSendDirectMessageRejectsNonFriend(t *testing.T) {
	t.Parallel()

	repo := &stubMessageRepository{}
	userFinder := &stubMessageUserFinder{
		users: map[string]*model.User{
			"U200": {UUID: "U200", Status: model.UserStatusNormal},
		},
	}
	service := NewMessageService(repo, userFinder, &stubFriendshipChecker{
		friendships: map[string]map[string]bool{
			"U100": {},
		},
	}, nil, nil)

	_, err := service.SendDirectMessage("U100", "U200", "hello")
	if !errors.Is(err, ErrMessageFriendRequired) {
		t.Fatalf("expected ErrMessageFriendRequired, got %v", err)
	}
	if len(repo.createdMessages) != 0 {
		t.Fatalf("expected no persisted message, got %d", len(repo.createdMessages))
	}
}

func TestMessageServiceListDirectMessagesSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubMessageRepository{
		listMessages: []*model.Message{
			{ID: 10, UUID: "M10"},
			{ID: 11, UUID: "M11"},
		},
	}
	userFinder := &stubMessageUserFinder{
		users: map[string]*model.User{
			"U200": {UUID: "U200", Status: model.UserStatusDisabled},
		},
	}
	service := NewMessageService(repo, userFinder, &stubFriendshipChecker{
		friendships: map[string]map[string]bool{
			"U100": {"U200": true},
		},
	}, nil, nil)

	messages, err := service.ListDirectMessages("U100", "U200", 99, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if repo.lastConversationKey != model.DirectConversationKey("U100", "U200") {
		t.Fatalf("unexpected conversation key: %s", repo.lastConversationKey)
	}
	if repo.lastBeforeID != 99 {
		t.Fatalf("expected before id 99, got %d", repo.lastBeforeID)
	}
	if repo.lastLimit != 10 {
		t.Fatalf("expected limit 10, got %d", repo.lastLimit)
	}
}

func TestMessageServiceListDirectMessagesRejectsMissingTarget(t *testing.T) {
	t.Parallel()

	service := NewMessageService(&stubMessageRepository{}, &stubMessageUserFinder{
		users: map[string]*model.User{},
	}, &stubFriendshipChecker{}, nil, nil)

	_, err := service.ListDirectMessages("U100", "U404", 0, 20)
	if !errors.Is(err, ErrMessageTargetNotFound) {
		t.Fatalf("expected ErrMessageTargetNotFound, got %v", err)
	}
}

func TestMessageServiceListDirectMessagesRejectsNonFriend(t *testing.T) {
	t.Parallel()

	service := NewMessageService(&stubMessageRepository{}, &stubMessageUserFinder{
		users: map[string]*model.User{
			"U200": {UUID: "U200", Status: model.UserStatusNormal},
		},
	}, &stubFriendshipChecker{
		friendships: map[string]map[string]bool{
			"U100": {},
		},
	}, nil, nil)

	_, err := service.ListDirectMessages("U100", "U200", 0, 20)
	if !errors.Is(err, ErrMessageFriendRequired) {
		t.Fatalf("expected ErrMessageFriendRequired, got %v", err)
	}
}

func TestMessageServiceSendGroupMessageSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubMessageRepository{}
	service := NewMessageService(repo, &stubMessageUserFinder{}, nil, &stubGroupMessageChecker{
		groups: map[string]*model.Group{
			"G100": {UUID: "G100", Status: model.GroupStatusNormal},
		},
		members: map[string]map[string]*model.GroupMember{
			"G100": {
				"U100": {GroupUUID: "G100", UserUUID: "U100"},
				"U200": {GroupUUID: "G100", UserUUID: "U200"},
			},
		},
	}, nil)

	message, recipients, err := service.SendGroupMessage("U100", "G100", "hello group")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if message.TargetType != model.MessageTargetGroup {
		t.Fatalf("expected group target type, got %d", message.TargetType)
	}
	if message.ConversationKey != model.GroupConversationKey("G100") {
		t.Fatalf("unexpected conversation key: %s", message.ConversationKey)
	}
	if len(recipients) != 2 {
		t.Fatalf("expected 2 recipients, got %d", len(recipients))
	}
}

func TestMessageServiceSendGroupMessageRejectsNonMember(t *testing.T) {
	t.Parallel()

	service := NewMessageService(&stubMessageRepository{}, &stubMessageUserFinder{}, nil, &stubGroupMessageChecker{
		groups: map[string]*model.Group{
			"G100": {UUID: "G100", Status: model.GroupStatusNormal},
		},
		members: map[string]map[string]*model.GroupMember{
			"G100": {},
		},
	}, nil)

	_, _, err := service.SendGroupMessage("U100", "G100", "hello")
	if !errors.Is(err, ErrMessageGroupForbidden) {
		t.Fatalf("expected ErrMessageGroupForbidden, got %v", err)
	}
}

func TestMessageServiceListGroupMessagesSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubMessageRepository{
		listMessages: []*model.Message{
			{ID: 10, UUID: "M10", TargetType: model.MessageTargetGroup},
		},
	}
	service := NewMessageService(repo, &stubMessageUserFinder{}, nil, &stubGroupMessageChecker{
		groups: map[string]*model.Group{
			"G100": {UUID: "G100", Status: model.GroupStatusNormal},
		},
		members: map[string]map[string]*model.GroupMember{
			"G100": {
				"U100": {GroupUUID: "G100", UserUUID: "U100"},
			},
		},
	}, nil)

	messages, err := service.ListGroupMessages("U100", "G100", 15, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if repo.lastConversationKey != model.GroupConversationKey("G100") {
		t.Fatalf("unexpected conversation key: %s", repo.lastConversationKey)
	}
}

func TestMessageServicePublishesKafkaEventOnDirectMessage(t *testing.T) {
	t.Parallel()

	repo := &stubMessageRepository{}
	publisher := &stubEventPublisher{}
	service := NewMessageService(repo, &stubMessageUserFinder{
		users: map[string]*model.User{
			"U200": {UUID: "U200", Status: model.UserStatusNormal},
		},
	}, &stubFriendshipChecker{
		friendships: map[string]map[string]bool{
			"U100": {"U200": true},
		},
	}, nil, publisher)

	if _, err := service.SendDirectMessage("U100", "U200", "hello"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(publisher.topics) != 1 || publisher.topics[0] != "message.direct.created" {
		t.Fatalf("expected direct message event, got %+v", publisher.topics)
	}
}
