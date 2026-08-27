package bootstrap

import (
	"sync"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
)

type shadowProbeApplication struct {
	stubMessageApplication
	mu       sync.Mutex
	commands int
	queries  int
	mismatch bool
}

func (p *shadowProbeApplication) SendDirectMessage(senderUUID, targetUUID, content, clientMessageID string) (*model.Message, error) {
	p.recordCommand()
	return p.stubMessageApplication.SendDirectMessage(senderUUID, targetUUID, content, clientMessageID)
}

func (p *shadowProbeApplication) SendGroupMessage(senderUUID, groupUUID, content, clientMessageID string) (*model.Message, []string, error) {
	p.recordCommand()
	return p.stubMessageApplication.SendGroupMessage(senderUUID, groupUUID, content, clientMessageID)
}

func (p *shadowProbeApplication) SendDirectFileMessage(senderUUID, targetUUID, fileUUID, clientMessageID string) (*model.Message, error) {
	p.recordCommand()
	return p.stubMessageApplication.SendDirectFileMessage(senderUUID, targetUUID, fileUUID, clientMessageID)
}

func (p *shadowProbeApplication) SendGroupFileMessage(senderUUID, groupUUID, fileUUID, clientMessageID string) (*model.Message, []string, error) {
	p.recordCommand()
	return p.stubMessageApplication.SendGroupFileMessage(senderUUID, groupUUID, fileUUID, clientMessageID)
}

func (p *shadowProbeApplication) ListDirectMessages(userUUID, targetUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	p.recordQuery()
	messages, err := p.stubMessageApplication.ListDirectMessages(userUUID, targetUUID, beforeID, limit)
	if p.mismatch && len(messages) > 0 {
		messages[0].Content = "shadow-mismatch"
	}
	return messages, err
}

func (p *shadowProbeApplication) ListDirectMessagesBeforeSeq(userUUID, targetUUID string, beforeSeq uint64, limit int) ([]*model.Message, error) {
	p.recordQuery()
	return p.stubMessageApplication.ListDirectMessagesBeforeSeq(userUUID, targetUUID, beforeSeq, limit)
}

func (p *shadowProbeApplication) ListDirectMessagesAfterSeq(userUUID, targetUUID string, afterSeq uint64, limit int) ([]*model.Message, error) {
	p.recordQuery()
	return p.stubMessageApplication.ListDirectMessagesAfterSeq(userUUID, targetUUID, afterSeq, limit)
}

func (p *shadowProbeApplication) ListGroupMessages(userUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	p.recordQuery()
	return p.stubMessageApplication.ListGroupMessages(userUUID, groupUUID, beforeID, limit)
}

func (p *shadowProbeApplication) ListGroupMessagesBeforeSeq(userUUID, groupUUID string, beforeSeq uint64, limit int) ([]*model.Message, error) {
	p.recordQuery()
	return p.stubMessageApplication.ListGroupMessagesBeforeSeq(userUUID, groupUUID, beforeSeq, limit)
}

func (p *shadowProbeApplication) ListGroupMessagesAfter(userUUID, groupUUID string, afterID uint, limit int) ([]*model.Message, error) {
	p.recordQuery()
	return p.stubMessageApplication.ListGroupMessagesAfter(userUUID, groupUUID, afterID, limit)
}

func (p *shadowProbeApplication) ListGroupMessagesAfterSeq(userUUID, groupUUID string, afterSeq uint64, limit int) ([]*model.Message, error) {
	p.recordQuery()
	return p.stubMessageApplication.ListGroupMessagesAfterSeq(userUUID, groupUUID, afterSeq, limit)
}

func (p *shadowProbeApplication) ListOfflineMessages(userUUID string, afterID uint, limit int) ([]*model.Message, error) {
	p.recordQuery()
	return p.stubMessageApplication.ListOfflineMessages(userUUID, afterID, limit)
}

func (p *shadowProbeApplication) recordCommand() {
	p.mu.Lock()
	p.commands++
	p.mu.Unlock()
}

func (p *shadowProbeApplication) recordQuery() {
	p.mu.Lock()
	p.queries++
	p.mu.Unlock()
}

func (p *shadowProbeApplication) counts() (int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.commands, p.queries
}

func TestMessageShadowApplicationNeverDuplicatesCommands(t *testing.T) {
	primary := &shadowProbeApplication{}
	shadow := &shadowProbeApplication{}
	comparisons := make(chan messageShadowComparison, 8)
	application := newMessageShadowApplication(primary, shadow, func(comparison messageShadowComparison) {
		comparisons <- comparison
	})

	runMessageApplicationContract(t, application)
	application.Wait()
	close(comparisons)

	primaryCommands, primaryQueries := primary.counts()
	shadowCommands, shadowQueries := shadow.counts()
	if primaryCommands != 4 || primaryQueries != 8 {
		t.Fatalf("unexpected primary calls: commands=%d queries=%d", primaryCommands, primaryQueries)
	}
	if shadowCommands != 0 || shadowQueries != 8 {
		t.Fatalf("shadow must remain query-only: commands=%d queries=%d", shadowCommands, shadowQueries)
	}
	for comparison := range comparisons {
		if !comparison.Match {
			t.Fatalf("expected matching comparison: %+v", comparison)
		}
	}
}

func TestMessageShadowMismatchDoesNotChangePrimaryResult(t *testing.T) {
	primary := &shadowProbeApplication{}
	shadow := &shadowProbeApplication{mismatch: true}
	comparisons := make(chan messageShadowComparison, 1)
	application := newMessageShadowApplication(primary, shadow, func(comparison messageShadowComparison) {
		comparisons <- comparison
	})

	messages, err := application.ListDirectMessages("U1", "U2", 40, 20)
	if err != nil || len(messages) != 1 || messages[0].Content != "direct" {
		t.Fatalf("primary response changed: messages=%+v err=%v", messages, err)
	}
	application.Wait()
	comparison := <-comparisons
	if comparison.Match || comparison.Operation != "list_direct_history" {
		t.Fatalf("expected direct-history mismatch, got %+v", comparison)
	}
}

func TestEqualMessagePagesIgnoresInternalTimestampsAndTimeLocation(t *testing.T) {
	instant := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	local := instant.In(time.FixedZone("CST", 8*60*60))
	primary := []*model.Message{{UUID: "M1", Content: "same", SentAt: instant, CreatedAt: instant, UpdatedAt: instant}}
	shadow := []*model.Message{{UUID: "M1", Content: "same", SentAt: local}}
	if !equalMessagePages(primary, shadow) {
		t.Fatal("expected public message fields to match")
	}
	shadow[0].Content = "different"
	if equalMessagePages(primary, shadow) {
		t.Fatal("expected public content mismatch")
	}
	shadow[0].Content = "same"
	shadow[0].Seq = 2
	if equalMessagePages(primary, shadow) {
		t.Fatal("expected public conversation sequence mismatch")
	}
}
