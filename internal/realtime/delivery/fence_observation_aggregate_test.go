package delivery

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisFenceObservationAggregatorBindsEveryExpectedNode(t *testing.T) {
	now := time.Date(2026, 8, 28, 4, 0, 0, 0, time.UTC)
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	transition := validAggregateTransition(now, FencePhaseActive)
	manifest := FenceExpectedNodeManifest{
		SchemaVersion: FenceExpectedNodeManifestSchemaV1,
		ManifestID:    "cutover-a",
		Nodes: []FenceExpectedNode{
			{Component: "realtime-delivery", ObserverID: "cpp-a"},
			{Component: "gateway", ObserverID: "gateway-a"},
		},
	}
	writeAggregateObservation(t, client, "fence:observation:realtime-delivery:cpp-a", validAggregateObservation(now, transition, "realtime-delivery", "cpp-a"), 15*time.Second)
	writeAggregateObservation(t, client, "fence:observation:gateway:gateway-a", validAggregateObservation(now, transition, "gateway", "gateway-a"), 15*time.Second)
	aggregator, err := NewRedisFenceObservationAggregator(client, "fence:observation:", func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	receipt, err := aggregator.Aggregate(context.Background(), manifest, transition)
	if err != nil {
		t.Fatalf("Aggregate(): %v", err)
	}
	if receipt.SchemaVersion != FenceObservationAggregateReceiptSchemaV1 || receipt.Decision != FenceObservationAggregateEligible {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
	if receipt.TransitionID != transition.TransitionID || receipt.LeaseSHA256 != transition.NextSHA256 || receipt.Epoch != transition.Epoch {
		t.Fatalf("receipt lost transition binding: %+v", receipt)
	}
	if !validSHA256(receipt.ManifestSHA256) || len(receipt.Observations) != 2 {
		t.Fatalf("receipt lost manifest observations: %+v", receipt)
	}
	if receipt.Observations[0].Component != "gateway" || receipt.Observations[1].Component != "realtime-delivery" {
		t.Fatalf("observations are not canonical: %+v", receipt.Observations)
	}
}

func TestRedisFenceObservationAggregatorAcceptsFrozenDenials(t *testing.T) {
	now := time.Date(2026, 8, 28, 4, 0, 0, 0, time.UTC)
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	transition := validAggregateTransition(now, FencePhaseFrozen)
	manifest := FenceExpectedNodeManifest{
		SchemaVersion: FenceExpectedNodeManifestSchemaV1,
		ManifestID:    "freeze-a",
		Nodes:         []FenceExpectedNode{{Component: "gateway", ObserverID: "gateway-a", ExpectedAuthority: AuthorityCPP}},
	}
	observation := validAggregateObservation(now, transition, "gateway", "gateway-a")
	observation.ExpectedAuthority = AuthorityCPP
	observation.Status = FenceObservationDenied
	observation.ReasonCode = FenceReasonFrozen
	writeAggregateObservation(t, client, "fence:observation:gateway:gateway-a", observation, 15*time.Second)
	aggregator, err := NewRedisFenceObservationAggregator(client, "fence:observation:", func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	if _, err := aggregator.Aggregate(context.Background(), manifest, transition); err != nil {
		t.Fatalf("Aggregate frozen observations: %v", err)
	}
}

func TestRedisFenceObservationAggregatorFailsClosed(t *testing.T) {
	now := time.Date(2026, 8, 28, 4, 0, 0, 0, time.UTC)
	transition := validAggregateTransition(now, FencePhaseActive)
	manifest := FenceExpectedNodeManifest{
		SchemaVersion: FenceExpectedNodeManifestSchemaV1,
		ManifestID:    "cutover-a",
		Nodes:         []FenceExpectedNode{{Component: "gateway", ObserverID: "gateway-a"}},
	}
	for _, test := range []struct {
		name   string
		mutate func(*FenceObservation)
		ttl    time.Duration
		want   string
	}{
		{name: "denied active", mutate: func(o *FenceObservation) {
			o.Status = FenceObservationDenied
			o.ReasonCode = FenceReasonAuthorityMismatch
		}, ttl: 15 * time.Second, want: "status"},
		{name: "wrong lease hash", mutate: func(o *FenceObservation) { o.ObservedLeaseSHA256 = strings.Repeat("a", 64) }, ttl: 15 * time.Second, want: "lease"},
		{name: "expired payload", mutate: func(o *FenceObservation) { o.ExpiresAtUnixMS = now.Add(-time.Millisecond).UnixMilli() }, ttl: 15 * time.Second, want: "expired"},
		{name: "missing redis ttl", ttl: 0, want: "expired"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := miniredis.RunT(t)
			client := redis.NewClient(&redis.Options{Addr: server.Addr()})
			t.Cleanup(func() { _ = client.Close() })
			observation := validAggregateObservation(now, transition, "gateway", "gateway-a")
			if test.mutate != nil {
				test.mutate(&observation)
			}
			payload, err := json.Marshal(observation)
			if err != nil {
				t.Fatal(err)
			}
			if test.ttl > 0 {
				if err := client.Set(context.Background(), "fence:observation:gateway:gateway-a", payload, test.ttl).Err(); err != nil {
					t.Fatal(err)
				}
			}
			aggregator, err := NewRedisFenceObservationAggregator(client, "fence:observation:", func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			if _, err := aggregator.Aggregate(context.Background(), manifest, transition); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Aggregate() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestRedisFenceObservationAggregatorRejectsInvalidManifest(t *testing.T) {
	now := time.Date(2026, 8, 28, 4, 0, 0, 0, time.UTC)
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	aggregator, err := NewRedisFenceObservationAggregator(client, "fence:observation:", func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	manifest := FenceExpectedNodeManifest{
		SchemaVersion: FenceExpectedNodeManifestSchemaV1,
		ManifestID:    "duplicate-a",
		Nodes: []FenceExpectedNode{
			{Component: "gateway", ObserverID: "gateway-a"},
			{Component: "gateway", ObserverID: "gateway-a"},
		},
	}
	if _, err := aggregator.Aggregate(context.Background(), manifest, validAggregateTransition(now, FencePhaseActive)); err == nil {
		t.Fatal("duplicate expected node must fail")
	}
}

func TestRedisFenceObservationAggregatorRejectsActiveNodePreparedForAnotherAuthority(t *testing.T) {
	now := time.Date(2026, 8, 28, 4, 0, 0, 0, time.UTC)
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	aggregator, err := NewRedisFenceObservationAggregator(client, "fence:observation:", func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	manifest := FenceExpectedNodeManifest{
		SchemaVersion: FenceExpectedNodeManifestSchemaV1,
		ManifestID:    "active-mismatch-a",
		Nodes:         []FenceExpectedNode{{Component: "gateway", ObserverID: "gateway-a", ExpectedAuthority: AuthorityCPP}},
	}
	if _, err := aggregator.Aggregate(context.Background(), manifest, validAggregateTransition(now, FencePhaseActive)); err == nil || !strings.Contains(err.Error(), "active transition") {
		t.Fatalf("Aggregate() error = %v", err)
	}
}

func validAggregateTransition(now time.Time, phase FencePhase) FenceTransitionReceipt {
	action := FenceTransitionActivate
	if phase == FencePhaseFrozen {
		action = FenceTransitionFreeze
	}
	return FenceTransitionReceipt{
		SchemaVersion: FenceTransitionReceiptSchemaV1,
		TransitionID:  "transition-a", RequestSHA256: strings.Repeat("1", 64), Action: action,
		OperatorID: "operator-a", ReasonSHA256: strings.Repeat("2", 64), PreviousSHA256: strings.Repeat("3", 64),
		NextSHA256: strings.Repeat("4", 64), Authority: AuthorityGo, Phase: phase, Epoch: 11,
		LeaseUntilUnixMS: now.Add(time.Minute).UnixMilli(), AppliedAtUnixMS: now.Add(-time.Second).UnixMilli(),
	}
}

func validAggregateObservation(now time.Time, transition FenceTransitionReceipt, component, observerID string) FenceObservation {
	return FenceObservation{
		SchemaVersion: FenceObservationSchemaV1, ObserverID: observerID, Component: component,
		ExpectedAuthority: transition.Authority, ExpectedEpoch: transition.Epoch,
		ObservedAuthority: transition.Authority, ObservedEpoch: transition.Epoch, ObservedPhase: transition.Phase,
		ObservedLeaseUntilUnixMS: transition.LeaseUntilUnixMS, ObservedLeaseSHA256: transition.NextSHA256,
		Status: FenceObservationAuthorized, ReasonCode: FenceReasonAuthorized,
		ObservedAtUnixMS: now.Add(-time.Second).UnixMilli(), ExpiresAtUnixMS: now.Add(14 * time.Second).UnixMilli(),
	}
}

func writeAggregateObservation(t *testing.T, client redis.Cmdable, key string, observation FenceObservation, ttl time.Duration) {
	t.Helper()
	payload, err := json.Marshal(observation)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Set(context.Background(), key, payload, ttl).Err(); err != nil {
		t.Fatal(err)
	}
}
