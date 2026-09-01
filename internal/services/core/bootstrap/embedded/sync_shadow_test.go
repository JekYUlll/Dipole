package embedded

import (
	"sync/atomic"
	"testing"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

type syncShadowProbe struct {
	reads    atomic.Int32
	advances atomic.Int32
}

func (p *syncShadowProbe) List(string, uint64, int) (*application.SyncPage, error) {
	p.reads.Add(1)
	return &application.SyncPage{NextSeq: 8}, nil
}
func (p *syncShadowProbe) GetCheckpoint(user, device string) (*model.DeviceSyncCheckpoint, error) {
	p.reads.Add(1)
	return &model.DeviceSyncCheckpoint{UserUUID: user, DeviceID: device, SyncSeq: 8}, nil
}
func (p *syncShadowProbe) AdvanceCheckpoint(user, device string, seq uint64) (*model.DeviceSyncCheckpoint, error) {
	p.advances.Add(1)
	return &model.DeviceSyncCheckpoint{UserUUID: user, DeviceID: device, SyncSeq: seq}, nil
}
func (p *syncShadowProbe) ListGroupCheckpoints(string, string, []string) ([]*model.GroupSyncCheckpoint, error) {
	p.reads.Add(1)
	return []*model.GroupSyncCheckpoint{{GroupUUID: "G1", LatestMessageSeq: 9}}, nil
}
func (p *syncShadowProbe) AdvanceGroupCheckpoint(_, _, group string, seq uint64) (*model.GroupSyncCheckpoint, error) {
	p.advances.Add(1)
	return &model.GroupSyncCheckpoint{GroupUUID: group, PulledMessageSeq: seq}, nil
}

func TestSyncShadowComparesOnlyReadOperations(t *testing.T) {
	primary, shadow := &syncShadowProbe{}, &syncShadowProbe{}
	application := newSyncShadowApplication(primary, shadow, nil)
	_, _ = application.List("U1", 7, 20)
	_, _ = application.GetCheckpoint("U1", "web-1")
	_, _ = application.ListGroupCheckpoints("U1", "web-1", []string{"G1"})
	_, _ = application.AdvanceCheckpoint("U1", "web-1", 8)
	_, _ = application.AdvanceGroupCheckpoint("U1", "web-1", "G1", 9)
	application.Wait()
	if primary.reads.Load() != 3 || shadow.reads.Load() != 3 {
		t.Fatalf("unexpected read calls: primary=%d shadow=%d", primary.reads.Load(), shadow.reads.Load())
	}
	if primary.advances.Load() != 2 || shadow.advances.Load() != 0 {
		t.Fatalf("checkpoint writes must be primary-only: primary=%d shadow=%d", primary.advances.Load(), shadow.advances.Load())
	}
}
