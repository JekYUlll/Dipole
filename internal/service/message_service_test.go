package service

import (
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

func (c *stubFriendshipChecker) AreFriends(userUUID, friendUUID string) (bool, error) {
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
	})

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
	})

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
	})

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
	})

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
	}, &stubFriendshipChecker{})

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
	})

	_, err := service.ListDirectMessages("U100", "U200", 0, 20)
	if !errors.Is(err, ErrMessageFriendRequired) {
		t.Fatalf("expected ErrMessageFriendRequired, got %v", err)
	}
}
