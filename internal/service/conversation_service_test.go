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
	lastReadThroughSeq    uint64
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

type batchConversationRepository struct {
	*stubConversationRepository
	batchCalls []*model.Message
	batchErr   error
}

func (r *batchConversationRepository) UpsertGroupMessageBatch(_ string, message *model.Message) error {
	r.batchCalls = append(r.batchCalls, message)
	return r.batchErr
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

func (r *stubConversationRepository) MarkReadThroughByConversationKey(userUUID, conversationKey string, readThroughSeq uint64) error {
	r.lastClearUserUUID = userUUID
	r.lastClearConversation = conversationKey
	r.lastReadThroughSeq = readThroughSeq
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
	members       []*model.GroupMember
}

func (r *stubConversationGroupRepository) GetByUUID(groupUUID string) (*model.Group, error) {
	return r.groupsByUUID[groupUUID], nil
}

func (r *stubConversationGroupRepository) ListMembers(groupUUID string) ([]*model.GroupMember, error) {
	return r.members, nil
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

func TestConversationServiceUpdateGroupConversationsUsesBatchRepository(t *testing.T) {
	t.Parallel()

	repo := &batchConversationRepository{stubConversationRepository: &stubConversationRepository{}}
	groupRepo := &stubConversationGroupRepository{members: []*model.GroupMember{{UserUUID: "U100"}, {UserUUID: "U200"}}}
	service := NewConversationService(repo, &stubConversationUserFinder{}, groupRepo, nil, nil)
	var observedProjection string
	service.SetProjectionWriteObserver(func(projection string, _ time.Duration, _ error) { observedProjection = projection })
	message := &model.Message{
		UUID: "M-batch", ConversationKey: "group:G100", SenderUUID: "U100", TargetType: model.MessageTargetGroup,
		TargetUUID: "G100", MessageType: model.MessageTypeText, Content: "hello", Seq: 7, SentAt: time.Now().UTC(),
	}

	if err := service.UpdateGroupConversations(message); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(repo.batchCalls) != 1 || repo.batchCalls[0] != message {
		t.Fatalf("expected one batch message, got %+v", repo.batchCalls)
	}
	if len(repo.upsertCalls) != 0 {
		t.Fatalf("expected no per-member upserts, got %d", len(repo.upsertCalls))
	}
	if observedProjection != "group_message" {
		t.Fatalf("batch path must preserve group_message metric label, got %q", observedProjection)
	}
}

func TestConversationServiceObservesProjectionWriteDurationAndOutcome(t *testing.T) {
	t.Parallel()

	repo := &stubConversationRepository{}
	groupRepo := &stubConversationGroupRepository{members: []*model.GroupMember{
		{UserUUID: "U100"},
		{UserUUID: "U200"},
	}}
	conversationService := NewConversationService(repo, &stubConversationUserFinder{}, groupRepo, nil, nil)
	type observation struct {
		projection string
		duration   time.Duration
		err        error
	}
	observations := make([]observation, 0, 6)
	conversationService.SetProjectionWriteObserver(func(projection string, duration time.Duration, err error) {
		observations = append(observations, observation{projection: projection, duration: duration, err: err})
	})

	direct := &model.Message{SenderUUID: "U100", TargetUUID: "U200", TargetType: model.MessageTargetDirect}
	if err := conversationService.UpdateDirectConversations(direct); err != nil {
		t.Fatalf("update direct conversations: %v", err)
	}
	if err := conversationService.InitGroupConversations("G100", []string{"U100", "U200"}, time.Now().UTC()); err != nil {
		t.Fatalf("initialize group conversations: %v", err)
	}
	group := &model.Message{SenderUUID: "U100", TargetUUID: "G100", TargetType: model.MessageTargetGroup}
	if err := conversationService.UpdateGroupConversations(group); err != nil {
		t.Fatalf("update group conversations: %v", err)
	}

	writes := map[string]int{}
	for _, observation := range observations {
		writes[observation.projection]++
		if observation.duration < 0 {
			t.Fatalf("projection %s duration = %v, want non-negative", observation.projection, observation.duration)
		}
		if observation.err != nil {
			t.Fatalf("projection %s returned unexpected observed error: %v", observation.projection, observation.err)
		}
	}
	for projection, want := range map[string]int{
		"direct_message": 2,
		"group_init":     2,
		"group_message":  2,
	} {
		if got := writes[projection]; got != want {
			t.Fatalf("projection %s writes = %v, want %v (all metrics: %v)", projection, got, want, writes)
		}
	}
}

func TestConversationServiceObservesFailedProjectionWrite(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("conversation write failed")
	repo := &stubConversationRepository{upsertErr: wantErr}
	conversationService := NewConversationService(repo, &stubConversationUserFinder{}, nil, nil, nil)
	var observedProjection string
	var observedDuration time.Duration
	var observedErr error
	conversationService.SetProjectionWriteObserver(func(projection string, duration time.Duration, err error) {
		observedProjection = projection
		observedDuration = duration
		observedErr = err
	})

	err := conversationService.UpdateDirectConversations(&model.Message{
		SenderUUID: "U100", TargetUUID: "U200", TargetType: model.MessageTargetDirect,
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("update direct conversations error = %v, want %v", err, wantErr)
	}
	if observedProjection != "direct_message" {
		t.Fatalf("observed projection = %q, want direct_message", observedProjection)
	}
	if observedDuration < 0 {
		t.Fatalf("observed duration = %v, want non-negative", observedDuration)
	}
	if !errors.Is(observedErr, wantErr) {
		t.Fatalf("observed error = %v, want %v", observedErr, wantErr)
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
			LastMessageSeq:  17,
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
	if receipt == nil || receipt.LastReadMessageUUID != "M100" || receipt.LastReadSeq != 17 || repo.lastReadThroughSeq != 17 {
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

	repo := &stubConversationRepository{conversationByKey: &model.Conversation{LastMessageSeq: 9}}
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
	if repo.lastReadThroughSeq != 9 {
		t.Fatalf("unexpected group read sequence: %d", repo.lastReadThroughSeq)
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
			LastMessageSeq:  18,
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
