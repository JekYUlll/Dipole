package shadow

import (
	"context"
	"errors"
	"testing"

	"github.com/JekYUlll/Dipole/internal/model"
)

type fallbackHydratorStub struct {
	messages map[string]*model.Message
	err      error
	calls    int
}

func (s *fallbackHydratorStub) Hydrate(context.Context, []model.SyncMessageLocator) (map[string]*model.Message, error) {
	s.calls++
	return s.messages, s.err
}

func TestFallbackSyncMessageHydratorUsesCassandraAndFallsBackByLocator(t *testing.T) {
	locators := []model.SyncMessageLocator{{MessageUUID: "M1", ConversationKey: "direct:U1:U2", MessageSeq: 1}}
	primary := &fallbackHydratorStub{messages: map[string]*model.Message{"M1": {UUID: "M1"}}}
	fallback := &fallbackHydratorStub{messages: map[string]*model.Message{"M1": {UUID: "M1"}}}
	var outcomes []string
	hydrator, err := NewFallbackSyncMessageHydrator(primary, fallback, func(value string) { outcomes = append(outcomes, value) })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hydrator.Hydrate(context.Background(), locators); err != nil || fallback.calls != 0 || len(outcomes) != 1 || outcomes[0] != "hit" {
		t.Fatalf("primary route: err=%v fallback_calls=%d outcomes=%v", err, fallback.calls, outcomes)
	}
	primary.err = errors.New("cassandra unavailable")
	if _, err := hydrator.Hydrate(context.Background(), locators); err != nil || fallback.calls != 1 || len(outcomes) != 2 || outcomes[1] != "fallback" {
		t.Fatalf("fallback route: err=%v fallback_calls=%d outcomes=%v", err, fallback.calls, outcomes)
	}
}

func TestFallbackSyncMessageHydratorReturnsFallbackFailure(t *testing.T) {
	hydrator, err := NewFallbackSyncMessageHydrator(&fallbackHydratorStub{err: errors.New("cassandra unavailable")}, &fallbackHydratorStub{err: errors.New("mysql unavailable")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hydrator.Hydrate(context.Background(), nil); err == nil {
		t.Fatal("expected both hydration failures")
	}
}

func TestFallbackSyncMessageHydratorDoesNotFallbackAfterCancellation(t *testing.T) {
	primary := &fallbackHydratorStub{err: errors.New("cassandra unavailable")}
	fallback := &fallbackHydratorStub{messages: map[string]*model.Message{"M1": {UUID: "M1"}}}
	hydrator, err := NewFallbackSyncMessageHydrator(primary, fallback, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := hydrator.Hydrate(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled hydration error = %v", err)
	}
	if fallback.calls != 0 {
		t.Fatalf("fallback was called after cancellation: %d", fallback.calls)
	}
}
