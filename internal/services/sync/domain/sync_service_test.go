package syncdomain

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
	groups     map[string]*model.GroupSyncCheckpoint
}

type stubSyncGroupAuthorizer struct{ memberships map[string]bool }

func (a stubSyncGroupAuthorizer) GetGroupMember(groupUUID, userUUID string) (*model.GroupMember, error) {
	if !a.memberships[groupUUID+":"+userUUID] {
		return nil, nil
	}
	return &model.GroupMember{GroupUUID: groupUUID, UserUUID: userUUID}, nil
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

func (r *stubSyncRepository) ListGroupSyncCheckpoints(_, _ string, groupUUIDs []string) ([]*model.GroupSyncCheckpoint, error) {
	result := make([]*model.GroupSyncCheckpoint, 0, len(groupUUIDs))
	for _, groupUUID := range groupUUIDs {
		if checkpoint := r.groups[groupUUID]; checkpoint != nil {
			copy := *checkpoint
			result = append(result, &copy)
		}
	}
	return result, nil
}

func (r *stubSyncRepository) GetGroupSyncState(groupUUID string) (*model.GroupSyncState, error) {
	checkpoint := r.groups[groupUUID]
	if checkpoint == nil {
		return nil, nil
	}
	return &model.GroupSyncState{GroupUUID: groupUUID, LatestMessageSeq: checkpoint.LatestMessageSeq, LatestMessageUUID: checkpoint.LatestMessageUUID}, nil
}

func (r *stubSyncRepository) AdvanceDeviceGroupSyncCheckpoint(_, _, groupUUID string, messageSeq uint64) error {
	checkpoint := r.groups[groupUUID]
	if checkpoint == nil {
		checkpoint = &model.GroupSyncCheckpoint{GroupUUID: groupUUID}
		r.groups[groupUUID] = checkpoint
	}
	if messageSeq > checkpoint.PulledMessageSeq {
		checkpoint.PulledMessageSeq = messageSeq
	}
	return nil
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

func TestSyncServiceListsAuthorizedGroupCheckpoints(t *testing.T) {
	repo := &stubSyncRepository{groups: map[string]*model.GroupSyncCheckpoint{
		"G1": {GroupUUID: "G1", LatestMessageSeq: 12, LatestMessageUUID: "M12", PulledMessageSeq: 9},
	}}
	authorizer := stubSyncGroupAuthorizer{memberships: map[string]bool{"G1:U1": true, "G2:U1": true}}
	checkpoints, err := NewSyncService(repo, authorizer).ListGroupCheckpoints(" U1 ", " web-1 ", []string{" G2 ", "G1", "G1"})
	if err != nil || len(checkpoints) != 2 {
		t.Fatalf("list group checkpoints: checkpoints=%+v err=%v", checkpoints, err)
	}
	if checkpoints[0].GroupUUID != "G1" || checkpoints[0].LatestMessageSeq != 12 || checkpoints[1].GroupUUID != "G2" || checkpoints[1].LatestMessageSeq != 0 {
		t.Fatalf("unexpected group checkpoints: %+v", checkpoints)
	}
}

func TestSyncServiceRejectsUnauthorizedGroupCheckpointRequest(t *testing.T) {
	repo := &stubSyncRepository{groups: map[string]*model.GroupSyncCheckpoint{}}
	_, err := NewSyncService(repo, stubSyncGroupAuthorizer{}).ListGroupCheckpoints("U1", "web-1", []string{"G-private"})
	if !errors.Is(err, ErrSyncGroupForbidden) {
		t.Fatalf("expected group forbidden, got %v", err)
	}
}

func TestSyncServiceAdvancesGroupCheckpointMonotonically(t *testing.T) {
	repo := &stubSyncRepository{groups: map[string]*model.GroupSyncCheckpoint{
		"G1": {GroupUUID: "G1", LatestMessageSeq: 20, LatestMessageUUID: "M20"},
	}}
	authorizer := stubSyncGroupAuthorizer{memberships: map[string]bool{"G1:U1": true}}
	service := NewSyncService(repo, authorizer)
	checkpoint, err := service.AdvanceGroupCheckpoint("U1", "web-1", "G1", 12)
	if err != nil || checkpoint.PulledMessageSeq != 12 {
		t.Fatalf("advance group checkpoint: checkpoint=%+v err=%v", checkpoint, err)
	}
	checkpoint, err = service.AdvanceGroupCheckpoint("U1", "web-1", "G1", 8)
	if err != nil || checkpoint.PulledMessageSeq != 12 {
		t.Fatalf("group checkpoint regressed: checkpoint=%+v err=%v", checkpoint, err)
	}
	if _, err := service.AdvanceGroupCheckpoint("U1", "web-1", "G1", 21); !errors.Is(err, ErrSyncCheckpointAhead) {
		t.Fatalf("expected checkpoint ahead, got %v", err)
	}
}
