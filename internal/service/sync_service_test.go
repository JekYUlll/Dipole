package service

import (
	"testing"

	"github.com/JekYUlll/Dipole/internal/model"
)

type stubSyncRepository struct {
	items    []*model.SyncMessage
	userUUID string
	afterSeq uint64
	limit    int
}

func (r *stubSyncRepository) ListByUserAfter(userUUID string, afterSeq uint64, limit int) ([]*model.SyncMessage, error) {
	r.userUUID = userUUID
	r.afterSeq = afterSeq
	r.limit = limit
	return r.items, nil
}

func TestSyncServiceListsPageAndAdvancesCursor(t *testing.T) {
	repo := &stubSyncRepository{items: []*model.SyncMessage{
		{SyncSeq: 11, ConversationKey: "direct:U100:U200", Message: &model.Message{UUID: "M11"}},
		{SyncSeq: 12, ConversationKey: "direct:U100:U200", Message: &model.Message{UUID: "M12"}},
		{SyncSeq: 13, ConversationKey: "group:G100", Message: &model.Message{UUID: "M13"}},
	}}

	page, err := NewSyncService(repo).List(" U200 ", 10, 2)
	if err != nil {
		t.Fatalf("list sync page: %v", err)
	}
	if repo.userUUID != "U200" || repo.afterSeq != 10 || repo.limit != 3 {
		t.Fatalf("unexpected repository query: user=%s after=%d limit=%d", repo.userUUID, repo.afterSeq, repo.limit)
	}
	if len(page.Items) != 2 || page.NextSeq != 12 || !page.HasMore {
		t.Fatalf("unexpected sync page: %+v", page)
	}
}

func TestSyncServiceKeepsCursorForEmptyPage(t *testing.T) {
	page, err := NewSyncService(&stubSyncRepository{}).List("U200", 42, 500)
	if err != nil {
		t.Fatalf("list empty sync page: %v", err)
	}
	if page.NextSeq != 42 || page.HasMore || len(page.Items) != 0 {
		t.Fatalf("unexpected empty sync page: %+v", page)
	}
}
