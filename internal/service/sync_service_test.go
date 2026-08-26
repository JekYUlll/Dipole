package service

import (
	"errors"
	"testing"

	"github.com/JekYUlll/Dipole/internal/model"
)

type stubSyncRepository struct {
	items      []*model.SyncMessage
	checkpoint *model.DeviceSyncCheckpoint
	latest     uint64
	userUUID   string
	afterSeq   uint64
	limit      int
}

func (r *stubSyncRepository) GetDeviceCheckpoint(userUUID, deviceID string) (*model.DeviceSyncCheckpoint, error) {
	if r.checkpoint == nil {
		return nil, nil
	}
	copy := *r.checkpoint
	return &copy, nil
}

func (r *stubSyncRepository) GetLatestUserSyncSequence(string) (uint64, error) { return r.latest, nil }

func (r *stubSyncRepository) AdvanceDeviceSyncCheckpoint(userUUID, deviceID string, syncSeq uint64) error {
	if r.checkpoint != nil && r.checkpoint.SyncSeq > syncSeq {
		syncSeq = r.checkpoint.SyncSeq
	}
	r.checkpoint = &model.DeviceSyncCheckpoint{UserUUID: userUUID, DeviceID: deviceID, SyncSeq: syncSeq}
	return nil
}

func (r *stubSyncRepository) ListByUserAfter(userUUID string, afterSeq uint64, limit int) ([]*model.SyncMessage, error) {
	r.userUUID = userUUID
	r.afterSeq = afterSeq
	r.limit = limit
	return r.items, nil
}

func TestSyncServiceAdvancesDeviceCheckpointMonotonically(t *testing.T) {
	repo := &stubSyncRepository{latest: 20}
	checkpoint, err := NewSyncService(repo).AdvanceCheckpoint(" U200 ", " web-1 ", 12)
	if err != nil || checkpoint.SyncSeq != 12 || checkpoint.DeviceID != "web-1" {
		t.Fatalf("advance checkpoint: checkpoint=%+v err=%v", checkpoint, err)
	}
	checkpoint, err = NewSyncService(repo).AdvanceCheckpoint("U200", "web-1", 8)
	if err != nil || checkpoint.SyncSeq != 12 {
		t.Fatalf("checkpoint regressed: checkpoint=%+v err=%v", checkpoint, err)
	}
	if _, err := NewSyncService(repo).AdvanceCheckpoint("U200", "web-1", 21); !errors.Is(err, ErrSyncCheckpointAhead) {
		t.Fatalf("expected checkpoint ahead error, got %v", err)
	}
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
