package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
)

type stubConversationRepository struct {
	upsertCalls           []conversationUpsertCall
	upsertErr             error
	listConversations     []*model.Conversation
	listErr               error
	lastListUserUUID      string
	lastListLimit         int
	conversationByKey     *model.Conversation
	getConversationErr    error
	lastClearUserUUID     string
	lastClearConversation string
	clearErr              error
	lastRemarkUserUUID    string
	lastRemarkKey         string
	lastRemark            string
	updateRemarkErr       error
}

type conversationUpsertCall struct {
	userUUID        string
	targetUUID      string
	message         *model.Message
	unreadIncrement int
}

func (r *stubConversationRepository) UpsertDirectMessage(userUUID, targetUUID string, message *model.Message, unreadIncrement int) error {
	if r.upsertErr != nil {
		return r.upsertErr
	}

	r.upsertCalls = append(r.upsertCalls, conversationUpsertCall{
		userUUID:        userUUID,
		targetUUID:      targetUUID,
		message:         message,
		unreadIncrement: unreadIncrement,
	})
	return nil
}

func (r *stubConversationRepository) UpsertGroupMessage(userUUID, targetUUID string, message *model.Message, unreadIncrement int) error {
	if r.upsertErr != nil {
		return r.upsertErr
	}

	r.upsertCalls = append(r.upsertCalls, conversationUpsertCall{
		userUUID:        userUUID,
		targetUUID:      targetUUID,
		message:         message,
		unreadIncrement: unreadIncrement,
	})
	return nil
}

func (r *stubConversationRepository) ListByUserUUID(userUUID string, limit int) ([]*model.Conversation, error) {
	r.lastListUserUUID = userUUID
	r.lastListLimit = limit
	if r.listErr != nil {
		return nil, r.listErr
	}

	return r.listConversations, nil
}

func (r *stubConversationRepository) GetByUserAndConversationKey(userUUID, conversationKey string) (*model.Conversation, error) {
	r.lastClearUserUUID = userUUID
	r.lastClearConversation = conversationKey
	if r.getConversationErr != nil {
		return nil, r.getConversationErr
	}
	return r.conversationByKey, nil
}

func (r *stubConversationRepository) ClearUnreadByConversationKey(userUUID, conversationKey string) error {
	r.lastClearUserUUID = userUUID
	r.lastClearConversation = conversationKey
	return r.clearErr
}

func (r *stubConversationRepository) UpdateRemarkByConversationKey(userUUID, conversationKey, remark string) error {
	r.lastRemarkUserUUID = userUUID
	r.lastRemarkKey = conversationKey
	r.lastRemark = remark
	return r.updateRemarkErr
}

func (r *stubConversationRepository) InitGroupConversation(userUUID, groupUUID, conversationKey string, createdAt time.Time) error {
	return nil
}

type stubConversationUserFinder struct {
	usersByUUID map[string]*model.User
	listErr     error
	getErr      error
}

func (f *stubConversationUserFinder) GetByUUID(uuid string) (*model.User, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	return f.usersByUUID[uuid], nil
}

func (f *stubConversationUserFinder) ListByUUIDs(uuids []string) ([]*model.User, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}

	users := make([]*model.User, 0, len(uuids))
	for _, uuid := range uuids {
		if user, ok := f.usersByUUID[uuid]; ok {
			users = append(users, user)
		}
	}

	return users, nil
}

type stubConversationGroupRepository struct {
	groupsByUUID  map[string]*model.Group
	membersByPair map[string]*model.GroupMember
}

func (r *stubConversationGroupRepository) GetByUUID(groupUUID string) (*model.Group, error) {
	return r.groupsByUUID[groupUUID], nil
}

func (r *stubConversationGroupRepository) ListMembers(groupUUID string) ([]*model.GroupMember, error) {
	return nil, nil
}

func (r *stubConversationGroupRepository) GetMember(groupUUID, userUUID string) (*model.GroupMember, error) {
	return r.membersByPair[groupUUID+":"+userUUID], nil
}

type stubConversationNotifier struct {
	receipts []ConversationReadReceipt
}

func (n *stubConversationNotifier) NotifyDirectRead(receipt ConversationReadReceipt) {
	n.receipts = append(n.receipts, receipt)
}

type stubConversationEvents struct {
	publishedTopic   string
	publishedKey     string
	publishedType    string
	publishedPayload any
	publishErr       error
}

func (e *stubConversationEvents) PublishJSON(_ context.Context, topic string, key string, payload any, headers map[string]string) error {
	return nil
}

func (e *stubConversationEvents) PublishEvent(_ context.Context, topic string, key string, eventType string, payload any, headers map[string]string) error {
	e.publishedTopic = topic
	e.publishedKey = key
	e.publishedType = eventType
	e.publishedPayload = payload
	return e.publishErr
}

func TestConversationServiceUpdateDirectConversationsSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubConversationRepository{}
	service := NewConversationService(repo, &stubConversationUserFinder{}, nil, nil, nil)
	message := &model.Message{
		UUID:            "M100",
		ConversationKey: model.DirectConversationKey("U100", "U200"),
		SenderUUID:      "U100",
		TargetType:      model.MessageTargetDirect,
		TargetUUID:      "U200",
		MessageType:     model.MessageTypeText,
		Content:         "hello",
		SentAt:          time.Now().UTC(),
	}

	if err := service.UpdateDirectConversations(message); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(repo.upsertCalls) != 2 {
		t.Fatalf("expected 2 upsert calls, got %d", len(repo.upsertCalls))
	}
	if repo.upsertCalls[0].userUUID != "U100" || repo.upsertCalls[0].unreadIncrement != 0 {
		t.Fatalf("unexpected sender upsert call: %+v", repo.upsertCalls[0])
	}
	if repo.upsertCalls[1].userUUID != "U200" || repo.upsertCalls[1].targetUUID != "U100" || repo.upsertCalls[1].unreadIncrement != 1 {
		t.Fatalf("unexpected target upsert call: %+v", repo.upsertCalls[1])
	}
}

func TestConversationServiceListForUserSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubConversationRepository{
		listConversations: []*model.Conversation{
			{
				UserUUID:        "U100",
				TargetType:      model.MessageTargetDirect,
				TargetUUID:      "U200",
				ConversationKey: model.DirectConversationKey("U100", "U200"),
				UnreadCount:     2,
			},
		},
	}
	userFinder := &stubConversationUserFinder{
		usersByUUID: map[string]*model.User{
			"U200": {UUID: "U200", Nickname: "Alice", Avatar: "avatar"},
		},
	}
	service := NewConversationService(repo, userFinder, nil, nil, nil)

	conversations, err := service.ListForUser("U100", 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.lastListUserUUID != "U100" || repo.lastListLimit != 10 {
		t.Fatalf("unexpected list query: user=%s limit=%d", repo.lastListUserUUID, repo.lastListLimit)
	}
	if len(conversations) != 1 {
		t.Fatalf("expected 1 conversation, got %d", len(conversations))
	}
	if conversations[0].TargetUser == nil || conversations[0].TargetUser.UUID != "U200" {
		t.Fatalf("expected target user U200, got %+v", conversations[0].TargetUser)
	}
}

func TestConversationServiceUpdateGroupRemarkSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubConversationRepository{
		conversationByKey: &model.Conversation{
			UserUUID:        "U100",
			TargetType:      model.MessageTargetGroup,
			TargetUUID:      "G100",
			ConversationKey: model.GroupConversationKey("G100"),
			Remark:          "新备注",
		},
	}
	groupRepo := &stubConversationGroupRepository{
		groupsByUUID: map[string]*model.Group{
			"G100": {UUID: "G100", Status: model.GroupStatusNormal},
		},
		membersByPair: map[string]*model.GroupMember{
			"G100:U100": {GroupUUID: "G100", UserUUID: "U100", Role: model.GroupMemberRoleMember},
		},
	}
	service := NewConversationService(repo, &stubConversationUserFinder{}, groupRepo, nil, nil)

	conversation, err := service.UpdateGroupRemark("U100", "G100", "新备注")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if conversation == nil || conversation.Remark != "新备注" {
		t.Fatalf("unexpected conversation: %+v", conversation)
	}
	if repo.lastRemarkUserUUID != "U100" || repo.lastRemarkKey != model.GroupConversationKey("G100") || repo.lastRemark != "新备注" {
		t.Fatalf("unexpected update remark call: user=%s key=%s remark=%s", repo.lastRemarkUserUUID, repo.lastRemarkKey, repo.lastRemark)
	}
}

func TestConversationServiceUpdateGroupRemarkAllowsDismissedGroup(t *testing.T) {
	t.Parallel()

	repo := &stubConversationRepository{
		conversationByKey: &model.Conversation{
			UserUUID:        "U100",
			TargetType:      model.MessageTargetGroup,
			TargetUUID:      "G100",
			ConversationKey: model.GroupConversationKey("G100"),
			Remark:          "老群备注",
		},
	}
	groupRepo := &stubConversationGroupRepository{
		groupsByUUID: map[string]*model.Group{
			"G100": {UUID: "G100", Status: model.GroupStatusDismissed},
		},
		membersByPair: map[string]*model.GroupMember{
			"G100:U100": {GroupUUID: "G100", UserUUID: "U100", Role: model.GroupMemberRoleMember},
		},
	}
	service := NewConversationService(repo, &stubConversationUserFinder{}, groupRepo, nil, nil)

	conversation, err := service.UpdateGroupRemark("U100", "G100", "已解散群")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if conversation == nil {
		t.Fatalf("unexpected conversation: %+v", conversation)
	}
	if repo.lastRemark != "已解散群" {
		t.Fatalf("expected dismissed group remark update, got %s", repo.lastRemark)
	}
}

func TestConversationServiceMarkDirectConversationReadRejectsMissingTarget(t *testing.T) {
	t.Parallel()

	service := NewConversationService(&stubConversationRepository{}, &stubConversationUserFinder{
		usersByUUID: map[string]*model.User{},
	}, nil, nil, nil)

	_, err := service.MarkDirectConversationRead("U100", "U404")
	if !errors.Is(err, ErrConversationTargetNotFound) {
		t.Fatalf("expected ErrConversationTargetNotFound, got %v", err)
	}
}

func TestConversationServiceMarkDirectConversationReadPublishesReceipt(t *testing.T) {
	t.Parallel()

	repo := &stubConversationRepository{
		conversationByKey: &model.Conversation{
			ConversationKey: model.DirectConversationKey("U100", "U200"),
			LastMessageUUID: "M100",
		},
	}
	events := &stubConversationEvents{}
	service := NewConversationService(repo, &stubConversationUserFinder{
		usersByUUID: map[string]*model.User{
			"U200": {UUID: "U200"},
		},
	}, nil, nil, events)

	receipt, err := service.MarkDirectConversationRead("U100", "U200")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if receipt == nil || receipt.LastReadMessageUUID != "M100" {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
	if events.publishedTopic != "conversation.direct.read" {
		t.Fatalf("unexpected topic: %s", events.publishedTopic)
	}
	if events.publishedKey != model.DirectConversationKey("U100", "U200") {
		t.Fatalf("unexpected published key: %s", events.publishedKey)
	}
}

func TestConversationServiceMarkGroupConversationReadClearsUnread(t *testing.T) {
	t.Parallel()

	repo := &stubConversationRepository{}
	groupRepo := &stubConversationGroupRepository{
		groupsByUUID: map[string]*model.Group{
			"G100": {UUID: "G100", Status: model.GroupStatusNormal},
		},
		membersByPair: map[string]*model.GroupMember{
			"G100:U100": {GroupUUID: "G100", UserUUID: "U100"},
		},
	}
	service := NewConversationService(repo, &stubConversationUserFinder{}, groupRepo, nil, nil)

	if err := service.MarkGroupConversationRead("U100", "G100"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.lastClearConversation != model.GroupConversationKey("G100") {
		t.Fatalf("unexpected cleared conversation: %s", repo.lastClearConversation)
	}
}

func TestConversationServiceMarkGroupConversationReadAllowsDismissedGroup(t *testing.T) {
	t.Parallel()

	repo := &stubConversationRepository{}
	groupRepo := &stubConversationGroupRepository{
		groupsByUUID: map[string]*model.Group{
			"G100": {UUID: "G100", Status: model.GroupStatusDismissed},
		},
		membersByPair: map[string]*model.GroupMember{
			"G100:U100": {GroupUUID: "G100", UserUUID: "U100"},
		},
	}
	service := NewConversationService(repo, &stubConversationUserFinder{}, groupRepo, nil, nil)

	if err := service.MarkGroupConversationRead("U100", "G100"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.lastClearConversation != model.GroupConversationKey("G100") {
		t.Fatalf("unexpected cleared conversation: %s", repo.lastClearConversation)
	}
}

func TestConversationServiceMarkDirectConversationReadNotifiesWithoutEvents(t *testing.T) {
	t.Parallel()

	notifier := &stubConversationNotifier{}
	repo := &stubConversationRepository{
		conversationByKey: &model.Conversation{
			ConversationKey: model.DirectConversationKey("U100", "U200"),
			LastMessageUUID: "M100",
		},
	}
	service := NewConversationService(repo, &stubConversationUserFinder{
		usersByUUID: map[string]*model.User{
			"U200": {UUID: "U200"},
		},
	}, nil, notifier, nil)

	if _, err := service.MarkDirectConversationRead("U100", "U200"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(notifier.receipts) != 1 {
		t.Fatalf("expected 1 receipt, got %d", len(notifier.receipts))
	}
}
