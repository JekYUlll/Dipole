package shadow

import (
	"context"
	"errors"
	"testing"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/prometheus/client_golang/prometheus"
)

type syncHydratorStub struct {
	messages map[string]*model.Message
	err      error
	block    <-chan struct{}
}

func TestSyncMessageHydratorExportsComparisonMetrics(t *testing.T) {
	message := &model.Message{UUID: "M1"}
	hydrator := NewSyncMessageHydrator(&syncHydratorStub{messages: map[string]*model.Message{"M1": message}}, &syncHydratorStub{messages: map[string]*model.Message{"M1": message}}, nil)
	registry := prometheus.NewRegistry()
	registry.MustRegister(hydrator)
	if _, err := hydrator.Hydrate(context.Background(), []model.SyncMessageLocator{{MessageUUID: "M1", ConversationKey: "group:G1", MessageSeq: 1}}); err != nil {
		t.Fatal(err)
	}
	hydrator.Wait()
	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	found := false
	for _, family := range families {
		if family.GetName() != "dipole_sync_hydration_shadow_total" {
			continue
		}
		for _, metric := range family.Metric {
			if len(metric.Label) == 1 && metric.Label[0].GetValue() == "match" && metric.Counter.GetValue() == 1 {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("matching Sync hydration metric was not exported")
	}
}

func (s *syncHydratorStub) Hydrate(context.Context, []model.SyncMessageLocator) (map[string]*model.Message, error) {
	if s.block != nil {
		<-s.block
	}
	return s.messages, s.err
}

func TestSyncMessageHydratorReturnsPrimaryAndComparesShadow(t *testing.T) {
	locator := model.SyncMessageLocator{MessageUUID: "M1", ConversationKey: "group:G1", MessageSeq: 1}
	message := &model.Message{UUID: "M1", ConversationKey: "group:G1", Seq: 1, Content: "same"}
	comparisons := make(chan SyncHydrationComparison, 1)
	hydrator := NewSyncMessageHydrator(&syncHydratorStub{messages: map[string]*model.Message{"M1": message}}, &syncHydratorStub{messages: map[string]*model.Message{"M1": message}}, func(value SyncHydrationComparison) { comparisons <- value })
	result, err := hydrator.Hydrate(context.Background(), []model.SyncMessageLocator{locator})
	if err != nil || result["M1"] == nil {
		t.Fatalf("primary result=%+v err=%v", result, err)
	}
	hydrator.Wait()
	comparison := <-comparisons
	if !comparison.Match || comparison.PrimaryCount != 1 || comparison.ShadowCount != 1 {
		t.Fatalf("comparison=%+v", comparison)
	}
}

func TestSyncMessageHydratorIsolatesShadowFailureAndMismatch(t *testing.T) {
	locator := model.SyncMessageLocator{MessageUUID: "M1", ConversationKey: "group:G1", MessageSeq: 1}
	primary := &syncHydratorStub{messages: map[string]*model.Message{"M1": {UUID: "M1", ConversationKey: "group:G1", Seq: 1, Content: "primary"}}}
	for name, shadow := range map[string]*syncHydratorStub{
		"error":    {err: errors.New("unavailable")},
		"mismatch": {messages: map[string]*model.Message{"M1": {UUID: "M1", ConversationKey: "group:G1", Seq: 1, Content: "different"}}},
	} {
		t.Run(name, func(t *testing.T) {
			comparisons := make(chan SyncHydrationComparison, 1)
			hydrator := NewSyncMessageHydrator(primary, shadow, func(value SyncHydrationComparison) { comparisons <- value })
			if _, err := hydrator.Hydrate(context.Background(), []model.SyncMessageLocator{locator}); err != nil {
				t.Fatalf("shadow affected primary: %v", err)
			}
			hydrator.Wait()
			if comparison := <-comparisons; comparison.Match || (name == "error" && comparison.ShadowError == "") {
				t.Fatalf("comparison=%+v", comparison)
			}
		})
	}
}

func TestSyncMessageHydratorBoundsAsyncWork(t *testing.T) {
	release := make(chan struct{})
	observed := make(chan SyncHydrationComparison, 2)
	primary := &syncHydratorStub{messages: map[string]*model.Message{"M1": {UUID: "M1"}}}
	hydrator := newSyncMessageHydrator(primary, &syncHydratorStub{messages: map[string]*model.Message{"M1": {UUID: "M1"}}, block: release}, func(value SyncHydrationComparison) { observed <- value }, 1)
	locators := []model.SyncMessageLocator{{MessageUUID: "M1", ConversationKey: "group:G1", MessageSeq: 1}}
	if _, err := hydrator.Hydrate(context.Background(), locators); err != nil {
		t.Fatal(err)
	}
	if _, err := hydrator.Hydrate(context.Background(), locators); err != nil {
		t.Fatal(err)
	}
	comparison := <-observed
	if !comparison.Skipped || comparison.SkipReason != "shadow_capacity_exhausted" {
		t.Fatalf("comparison=%+v", comparison)
	}
	close(release)
	hydrator.Wait()
}
