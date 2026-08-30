package messagedomain

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/cache"
	"github.com/JekYUlll/Dipole/internal/platform/correlation"
	platformHotGroup "github.com/JekYUlll/Dipole/internal/platform/hotgroup"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	"github.com/alicebob/miniredis/v2"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

type stubMessageRepository struct {
	mu                    sync.Mutex
	createErr             error
	storeWithOutboxErr    error
	ensureOutboxErr       error
	listErr               error
	createdMessages       []*model.Message
	outboxEvents          []*model.OutboxEvent
	ensuredOutboxEvents   []*model.OutboxEvent
	syncRecipients        []string
	ensuredSyncRecipients []string
	listMessages          []*model.Message
	listAfterMessages     []*model.Message
	offlineMessages       []*model.Message
	messagesByUUID        map[string]*model.Message
	hasConversation       bool
	lastConversationKey   string
	lastBeforeID          uint
	lastBeforeSeq         uint64
	lastAfterID           uint
	lastAfterSeq          uint64
	lastLimit             int
	lastUserUUID          string
	listAfterCallCount    int
	getByUUIDCalls        int
	getBySenderErr        error
	listAfterDelay        time.Duration
}

type stubCoreCapability struct {
	users              map[string]*model.User
	ownedFiles         map[string]*model.UploadedFile
	directMessageAllow bool
	userLookups        []string
	friendshipChecks   [][2]string
}

func (c *stubCoreCapability) ListSearchConversationKeys(string) ([]string, error) { return nil, nil }

func (c *stubCoreCapability) GetOwnedFile(uploaderUUID, fileUUID string) (*model.UploadedFile, error) {
	file := c.ownedFiles[fileUUID]
	if file == nil || file.UploaderUUID != uploaderUUID {
		return nil, nil
	}
	return file, nil
}

func (c *stubCoreCapability) ListOwnedFiles(string, string, int) (*applicationPort.OwnedFilePage, error) {
	return &applicationPort.OwnedFilePage{}, nil
}

func (c *stubCoreCapability) GetUserByUUID(userUUID string) (*model.User, error) {
	c.userLookups = append(c.userLookups, userUUID)
	return c.users[userUUID], nil
}

func (c *stubCoreCapability) CanSendDirectMessage(userUUID, friendUUID string) (bool, error) {
	c.friendshipChecks = append(c.friendshipChecks, [2]string{userUUID, friendUUID})
	return c.directMessageAllow, nil
}

func (c *stubCoreCapability) GetGroupByUUID(string) (*model.Group, error) {
	return nil, nil
}

func (c *stubCoreCapability) GetGroupMember(string, string) (*model.GroupMember, error) {
	return nil, nil
}

func (c *stubCoreCapability) ListGroupMembers(string) ([]*model.GroupMember, error) {
	return nil, nil
}

func (r *stubMessageRepository) GetByUUID(uuid string) (*model.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.getByUUIDCalls++

	if r.messagesByUUID == nil {
		return nil, nil
	}

	return r.messagesByUUID[uuid], nil
}

func (r *stubMessageRepository) GetBySenderAndClientMessageID(senderUUID, clientMessageID string) (*model.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getBySenderErr != nil {
		return nil, r.getBySenderErr
	}

	if r.messagesByUUID == nil {
		return nil, nil
	}

	for _, message := range r.messagesByUUID {
		if message != nil && message.SenderUUID == senderUUID && message.ClientMessageID == clientMessageID {
			return message, nil
		}
	}

	return nil, nil
}

func TestMessageServiceGetsSenderScopedCommandReceipt(t *testing.T) {
	t.Parallel()

	existing := &model.Message{UUID: "M100", SenderUUID: "U100", ClientMessageID: "C100", Content: "hello"}
	repo := &stubMessageRepository{messagesByUUID: map[string]*model.Message{"M100": existing}}
	messageService := NewMessageService(repo, nil, nil, nil, nil, nil, nil)

	receipt, err := messageService.GetMessageCommandReceipt(" U100 ", " C100 ")
	if err != nil || receipt.Status != applicationPort.MessageCommandReceiptStatusCommitted || receipt.Message != existing {
		t.Fatalf("committed receipt=%+v err=%v", receipt, err)
	}
	receipt, err = messageService.GetMessageCommandReceipt("U100", "C404")
	if err != nil || receipt.Status != applicationPort.MessageCommandReceiptStatusAbsent || receipt.Message != nil {
		t.Fatalf("absent receipt=%+v err=%v", receipt, err)
	}
	if _, err := messageService.GetMessageCommandReceipt("U100", " "); !errors.Is(err, applicationPort.ErrMessageClientMessageIDInvalid) {
		t.Fatalf("blank client message ID error=%v", err)
	}
	if _, err := messageService.GetMessageCommandReceipt("U100", strings.Repeat("x", 65)); !errors.Is(err, applicationPort.ErrMessageClientMessageIDInvalid) {
		t.Fatalf("oversized client message ID error=%v", err)
	}
	repo.getBySenderErr = errors.New("metadata unavailable")
	if _, err := messageService.GetMessageCommandReceipt("U100", "C100"); err == nil || !strings.Contains(err.Error(), "metadata unavailable") {
		t.Fatalf("repository error=%v", err)
	}
}

func (r *stubMessageRepository) GetMetadataByUUID(uuid string) (*model.MessageMetadata, error) {
	message, err := r.GetByUUID(uuid)
	return stubMessageMetadata(message), err
}

func (r *stubMessageRepository) GetMetadataBySenderAndClientMessageID(senderUUID, clientMessageID string) (*model.MessageMetadata, error) {
	message, err := r.GetBySenderAndClientMessageID(senderUUID, clientMessageID)
	return stubMessageMetadata(message), err
}

func stubMessageMetadata(message *model.Message) *model.MessageMetadata {
	return model.MetadataFromMessage(message)
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

func (r *stubMessageRepository) StoreWithOutboxAndSync(message *model.Message, buildOutbox applicationPort.MessageOutboxBuilder, recipientUUIDs []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.storeWithOutboxErr != nil {
		return r.storeWithOutboxErr
	}
	if message.Seq == 0 {
		message.Seq = 1
	}
	event, err := buildOutbox(message)
	if err != nil {
		return err
	}

	r.createdMessages = append(r.createdMessages, message)
	if r.messagesByUUID == nil {
		r.messagesByUUID = make(map[string]*model.Message)
	}
	r.messagesByUUID[message.UUID] = message
	r.outboxEvents = append(r.outboxEvents, event)
	r.syncRecipients = append([]string(nil), recipientUUIDs...)
	return nil
}

func (r *stubMessageRepository) CreateWithSync(message *model.Message, recipientUUIDs []string) error {
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
	r.syncRecipients = append([]string(nil), recipientUUIDs...)
	return nil
}

func (r *stubMessageRepository) EnsureSyncInbox(_ *model.Message, recipientUUIDs []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensuredSyncRecipients = append([]string(nil), recipientUUIDs...)
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

func (r *stubMessageRepository) ListByConversationSeqAfter(conversationKey string, afterSeq uint64, limit int) ([]*model.Message, error) {
	r.mu.Lock()
	r.lastConversationKey = conversationKey
	r.lastAfterSeq = afterSeq
	r.lastLimit = limit
	r.listAfterCallCount++
	err := r.listErr
	messages := r.listAfterMessages
	r.mu.Unlock()
	return messages, err
}

func (r *stubMessageRepository) ListByConversationSeqBefore(conversationKey string, beforeSeq uint64, limit int) ([]*model.Message, error) {
	r.mu.Lock()
	r.lastConversationKey, r.lastBeforeSeq, r.lastLimit = conversationKey, beforeSeq, limit
	messages, err := r.listMessages, r.listErr
	r.mu.Unlock()
	return messages, err
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
	contexts   []correlation.IDs
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

func (o *stubHotGroupObserver) Status(groupUUID string, memberCount int) (platformHotGroup.Status, error) {
	return o.status, o.err
}

func (p *stubEventPublisher) PublishJSON(ctx context.Context, topic string, key string, payload any, headers map[string]string) error {
	p.topics = append(p.topics, topic)
	p.keys = append(p.keys, key)
	p.payloads = append(p.payloads, payload)
	p.contexts = append(p.contexts, correlation.FromContext(ctx))
	_ = headers
	return nil
}

func (p *stubEventPublisher) PublishEvent(ctx context.Context, topic string, key string, eventType string, payload any, headers map[string]string) error {
	p.topics = append(p.topics, topic)
	p.keys = append(p.keys, key)
	p.eventTypes = append(p.eventTypes, eventType)
	p.payloads = append(p.payloads, payload)
	p.contexts = append(p.contexts, correlation.FromContext(ctx))
	_ = headers
	return nil
}

func TestMessageServiceCommandContextReachesRequestedEvent(t *testing.T) {
	t.Parallel()
	publisher := &stubEventPublisher{}
	messageService := NewMessageService(
		&stubMessageRepository{},
		&stubMessageUserFinder{users: map[string]*model.User{"U200": {UUID: "U200"}}},
		&stubFriendshipChecker{friendships: map[string]map[string]bool{"U100": {"U200": true}}}, nil, nil, publisher, nil,
	)
	ctx := correlation.WithContext(context.Background(), correlation.IDs{RequestID: "R1", TraceID: "T1"})
	if _, err := messageService.SendDirectMessageContext(ctx, "U100", "U200", "hello", "C1"); err != nil {
		t.Fatalf("send direct message: %v", err)
	}
	if len(publisher.contexts) != 1 || publisher.contexts[0].RequestID != "R1" || publisher.contexts[0].TraceID != "T1" {
		t.Fatalf("unexpected event context: %+v", publisher.contexts)
	}
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

	message, err := service.SendDirectMessage("U100", " U200 ", " hello world ", "cmid-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(repo.createdMessages) != 1 {
		t.Fatalf("expected one persisted message, got %d", len(repo.createdMessages))
	}
	if message.UUID == "" || !strings.HasPrefix(message.UUID, "M") {
		t.Fatalf("expected generated message uuid, got %s", message.UUID)
	}
	if message.ClientMessageID != "cmid-1" {
		t.Fatalf("expected client message id cmid-1, got %s", message.ClientMessageID)
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

func TestMessageServiceWithCoreListsDirectMessagesThroughCapability(t *testing.T) {
	t.Parallel()

	repo := &stubMessageRepository{
		listMessages: []*model.Message{{UUID: "M100"}},
	}
	core := &stubCoreCapability{
		users: map[string]*model.User{
			"U200": {UUID: "U200", Status: model.UserStatusNormal},
		},
		directMessageAllow: true,
	}
	service := NewMessageServiceWithCore(repo, core, nil, nil, nil)

	messages, err := service.ListDirectMessages("U100", " U200 ", 0, 20)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(messages) != 1 || messages[0].UUID != "M100" {
		t.Fatalf("unexpected messages: %#v", messages)
	}
	if len(core.userLookups) != 1 || core.userLookups[0] != "U200" {
		t.Fatalf("unexpected user lookups: %#v", core.userLookups)
	}
	if len(core.friendshipChecks) != 1 || core.friendshipChecks[0] != [2]string{"U100", "U200"} {
		t.Fatalf("unexpected friendship checks: %#v", core.friendshipChecks)
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

	_, err := service.SendDirectMessage("U100", "U200", "hello", "cmid-2")
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

	_, err := service.SendDirectMessage("U100", "U200", "hello", "cmid-3")
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

	message, err := service.SendDirectMessage("U100", "UAI", "hello ai", "cmid-4")
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

func TestMessageServiceListDirectMessagesBeforeSeqUsesSequenceCursor(t *testing.T) {
	t.Parallel()
	repo := &stubMessageRepository{listMessages: []*model.Message{{Seq: 40, UUID: "M40"}}}
	service := NewMessageService(repo, &stubMessageUserFinder{users: map[string]*model.User{
		"U200": {UUID: "U200", Status: model.UserStatusNormal},
	}}, &stubFriendshipChecker{friendships: map[string]map[string]bool{
		"U100": {"U200": true},
	}}, nil, nil, nil, nil)

	messages, err := service.ListDirectMessagesBeforeSeq("U100", " U200 ", 41, 10)
	if err != nil || len(messages) != 1 || messages[0].Seq != 40 {
		t.Fatalf("unexpected sequence page=%+v err=%v", messages, err)
	}
	if repo.lastConversationKey != model.DirectConversationKey("U100", "U200") || repo.lastBeforeSeq != 41 || repo.lastLimit != 10 {
		t.Fatalf("unexpected repository query: key=%q before=%d limit=%d", repo.lastConversationKey, repo.lastBeforeSeq, repo.lastLimit)
	}
}

func TestMessageServiceListDirectMessagesAfterSeqUsesSequenceCursor(t *testing.T) {
	t.Parallel()
	repo := &stubMessageRepository{listAfterMessages: []*model.Message{{Seq: 42, UUID: "M42"}}}
	service := NewMessageService(repo, &stubMessageUserFinder{users: map[string]*model.User{
		"U200": {UUID: "U200", Status: model.UserStatusNormal},
	}}, &stubFriendshipChecker{friendships: map[string]map[string]bool{
		"U100": {"U200": true},
	}}, nil, nil, nil, nil)

	messages, err := service.ListDirectMessagesAfterSeq("U100", " U200 ", 41, 10)
	if err != nil || len(messages) != 1 || messages[0].Seq != 42 {
		t.Fatalf("unexpected sequence page=%+v err=%v", messages, err)
	}
	if repo.lastConversationKey != model.DirectConversationKey("U100", "U200") || repo.lastAfterSeq != 41 || repo.lastLimit != 10 {
		t.Fatalf("unexpected repository query: key=%q after=%d limit=%d", repo.lastConversationKey, repo.lastAfterSeq, repo.lastLimit)
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

	message, recipients, err := service.SendGroupMessage("U100", "G100", "hello group", "cmid-group-1")
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

	_, _, err := service.SendGroupMessage("U100", "G100", "hello", "cmid-group-2")
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

func TestMessageServiceListGroupMessagesBeforeSeqUsesSequenceCursor(t *testing.T) {
	t.Parallel()
	repo := &stubMessageRepository{listMessages: []*model.Message{{Seq: 40, UUID: "M40", TargetType: model.MessageTargetGroup}}}
	service := NewMessageService(repo, &stubMessageUserFinder{}, nil, &stubGroupMessageChecker{
		groups: map[string]*model.Group{"G100": {UUID: "G100", Status: model.GroupStatusNormal}},
		members: map[string]map[string]*model.GroupMember{"G100": {
			"U100": {GroupUUID: "G100", UserUUID: "U100"},
		}},
	}, nil, nil, nil)

	messages, err := service.ListGroupMessagesBeforeSeq("U100", " G100 ", 41, 10)
	if err != nil || len(messages) != 1 || messages[0].Seq != 40 {
		t.Fatalf("unexpected sequence page=%+v err=%v", messages, err)
	}
	if repo.lastConversationKey != model.GroupConversationKey("G100") || repo.lastBeforeSeq != 41 || repo.lastLimit != 10 {
		t.Fatalf("unexpected repository query: key=%q before=%d limit=%d", repo.lastConversationKey, repo.lastBeforeSeq, repo.lastLimit)
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

	_, _, err := service.SendGroupMessage("U100", "G100", "hello", "cmid-group-3")
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
	oldRDB := cache.RDB
	cache.RDB = rdb

	return func() {
		_ = rdb.Close()
		mr.Close()
		cache.RDB = oldRDB
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

	message, err := service.SendDirectFileMessage("U100", "U200", "F100", "cmid-file-1")
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

func TestMessageServiceWithCoreLoadsOwnedFileThroughCapability(t *testing.T) {
	t.Parallel()

	core := &stubCoreCapability{
		users: map[string]*model.User{
			"U200": {UUID: "U200", Status: model.UserStatusNormal},
		},
		ownedFiles: map[string]*model.UploadedFile{
			"F100": {UUID: "F100", UploaderUUID: "U100", FileName: "remote.txt", FileSize: 64, ContentType: "text/plain", URL: "https://files.test/remote.txt"},
		},
		directMessageAllow: true,
	}
	service := NewMessageServiceWithCore(&stubMessageRepository{}, core, nil, nil, nil)

	message, err := service.SendDirectFileMessage("U100", "U200", "F100", "cmid-core-file")
	if err != nil {
		t.Fatalf("send file through core capability: %v", err)
	}
	if message == nil || message.FileID != "F100" || message.FileName != "remote.txt" {
		t.Fatalf("unexpected file message: %+v", message)
	}

	_, err = service.SendDirectFileMessage("U999", "U200", "F100", "cmid-unowned-file")
	if !errors.Is(err, ErrMessageFileUnavailable) {
		t.Fatalf("expected unavailable unowned file, got %v", err)
	}
}

func TestMessageServiceSendDirectFileMessageRejectsMissingFileID(t *testing.T) {
	t.Parallel()

	service := NewMessageService(&stubMessageRepository{}, &stubMessageUserFinder{}, &stubFriendshipChecker{}, nil, &stubMessageFileFinder{}, nil, nil)

	_, err := service.SendDirectFileMessage("U100", "U200", "", "cmid-file-2")
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

func TestMessageServiceAgentCommandsPreserveExplicitClientMessageID(t *testing.T) {
	t.Parallel()

	repo := &stubMessageRepository{}
	service := NewMessageService(repo, &stubMessageUserFinder{users: map[string]*model.User{
		"UAI":  {UUID: "UAI", Status: model.UserStatusNormal, UserType: model.UserTypeAssistant},
		"U100": {UUID: "U100", Status: model.UserStatusNormal},
	}}, &stubFriendshipChecker{}, nil, nil, nil, nil)

	reply, err := service.SendAssistantTextMessageContext(context.Background(), "UAI", "U100", "hello", "agent-command-reply")
	if err != nil {
		t.Fatalf("send assistant command: %v", err)
	}
	system, err := service.SendSystemDirectMessageCommandContext(context.Background(), "UAI", "U100", "notice", "agent-command-system")
	if err != nil {
		t.Fatalf("send system command: %v", err)
	}
	if reply.ClientMessageID != "agent-command-reply" || system.ClientMessageID != "agent-command-system" {
		t.Fatalf("explicit command IDs were not preserved: reply=%q system=%q", reply.ClientMessageID, system.ClientMessageID)
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

	if _, err := service.SendDirectMessage("U100", "U200", "hello", "cmid-5"); err != nil {
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
	if len(publisher.keys) != 1 || publisher.keys[0] != model.DirectConversationKey("U100", "U200") {
		t.Fatalf("expected direct routing key %s, got %+v", model.DirectConversationKey("U100", "U200"), publisher.keys)
	}
	payload := publisher.payloads[0].(MessageEventPayload)
	if payload.SyncFanout == nil || !*payload.SyncFanout {
		t.Fatal("expected direct message sync fanout")
	}
}

func TestMessageServicePublishesKafkaEventOnGroupMessageWithGroupRoutingKey(t *testing.T) {
	t.Parallel()

	repo := &stubMessageRepository{}
	publisher := &stubEventPublisher{}
	service := NewMessageService(
		repo,
		&stubMessageUserFinder{},
		&stubFriendshipChecker{},
		&stubGroupMessageChecker{
			groups: map[string]*model.Group{
				"G100": {UUID: "G100", Status: model.GroupStatusNormal},
			},
			members: map[string]map[string]*model.GroupMember{
				"G100": {
					"U100": {GroupUUID: "G100", UserUUID: "U100"},
					"U200": {GroupUUID: "G100", UserUUID: "U200"},
				},
			},
		},
		nil,
		publisher,
		nil,
	)

	if _, _, err := service.SendGroupMessage("U100", "G100", "hello", "cmid-group-key"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(publisher.keys) != 1 || publisher.keys[0] != "G100" {
		t.Fatalf("expected group routing key G100, got %+v", publisher.keys)
	}
	payload := publisher.payloads[0].(MessageEventPayload)
	if payload.SyncFanout == nil || !*payload.SyncFanout {
		t.Fatal("expected regular group sync fanout")
	}
}

func TestMessageServiceSkipsSyncFanoutForHotGroup(t *testing.T) {
	publisher := &stubEventPublisher{}
	service := NewMessageService(
		&stubMessageRepository{},
		&stubMessageUserFinder{},
		nil,
		&stubGroupMessageChecker{
			groups: map[string]*model.Group{"G100": {UUID: "G100", Status: model.GroupStatusNormal}},
			members: map[string]map[string]*model.GroupMember{"G100": {
				"U100": {GroupUUID: "G100", UserUUID: "U100"},
				"U200": {GroupUUID: "G100", UserUUID: "U200"},
			}},
		},
		nil,
		publisher,
		&stubHotGroupObserver{status: platformHotGroup.Status{IsHot: true}},
	)

	if _, _, err := service.SendGroupMessage("U100", "G100", "hot", "cmid-hot"); err != nil {
		t.Fatalf("send hot group message: %v", err)
	}
	payload := publisher.payloads[0].(MessageEventPayload)
	if payload.SyncFanout == nil || *payload.SyncFanout {
		t.Fatal("expected hot group to keep notify-and-pull without inbox fanout")
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
		SyncFanout:      boolFlag(true),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if message == nil || message.UUID != "M100" || message.Seq != 1 {
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
	if repo.outboxEvents[0].AggregateID != "M100" || repo.outboxEvents[0].MessageKey != "M100" {
		t.Fatalf("created outbox identity changed: %+v", repo.outboxEvents[0])
	}
	var headers map[string]string
	if err := json.Unmarshal(repo.outboxEvents[0].HeadersJSON, &headers); err != nil {
		t.Fatalf("decode outbox headers: %v", err)
	}
	if headers["version"] != "v1" || headers["schema_version"] != "v1" {
		t.Fatalf("expected versioned outbox headers, got %+v", headers)
	}
	var envelope struct {
		Payload MessageEventPayload `json:"payload"`
	}
	if err := json.Unmarshal(repo.outboxEvents[0].Value, &envelope); err != nil {
		t.Fatalf("decode outbox envelope: %v", err)
	}
	if envelope.Payload.MessageSeq != 1 {
		t.Fatalf("created event sequence = %d, want 1", envelope.Payload.MessageSeq)
	}
	if envelope.Payload.MutationType != MessageMutationCreated || envelope.Payload.Revision != 1 || envelope.Payload.ActorUUID != "U100" {
		t.Fatalf("unexpected created mutation metadata: %+v", envelope.Payload)
	}
	if len(repo.syncRecipients) != 2 || repo.syncRecipients[0] != "U100" || repo.syncRecipients[1] != "U200" {
		t.Fatalf("expected direct participants in sync inbox, got %+v", repo.syncRecipients)
	}
}

func TestMessageServicePersistRequestedLegacyDirectEventStillWritesSyncInbox(t *testing.T) {
	repo := &stubMessageRepository{}
	service := NewMessageService(repo, &stubMessageUserFinder{}, nil, nil, nil, &stubEventPublisher{}, nil)

	if _, err := service.PersistRequestedMessage(MessageEventPayload{
		MessageID:       "M-legacy",
		ConversationKey: model.DirectConversationKey("U100", "U200"),
		SenderUUID:      "U100",
		TargetUUID:      "U200",
		TargetType:      model.MessageTargetDirect,
		MessageType:     model.MessageTypeText,
		Content:         "legacy",
	}); err != nil {
		t.Fatalf("persist legacy direct event: %v", err)
	}
	if len(repo.syncRecipients) != 2 || repo.syncRecipients[0] != "U100" || repo.syncRecipients[1] != "U200" {
		t.Fatalf("expected legacy direct event to sync both participants, got %+v", repo.syncRecipients)
	}
}

func TestMessageServicePersistRequestedContextStoresCorrelationInOutbox(t *testing.T) {
	t.Parallel()
	repo := &stubMessageRepository{}
	messageService := NewMessageService(repo, &stubMessageUserFinder{}, &stubFriendshipChecker{}, nil, nil, &stubEventPublisher{}, nil)
	ctx := correlation.WithContext(context.Background(), correlation.IDs{RequestID: "R1", TraceID: "T1", EventID: "requested-event"})
	_, err := messageService.PersistRequestedMessageContext(ctx, MessageEventPayload{
		MessageID: "M-context", ConversationKey: model.DirectConversationKey("U100", "U200"),
		SenderUUID: "U100", TargetUUID: "U200", TargetType: model.MessageTargetDirect,
		MessageType: model.MessageTypeText, Content: "hello", SyncFanout: boolFlag(true),
	})
	if err != nil {
		t.Fatalf("persist requested message: %v", err)
	}
	if len(repo.outboxEvents) != 1 {
		t.Fatalf("outbox events = %d, want 1", len(repo.outboxEvents))
	}
	var headers map[string]string
	if err := json.Unmarshal(repo.outboxEvents[0].HeadersJSON, &headers); err != nil {
		t.Fatalf("decode headers: %v", err)
	}
	if headers["request_id"] != "R1" || headers["trace_id"] != "T1" || headers["event_id"] == "" || headers["event_id"] == "requested-event" {
		t.Fatalf("unexpected outbox correlation: %+v", headers)
	}
	var envelope platformKafka.Envelope
	if err := json.Unmarshal(repo.outboxEvents[0].Value, &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.RequestID != "R1" || envelope.TraceID != "T1" || envelope.EventID != headers["event_id"] {
		t.Fatalf("envelope/header mismatch: envelope=%+v headers=%+v", envelope, headers)
	}
}

func TestMessageServicePersistRequestedLegacyGroupEventDefaultsToSyncFanout(t *testing.T) {
	repo := &stubMessageRepository{}
	service := NewMessageService(repo, &stubMessageUserFinder{}, nil, nil, nil, &stubEventPublisher{}, nil)

	if _, err := service.PersistRequestedMessage(MessageEventPayload{
		MessageID:       "M-legacy-group",
		ConversationKey: model.GroupConversationKey("G100"),
		SenderUUID:      "U100",
		TargetUUID:      "G100",
		TargetType:      model.MessageTargetGroup,
		MessageType:     model.MessageTypeText,
		Content:         "legacy group",
		RecipientUUIDs:  []string{"U100", "U200"},
	}); err != nil {
		t.Fatalf("persist legacy group event: %v", err)
	}
	if len(repo.syncRecipients) != 2 || repo.syncRecipients[0] != "U100" || repo.syncRecipients[1] != "U200" {
		t.Fatalf("expected legacy group event to default to sync fanout, got %+v", repo.syncRecipients)
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
		storeWithOutboxErr: &mysqlDriver.MySQLError{Number: 1062},
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
		SyncFanout:      boolFlag(true),
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
	if len(repo.ensuredSyncRecipients) != 2 {
		t.Fatalf("expected duplicate replay to ensure sync inbox, got %+v", repo.ensuredSyncRecipients)
	}
}

func TestMessageServicePersistRequestedMessageReusesExistingMessageByClientMessageID(t *testing.T) {
	t.Parallel()

	existing := &model.Message{
		UUID:            "M100",
		ClientMessageID: "cmid-duplicate",
		ConversationKey: model.DirectConversationKey("U100", "U200"),
		SenderUUID:      "U100",
		TargetUUID:      "U200",
		TargetType:      model.MessageTargetDirect,
		MessageType:     model.MessageTypeText,
		Content:         "hello",
	}
	repo := &stubMessageRepository{
		storeWithOutboxErr: &mysqlDriver.MySQLError{Number: 1062},
		messagesByUUID: map[string]*model.Message{
			"M100": existing,
		},
	}
	service := NewMessageService(repo, &stubMessageUserFinder{}, &stubFriendshipChecker{}, nil, nil, &stubEventPublisher{}, nil)

	message, err := service.PersistRequestedMessage(MessageEventPayload{
		MessageID:       "M999",
		ClientMessageID: "cmid-duplicate",
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
		t.Fatalf("expected existing message to be reused by client_message_id")
	}
	if len(repo.ensuredOutboxEvents) != 1 {
		t.Fatalf("expected ensured outbox event, got %d", len(repo.ensuredOutboxEvents))
	}
}

func TestMessageServiceDuplicateHydrationUsesCassandraAndFallsBackToMySQL(t *testing.T) {
	existing := &model.Message{
		ID: 42, UUID: "M100", ClientMessageID: "cmid-duplicate", ConversationKey: model.DirectConversationKey("U100", "U200"),
		Seq: 7, SenderUUID: "U100", TargetUUID: "U200", TargetType: model.MessageTargetDirect,
		MessageType: model.MessageTypeText, Content: "hello",
	}
	request := MessageEventPayload{
		MessageID: "M999", ClientMessageID: "cmid-duplicate", ConversationKey: existing.ConversationKey,
		SenderUUID: "U100", TargetUUID: "U200", TargetType: model.MessageTargetDirect,
		MessageType: model.MessageTypeText, Content: "hello",
	}
	cassandraMessage := *existing
	cassandraMessage.ID = 0
	for _, test := range []struct {
		name           string
		hydrator       *stubDuplicateHydrator
		wantMySQLReads int
		wantOutcome    string
	}{
		{name: "Cassandra hit", hydrator: &stubDuplicateHydrator{message: &cassandraMessage}, wantOutcome: "hit"},
		{name: "Cassandra miss", hydrator: &stubDuplicateHydrator{err: errors.New("missing")}, wantMySQLReads: 1, wantOutcome: "fallback"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &stubMessageRepository{storeWithOutboxErr: &mysqlDriver.MySQLError{Number: 1062}, messagesByUUID: map[string]*model.Message{"M100": existing}}
			messageService := NewMessageService(repo, &stubMessageUserFinder{}, nil, nil, nil, &stubEventPublisher{}, nil)
			var outcome string
			messageService.SetDuplicateMessageHydrator(test.hydrator, func(value string) { outcome = value })
			message, err := messageService.PersistRequestedMessage(request)
			if err != nil || message == nil || message.UUID != existing.UUID || message.ID != existing.ID {
				t.Fatalf("duplicate hydration: message=%+v err=%v", message, err)
			}
			if test.hydrator.locator.MessageUUID != "M100" || test.hydrator.locator.ConversationKey != existing.ConversationKey || test.hydrator.locator.MessageSeq != 7 {
				t.Fatalf("unexpected Cassandra locator: %+v", test.hydrator.locator)
			}
			if repo.getByUUIDCalls != test.wantMySQLReads {
				t.Fatalf("MySQL body reads=%d want=%d", repo.getByUUIDCalls, test.wantMySQLReads)
			}
			if outcome != test.wantOutcome {
				t.Fatalf("hydration outcome=%q want=%q", outcome, test.wantOutcome)
			}
		})
	}
}

func TestMessageServiceDuplicateHydrationSkipsHistoricalMetadataWithoutSequence(t *testing.T) {
	existing := &model.Message{
		UUID: "M100", ClientMessageID: "cmid-duplicate", ConversationKey: model.DirectConversationKey("U100", "U200"),
		SenderUUID: "U100", TargetUUID: "U200", TargetType: model.MessageTargetDirect, MessageType: model.MessageTypeText, Content: "hello",
	}
	repo := &stubMessageRepository{storeWithOutboxErr: &mysqlDriver.MySQLError{Number: 1062}, messagesByUUID: map[string]*model.Message{"M100": existing}}
	hydrator := &stubDuplicateHydrator{message: existing}
	messageService := NewMessageService(repo, &stubMessageUserFinder{}, nil, nil, nil, &stubEventPublisher{}, nil)
	var outcome string
	messageService.SetDuplicateMessageHydrator(hydrator, func(value string) { outcome = value })
	message, err := messageService.PersistRequestedMessage(MessageEventPayload{
		MessageID: "M999", ClientMessageID: "cmid-duplicate", ConversationKey: existing.ConversationKey,
		SenderUUID: "U100", TargetUUID: "U200", TargetType: model.MessageTargetDirect, MessageType: model.MessageTypeText, Content: "hello",
	})
	if err != nil || message != existing || repo.getByUUIDCalls != 1 {
		t.Fatalf("historical duplicate fallback: message=%+v reads=%d err=%v", message, repo.getByUUIDCalls, err)
	}
	if hydrator.locator.MessageUUID != "" {
		t.Fatalf("historical metadata reached Cassandra: %+v", hydrator.locator)
	}
	if outcome != "skipped_no_seq" {
		t.Fatalf("historical hydration outcome=%q", outcome)
	}
}

type stubDuplicateHydrator struct {
	message *model.Message
	err     error
	locator model.SyncMessageLocator
}

func (h *stubDuplicateHydrator) Hydrate(_ context.Context, locators []model.SyncMessageLocator) (map[string]*model.Message, error) {
	if len(locators) == 1 {
		h.locator = locators[0]
	}
	if h.err != nil {
		return nil, h.err
	}
	return map[string]*model.Message{h.message.UUID: h.message}, nil
}

func TestMessageServicePersistRequestedMessageRejectsConflictingIdempotencyTarget(t *testing.T) {
	existing := &model.Message{
		UUID:            "M100",
		ClientMessageID: "cmid-duplicate",
		ConversationKey: model.DirectConversationKey("U100", "U200"),
		SenderUUID:      "U100",
		TargetUUID:      "U200",
		TargetType:      model.MessageTargetDirect,
		MessageType:     model.MessageTypeText,
		Content:         "private",
	}
	repo := &stubMessageRepository{
		storeWithOutboxErr: &mysqlDriver.MySQLError{Number: 1062},
		messagesByUUID:     map[string]*model.Message{"M100": existing},
	}
	service := NewMessageService(repo, &stubMessageUserFinder{}, nil, nil, nil, &stubEventPublisher{}, nil)

	_, err := service.PersistRequestedMessage(MessageEventPayload{
		MessageID:       "M999",
		ClientMessageID: "cmid-duplicate",
		ConversationKey: model.DirectConversationKey("U100", "U300"),
		SenderUUID:      "U100",
		TargetUUID:      "U300",
		TargetType:      model.MessageTargetDirect,
		MessageType:     model.MessageTypeText,
		Content:         "private",
	})
	if !errors.Is(err, ErrMessageIdempotencyConflict) {
		t.Fatalf("expected ErrMessageIdempotencyConflict, got %v", err)
	}
	if len(repo.ensuredOutboxEvents) != 0 || len(repo.ensuredSyncRecipients) != 0 {
		t.Fatalf("expected no duplicate repair for conflicting target, outbox=%d inbox=%+v", len(repo.ensuredOutboxEvents), repo.ensuredSyncRecipients)
	}
}

func TestMessageServicePersistRequestedMessageRejectsConflictingIdempotencyPayload(t *testing.T) {
	existing := &model.Message{
		UUID: "M100", ClientMessageID: "cmid-duplicate",
		ConversationKey: model.DirectConversationKey("U100", "U200"),
		SenderUUID:      "U100", TargetUUID: "U200", TargetType: model.MessageTargetDirect,
		MessageType: model.MessageTypeText, Content: "original",
	}
	repo := &stubMessageRepository{
		storeWithOutboxErr: &mysqlDriver.MySQLError{Number: 1062},
		messagesByUUID:     map[string]*model.Message{"M100": existing},
	}
	service := NewMessageService(repo, &stubMessageUserFinder{}, nil, nil, nil, &stubEventPublisher{}, nil)

	_, err := service.PersistRequestedMessage(MessageEventPayload{
		MessageID: "M999", ClientMessageID: "cmid-duplicate",
		ConversationKey: model.DirectConversationKey("U100", "U200"),
		SenderUUID:      "U100", TargetUUID: "U200", TargetType: model.MessageTargetDirect,
		MessageType: model.MessageTypeText, Content: "changed",
	})
	if !errors.Is(err, ErrMessageIdempotencyConflict) {
		t.Fatalf("expected payload conflict, got %v", err)
	}
	if len(repo.ensuredOutboxEvents) != 0 || len(repo.ensuredSyncRecipients) != 0 {
		t.Fatalf("payload conflict repaired duplicate state: outbox=%d inbox=%+v", len(repo.ensuredOutboxEvents), repo.ensuredSyncRecipients)
	}
}

func TestMessageServicePersistLocalMessageRejectsConflictingIdempotencyTarget(t *testing.T) {
	existing := &model.Message{
		UUID:            "M100",
		ClientMessageID: "cmid-duplicate",
		ConversationKey: model.DirectConversationKey("U100", "U200"),
		SenderUUID:      "U100",
		TargetUUID:      "U200",
		TargetType:      model.MessageTargetDirect,
	}
	repo := &stubMessageRepository{
		createErr:      &mysqlDriver.MySQLError{Number: 1062},
		messagesByUUID: map[string]*model.Message{"M100": existing},
	}
	service := NewMessageService(repo, &stubMessageUserFinder{}, nil, nil, nil, nil, nil)
	requested := &model.Message{
		UUID:            "M999",
		ClientMessageID: "cmid-duplicate",
		ConversationKey: model.DirectConversationKey("U100", "U300"),
		SenderUUID:      "U100",
		TargetUUID:      "U300",
		TargetType:      model.MessageTargetDirect,
	}

	_, err := service.persistLocalMessage(requested, "persist direct message", []string{"U100", "U300"})
	if !errors.Is(err, ErrMessageIdempotencyConflict) {
		t.Fatalf("expected ErrMessageIdempotencyConflict, got %v", err)
	}
	if len(repo.ensuredSyncRecipients) != 0 {
		t.Fatalf("expected no local inbox repair for conflicting target, got %+v", repo.ensuredSyncRecipients)
	}
}
