package service

import (
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
	lastClearUserUUID     string
	lastClearConversation string
	clearErr              error
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

func (r *stubConversationRepository) ListByUserUUID(userUUID string, limit int) ([]*model.Conversation, error) {
	r.lastListUserUUID = userUUID
	r.lastListLimit = limit
	if r.listErr != nil {
		return nil, r.listErr
	}

	return r.listConversations, nil
}

func (r *stubConversationRepository) ClearUnreadByConversationKey(userUUID, conversationKey string) error {
	r.lastClearUserUUID = userUUID
	r.lastClearConversation = conversationKey
	return r.clearErr
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

func TestConversationServiceUpdateDirectConversationsSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubConversationRepository{}
	service := NewConversationService(repo, &stubConversationUserFinder{})
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
	service := NewConversationService(repo, userFinder)

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

func TestConversationServiceMarkDirectConversationReadRejectsMissingTarget(t *testing.T) {
	t.Parallel()

	service := NewConversationService(&stubConversationRepository{}, &stubConversationUserFinder{
		usersByUUID: map[string]*model.User{},
	})

	err := service.MarkDirectConversationRead("U100", "U404")
	if !errors.Is(err, ErrConversationTargetNotFound) {
		t.Fatalf("expected ErrConversationTargetNotFound, got %v", err)
	}
}
