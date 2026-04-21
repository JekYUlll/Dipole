package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
	platformHotGroup "github.com/JekYUlll/Dipole/internal/platform/hotgroup"
	"github.com/JekYUlll/Dipole/internal/store"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type stubMessageRepository struct {
	mu                  sync.Mutex
	createErr           error
	storeWithOutboxErr  error
	ensureOutboxErr     error
	listErr             error
	createdMessages     []*model.Message
	outboxEvents        []*model.OutboxEvent
	ensuredOutboxEvents []*model.OutboxEvent
	listMessages        []*model.Message
	listAfterMessages   []*model.Message
	offlineMessages     []*model.Message
	messagesByUUID      map[string]*model.Message
	hasConversation     bool
	lastConversationKey string
	lastBeforeID        uint
	lastAfterID         uint
	lastLimit           int
	lastUserUUID        string
	listAfterCallCount  int
	listAfterDelay      time.Duration
}

func (r *stubMessageRepository) Create(message *model.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.createErr != nil {
		return r.createErr
	}

	r.createdMessages = append(r.createdMessages, message)
	if r.messagesByUUID == nil {
		r.messagesByUUID = make(map[string]*model.Message)
	}
	r.messagesByUUID[message.UUID] = message
	return nil
}

func (r *stubMessageRepository) GetByUUID(uuid string) (*model.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.messagesByUUID == nil {
		return nil, nil
	}

	return r.messagesByUUID[uuid], nil
}

func (r *stubMessageRepository) HasConversationMessages(conversationKey string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.lastConversationKey = conversationKey
	if r.listErr != nil {
		return false, r.listErr
	}
	return r.hasConversation, nil
}

func (r *stubMessageRepository) StoreWithOutbox(message *model.Message, event *model.OutboxEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.storeWithOutboxErr != nil {
		return r.storeWithOutboxErr
	}

	r.createdMessages = append(r.createdMessages, message)
	if r.messagesByUUID == nil {
		r.messagesByUUID = make(map[string]*model.Message)
	}
	r.messagesByUUID[message.UUID] = message
	r.outboxEvents = append(r.outboxEvents, event)
	return nil
}

func (r *stubMessageRepository) EnsureOutbox(event *model.OutboxEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.ensureOutboxErr != nil {
		return r.ensureOutboxErr
	}

	r.ensuredOutboxEvents = append(r.ensuredOutboxEvents, event)
	return nil
}

func (r *stubMessageRepository) ListByConversationKey(conversationKey string, beforeID uint, limit int) ([]*model.Message, error) {
	r.mu.Lock()
	r.lastConversationKey = conversationKey
	r.lastBeforeID = beforeID
	r.lastLimit = limit
	err := r.listErr
	messages := r.listMessages
	r.mu.Unlock()
	if err != nil {
		return nil, err
	}

	return messages, nil
}

func (r *stubMessageRepository) ListByConversationKeyAfter(conversationKey string, afterID uint, limit int) ([]*model.Message, error) {
	r.mu.Lock()
	r.lastConversationKey = conversationKey
	r.lastAfterID = afterID
	r.lastLimit = limit
	r.listAfterCallCount++
	err := r.listErr
	messages := r.listAfterMessages
	delay := r.listAfterDelay
	r.mu.Unlock()
	if delay > 0 {
		time.Sleep(delay)
	}
	if err != nil {
		return nil, err
	}

	return messages, nil
}

func (r *stubMessageRepository) ListOfflineByUserUUID(userUUID string, afterID uint, limit int) ([]*model.Message, error) {
	r.mu.Lock()
	r.lastUserUUID = userUUID
	r.lastAfterID = afterID
	r.lastLimit = limit
	err := r.listErr
	messages := r.offlineMessages
	r.mu.Unlock()
	if err != nil {
		return nil, err
	}

	return messages, nil
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

type stubMessageFileFinder struct {
	files map[string]*model.UploadedFile
	err   error
}

func (f *stubMessageFileFinder) GetOwnedFile(uploaderUUID, fileUUID string) (*model.UploadedFile, error) {
	if f.err != nil {
		return nil, f.err
	}
	file := f.files[fileUUID]
	if file == nil {
		return nil, ErrFileNotFound
	}
	if file.UploaderUUID != uploaderUUID {
		return nil, ErrFilePermissionDenied
	}
	return file, nil
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
	topics     []string
	keys       []string
	eventTypes []string
	payloads   []any
}

type stubHotGroupObserver struct {
	groupUUIDs   []string
	memberCounts []int
	status       platformHotGroup.Status
	err          error
}

func (o *stubHotGroupObserver) ObserveMessage(groupUUID string, memberCount int) (platformHotGroup.Status, error) {
	o.groupUUIDs = append(o.groupUUIDs, groupUUID)
	o.memberCounts = append(o.memberCounts, memberCount)
	return o.status, o.err
}

func (p *stubEventPublisher) PublishJSON(_ context.Context, topic string, key string, payload any, headers map[string]string) error {
	p.topics = append(p.topics, topic)
	p.keys = append(p.keys, key)
	p.payloads = append(p.payloads, payload)
	_ = headers
	return nil
}

func (p *stubEventPublisher) PublishEvent(_ context.Context, topic string, key string, eventType string, payload any, headers map[string]string) error {
	p.topics = append(p.topics, topic)
	p.keys = append(p.keys, key)
	p.eventTypes = append(p.eventTypes, eventType)
	p.payloads = append(p.payloads, payload)
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
	}, nil, nil, nil, nil)

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
	}, nil, nil, nil, nil)

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
	}, nil, nil, nil, nil)

	_, err := service.SendDirectMessage("U100", "U200", "hello")
	if !errors.Is(err, ErrMessageFriendRequired) {
		t.Fatalf("expected ErrMessageFriendRequired, got %v", err)
	}
	if len(repo.createdMessages) != 0 {
		t.Fatalf("expected no persisted message, got %d", len(repo.createdMessages))
	}
}

func TestMessageServiceSendDirectMessageAllowsAssistantTarget(t *testing.T) {
	t.Parallel()

	repo := &stubMessageRepository{}
	userFinder := &stubMessageUserFinder{
		users: map[string]*model.User{
			"UAI": {UUID: "UAI", Status: model.UserStatusNormal, UserType: model.UserTypeAssistant},
		},
	}
	service := NewMessageService(repo, userFinder, &stubFriendshipChecker{
		friendships: map[string]map[string]bool{
			"U100": {},
		},
	}, nil, nil, nil, nil)

	message, err := service.SendDirectMessage("U100", "UAI", "hello ai")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if message.TargetUUID != "UAI" {
		t.Fatalf("expected assistant target UAI, got %s", message.TargetUUID)
	}
	if len(repo.createdMessages) != 1 {
		t.Fatalf("expected assistant direct message to persist, got %d", len(repo.createdMessages))
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
	}, nil, nil, nil, nil)

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
	}, &stubFriendshipChecker{}, nil, nil, nil, nil)

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
	}, nil, nil, nil, nil)

	_, err := service.ListDirectMessages("U100", "U200", 0, 20)
	if !errors.Is(err, ErrMessageFriendRequired) {
		t.Fatalf("expected ErrMessageFriendRequired, got %v", err)
	}
}

func TestMessageServiceListDirectMessagesAllowsHistoryAfterFriendDeleted(t *testing.T) {
	t.Parallel()

	repo := &stubMessageRepository{
		hasConversation: true,
		listMessages: []*model.Message{
			{ID: 1, UUID: "M1"},
		},
	}
	service := NewMessageService(repo, &stubMessageUserFinder{
		users: map[string]*model.User{
			"U200": {UUID: "U200", Status: model.UserStatusNormal},
		},
	}, &stubFriendshipChecker{
		friendships: map[string]map[string]bool{
			"U100": {},
		},
	}, nil, nil, nil, nil)

	messages, err := service.ListDirectMessages("U100", "U200", 0, 20)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
}

func TestMessageServiceListDirectMessagesAllowsAssistantTarget(t *testing.T) {
	t.Parallel()

	repo := &stubMessageRepository{
		listMessages: []*model.Message{
			{ID: 1, UUID: "M1"},
		},
	}
	service := NewMessageService(repo, &stubMessageUserFinder{
		users: map[string]*model.User{
			"UAI": {UUID: "UAI", Status: model.UserStatusNormal, UserType: model.UserTypeAssistant},
		},
	}, &stubFriendshipChecker{
		friendships: map[string]map[string]bool{
			"U100": {},
		},
	}, nil, nil, nil, nil)

	messages, err := service.ListDirectMessages("U100", "UAI", 0, 20)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
}

func TestMessageServiceSendSystemDirectMessageSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubMessageRepository{}
	userFinder := &stubMessageUserFinder{
		users: map[string]*model.User{
			"UAI":  {UUID: "UAI", Status: model.UserStatusNormal, UserType: model.UserTypeAssistant},
			"U100": {UUID: "U100", Status: model.UserStatusNormal},
		},
	}
	service := NewMessageService(repo, userFinder, &stubFriendshipChecker{}, nil, nil, nil, nil)

	message, err := service.SendSystemDirectMessage("UAI", "U100", "system notice")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if message == nil {
		t.Fatal("expected message")
	}
	if message.MessageType != model.MessageTypeSystem {
		t.Fatalf("expected system message type, got %d", message.MessageType)
	}
	if message.TargetUUID != "U100" || message.SenderUUID != "UAI" {
		t.Fatalf("unexpected participants: %+v", message)
	}
}

func TestMessageServiceSendGroupMessageSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubMessageRepository{}
	observer := &stubHotGroupObserver{}
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
	}, nil, nil, observer)

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
	if len(observer.groupUUIDs) != 1 || observer.groupUUIDs[0] != "G100" {
		t.Fatalf("expected group heat observer for G100, got %+v", observer.groupUUIDs)
	}
	if len(observer.memberCounts) != 1 || observer.memberCounts[0] != 2 {
		t.Fatalf("expected group heat member count 2, got %+v", observer.memberCounts)
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
	}, nil, nil, nil)

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
	}, nil, nil, nil)

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

func TestMessageServiceListGroupMessagesAllowsDismissedGroup(t *testing.T) {
	t.Parallel()

	repo := &stubMessageRepository{
		listMessages: []*model.Message{
			{ID: 10, UUID: "M10", TargetType: model.MessageTargetGroup},
		},
	}
	service := NewMessageService(repo, &stubMessageUserFinder{}, nil, &stubGroupMessageChecker{
		groups: map[string]*model.Group{
			"G100": {UUID: "G100", Status: model.GroupStatusDismissed},
		},
		members: map[string]map[string]*model.GroupMember{
			"G100": {
				"U100": {GroupUUID: "G100", UserUUID: "U100"},
			},
		},
	}, nil, nil, nil)

	messages, err := service.ListGroupMessages("U100", "G100", 15, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
}

func TestMessageServiceListGroupMessagesAfterSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubMessageRepository{
		listAfterMessages: []*model.Message{
			{ID: 11, UUID: "M11", TargetType: model.MessageTargetGroup},
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
	}, nil, nil, nil)

	messages, err := service.ListGroupMessagesAfter("U100", "G100", 10, 20)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if repo.lastConversationKey != model.GroupConversationKey("G100") {
		t.Fatalf("unexpected conversation key: %s", repo.lastConversationKey)
	}
	if repo.lastAfterID != 10 {
		t.Fatalf("expected after id 10, got %d", repo.lastAfterID)
	}
}

func TestMessageServiceSendGroupMessageRejectsDismissedGroup(t *testing.T) {
	t.Parallel()

	service := NewMessageService(&stubMessageRepository{}, &stubMessageUserFinder{}, nil, &stubGroupMessageChecker{
		groups: map[string]*model.Group{
			"G100": {UUID: "G100", Status: model.GroupStatusDismissed},
		},
		members: map[string]map[string]*model.GroupMember{
			"G100": {
				"U100": {GroupUUID: "G100", UserUUID: "U100"},
			},
		},
	}, nil, nil, nil)

	_, _, err := service.SendGroupMessage("U100", "G100", "hello")
	if !errors.Is(err, ErrMessageTargetNotFound) {
		t.Fatalf("expected ErrMessageTargetNotFound, got %v", err)
	}
}

func TestMessageServiceListGroupMessagesAfterUsesSingleflight(t *testing.T) {
	t.Parallel()

	repo := &stubMessageRepository{
		listAfterMessages: []*model.Message{
			{ID: 11, UUID: "M11", TargetType: model.MessageTargetGroup},
		},
		listAfterDelay: 50 * time.Millisecond,
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
	}, nil, nil, nil)

	const workers = 8
	results := make(chan []*model.Message, workers)
	errs := make(chan error, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			messages, err := service.ListGroupMessagesAfter("U100", "G100", 10, 20)
			if err != nil {
				errs <- err
				return
			}
			results <- messages
		}()
	}

	close(start)
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	}

	count := 0
	for messages := range results {
		count++
		if len(messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(messages))
		}
	}
	if count != workers {
		t.Fatalf("expected %d successful results, got %d", workers, count)
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.listAfterCallCount != 1 {
		t.Fatalf("expected singleflight to collapse calls to 1, got %d", repo.listAfterCallCount)
	}
}

func TestMessageServiceListGroupMessagesAfterUsesRedisCache(t *testing.T) {
	cleanup := setupMessageServiceRedisTest(t)
	defer cleanup()

	repo := &stubMessageRepository{
		listAfterMessages: []*model.Message{
			{ID: 11, UUID: "M11", TargetType: model.MessageTargetGroup},
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
	}, nil, nil, nil)

	first, err := service.ListGroupMessagesAfter("U100", "G100", 10, 20)
	if err != nil {
		t.Fatalf("expected no error on first read, got %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("expected 1 message on first read, got %d", len(first))
	}

	repo.mu.Lock()
	repo.listAfterMessages = []*model.Message{
		{ID: 12, UUID: "M12", TargetType: model.MessageTargetGroup},
	}
	repo.mu.Unlock()

	second, err := service.ListGroupMessagesAfter("U100", "G100", 10, 20)
	if err != nil {
		t.Fatalf("expected no error on cached read, got %v", err)
	}
	if len(second) != 1 || second[0].UUID != "M11" {
		t.Fatalf("expected cached message M11, got %+v", second)
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.listAfterCallCount != 1 {
		t.Fatalf("expected redis cache to avoid second repository call, got %d", repo.listAfterCallCount)
	}
}

func TestMessageServiceListOfflineMessagesSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubMessageRepository{
		offlineMessages: []*model.Message{
			{ID: 31, UUID: "M31"},
			{ID: 32, UUID: "M32"},
		},
	}
	service := NewMessageService(repo, &stubMessageUserFinder{}, nil, nil, nil, nil, nil)

	messages, err := service.ListOfflineMessages(" U100 ", 30, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 offline messages, got %d", len(messages))
	}
	if repo.lastUserUUID != "U100" {
		t.Fatalf("expected user uuid U100, got %s", repo.lastUserUUID)
	}
	if repo.lastAfterID != 30 {
		t.Fatalf("expected after id 30, got %d", repo.lastAfterID)
	}
	if repo.lastLimit != 10 {
		t.Fatalf("expected limit 10, got %d", repo.lastLimit)
	}
}

func setupMessageServiceRedisTest(t *testing.T) func() {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("run miniredis: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	oldRDB := store.RDB
	store.RDB = rdb

	return func() {
		_ = rdb.Close()
		mr.Close()
		store.RDB = oldRDB
	}
}

func TestMessageServiceListOfflineMessagesNormalizesLimit(t *testing.T) {
	t.Parallel()

	repo := &stubMessageRepository{}
	service := NewMessageService(repo, &stubMessageUserFinder{}, nil, nil, nil, nil, nil)

	if _, err := service.ListOfflineMessages("U100", 0, 200); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.lastLimit != 50 {
		t.Fatalf("expected normalized limit 50, got %d", repo.lastLimit)
	}
}

func TestMessageServiceSendDirectFileMessageSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubMessageRepository{}
	service := NewMessageService(repo, &stubMessageUserFinder{
		users: map[string]*model.User{
			"U200": {UUID: "U200", Status: model.UserStatusNormal},
		},
	}, &stubFriendshipChecker{
		friendships: map[string]map[string]bool{
			"U100": {"U200": true},
		},
	}, nil, &stubMessageFileFinder{
		files: map[string]*model.UploadedFile{
			"F100": {
				UUID:         "F100",
				UploaderUUID: "U100",
				FileName:     "hello.txt",
				FileSize:     128,
				ContentType:  "text/plain",
				URL:          "http://127.0.0.1:9000/dipole-files/message-files/hello.txt",
			},
		},
	}, nil, nil)

	message, err := service.SendDirectFileMessage("U100", "U200", "F100")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if message.MessageType != model.MessageTypeFile {
		t.Fatalf("expected file message type, got %d", message.MessageType)
	}
	if message.FileID != "F100" || message.FileName != "hello.txt" {
		t.Fatalf("unexpected file message payload: %+v", message)
	}
}

func TestMessageServiceSendDirectFileMessageRejectsMissingFileID(t *testing.T) {
	t.Parallel()

	service := NewMessageService(&stubMessageRepository{}, &stubMessageUserFinder{}, &stubFriendshipChecker{}, nil, &stubMessageFileFinder{}, nil, nil)

	_, err := service.SendDirectFileMessage("U100", "U200", "")
	if !errors.Is(err, ErrMessageFileRequired) {
		t.Fatalf("expected ErrMessageFileRequired, got %v", err)
	}
}

func TestMessageServiceSendAssistantTextMessageSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubMessageRepository{}
	service := NewMessageService(repo, &stubMessageUserFinder{
		users: map[string]*model.User{
			"UAI":  {UUID: "UAI", Status: model.UserStatusNormal, UserType: model.UserTypeAssistant},
			"U100": {UUID: "U100", Status: model.UserStatusNormal},
		},
	}, &stubFriendshipChecker{}, nil, nil, nil, nil)

	message, err := service.SendAssistantTextMessage("UAI", "U100", "hello from ai")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if message.MessageType != model.MessageTypeAIText {
		t.Fatalf("expected ai text message type, got %d", message.MessageType)
	}
	if len(repo.createdMessages) != 1 {
		t.Fatalf("expected one persisted assistant message, got %d", len(repo.createdMessages))
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
	}, nil, nil, publisher, nil)

	if _, err := service.SendDirectMessage("U100", "U200", "hello"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(repo.createdMessages) != 0 {
		t.Fatalf("expected no synchronous persistence when kafka publisher is enabled, got %d", len(repo.createdMessages))
	}
	if len(publisher.topics) != 1 || publisher.topics[0] != "message.direct.send_requested" {
		t.Fatalf("expected direct message event, got %+v", publisher.topics)
	}
	if len(publisher.eventTypes) != 1 || publisher.eventTypes[0] != "message.direct.send_requested" {
		t.Fatalf("expected direct message event type, got %+v", publisher.eventTypes)
	}
}

func TestMessageServicePersistRequestedMessageStoresCreatedOutbox(t *testing.T) {
	t.Parallel()

	repo := &stubMessageRepository{}
	publisher := &stubEventPublisher{}
	service := NewMessageService(repo, &stubMessageUserFinder{}, &stubFriendshipChecker{}, nil, nil, publisher, nil)

	message, err := service.PersistRequestedMessage(MessageEventPayload{
		MessageID:       "M100",
		ConversationKey: model.DirectConversationKey("U100", "U200"),
		SenderUUID:      "U100",
		TargetUUID:      "U200",
		TargetType:      model.MessageTargetDirect,
		MessageType:     model.MessageTypeText,
		Content:         "hello",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if message == nil || message.UUID != "M100" {
		t.Fatalf("expected persisted message M100, got %+v", message)
	}
	if len(repo.createdMessages) != 1 {
		t.Fatalf("expected one persisted message, got %d", len(repo.createdMessages))
	}
	if len(repo.outboxEvents) != 1 {
		t.Fatalf("expected one outbox event, got %d", len(repo.outboxEvents))
	}
	if repo.outboxEvents[0].Topic != "message.direct.created" {
		t.Fatalf("expected outbox topic message.direct.created, got %s", repo.outboxEvents[0].Topic)
	}
}

func TestMessageServicePersistRequestedMessageEnsuresOutboxOnDuplicate(t *testing.T) {
	t.Parallel()

	existing := &model.Message{
		UUID:            "M100",
		ConversationKey: model.DirectConversationKey("U100", "U200"),
		SenderUUID:      "U100",
		TargetUUID:      "U200",
		TargetType:      model.MessageTargetDirect,
		MessageType:     model.MessageTypeText,
		Content:         "hello",
	}
	repo := &stubMessageRepository{
		storeWithOutboxErr: gorm.ErrDuplicatedKey,
		messagesByUUID: map[string]*model.Message{
			"M100": existing,
		},
	}
	publisher := &stubEventPublisher{}
	service := NewMessageService(repo, &stubMessageUserFinder{}, &stubFriendshipChecker{}, nil, nil, publisher, nil)

	message, err := service.PersistRequestedMessage(MessageEventPayload{
		MessageID:       "M100",
		ConversationKey: model.DirectConversationKey("U100", "U200"),
		SenderUUID:      "U100",
		TargetUUID:      "U200",
		TargetType:      model.MessageTargetDirect,
		MessageType:     model.MessageTypeText,
		Content:         "hello",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if message != existing {
		t.Fatalf("expected existing message to be reused")
	}
	if len(repo.createdMessages) != 0 {
		t.Fatalf("expected no new persisted messages, got %d", len(repo.createdMessages))
	}
	if len(repo.ensuredOutboxEvents) != 1 {
		t.Fatalf("expected one ensured outbox event, got %d", len(repo.ensuredOutboxEvents))
	}
	if repo.ensuredOutboxEvents[0].Topic != "message.direct.created" {
		t.Fatalf("expected ensured outbox topic message.direct.created, got %s", repo.ensuredOutboxEvents[0].Topic)
	}
}
