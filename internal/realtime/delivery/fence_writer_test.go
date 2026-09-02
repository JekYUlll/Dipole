package delivery

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisAuthorityFenceWriterTransitionLifecycle(t *testing.T) {
	now := time.Date(2026, 8, 28, 2, 0, 0, 0, time.UTC)
	server := miniredis.RunT(t)
	server.SetTime(now)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	writer, err := NewRedisAuthorityFenceWriter(
		client, "dipole:test:fence", "dipole:test:fence:receipt:", 24*time.Hour, func() time.Time { return now },
	)
	if err != nil {
		t.Fatalf("NewRedisAuthorityFenceWriter(): %v", err)
	}

	bootstrap, err := writer.Apply(context.Background(), FenceTransitionRequest{
		TransitionID: "T-bootstrap", Action: FenceTransitionBootstrap,
		OperatorID: "operator-a", Reason: "enable guarded Go baseline",
		TargetAuthority: AuthorityGo, LeaseUntil: now.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	assertFenceReceipt(t, bootstrap, FenceTransitionBootstrap, AuthorityGo, FencePhaseActive, 1)
	recovered, err := writer.GetReceipt(context.Background(), "T-bootstrap")
	if err != nil || recovered != bootstrap {
		t.Fatalf("GetReceipt() = %+v, %v", recovered, err)
	}
	if server.TTL("dipole:test:fence") <= 0 || server.TTL("dipole:test:fence:receipt:T-bootstrap") <= 0 {
		t.Fatal("lease and transition receipt must both have bounded Redis retention")
	}
	if raw, err := client.Get(context.Background(), "dipole:test:fence:receipt:T-bootstrap").Result(); err != nil ||
		strings.Contains(raw, "enable guarded Go baseline") {
		t.Fatalf("receipt must persist only the reason digest: error=%v", err)
	}
	current := readFenceRecordForTest(t, client, "dipole:test:fence")
	if current.Authority != AuthorityGo || current.Phase != FencePhaseActive || current.Epoch != 1 {
		t.Fatalf("bootstrap record = %+v", current)
	}

	frozen, err := writer.Apply(context.Background(), FenceTransitionRequest{
		TransitionID: "T-freeze", Action: FenceTransitionFreeze,
		OperatorID: "operator-a", Reason: "prepare C++ authority",
		ExpectedSHA256: bootstrap.NextSHA256, LeaseUntil: now.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("freeze: %v", err)
	}
	assertFenceReceipt(t, frozen, FenceTransitionFreeze, AuthorityGo, FencePhaseFrozen, 2)

	activated, err := writer.Apply(context.Background(), FenceTransitionRequest{
		TransitionID: "T-activate-cpp", Action: FenceTransitionActivate,
		OperatorID: "operator-b", Reason: "C++ readers prepared",
		ExpectedSHA256: frozen.NextSHA256, TargetAuthority: AuthorityCPP,
		LeaseUntil: now.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("activate C++: %v", err)
	}
	assertFenceReceipt(t, activated, FenceTransitionActivate, AuthorityCPP, FencePhaseActive, 2)

	renewed, err := writer.Apply(context.Background(), FenceTransitionRequest{
		TransitionID: "T-renew-cpp", Action: FenceTransitionRenew,
		OperatorID: "operator-b", Reason: "extend stable observation",
		ExpectedSHA256: activated.NextSHA256, LeaseUntil: now.Add(20 * time.Minute),
	})
	if err != nil {
		t.Fatalf("renew C++: %v", err)
	}
	assertFenceReceipt(t, renewed, FenceTransitionRenew, AuthorityCPP, FencePhaseActive, 2)
	if renewed.PreviousSHA256 != activated.NextSHA256 || renewed.NextSHA256 == activated.NextSHA256 {
		t.Fatal("renewal must bind the previous lease and produce a new lease hash")
	}
}

func TestRedisAuthorityFenceWriterGetReceiptFailsClosed(t *testing.T) {
	now := time.Date(2026, 8, 28, 2, 0, 0, 0, time.UTC)
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	writer, err := NewRedisAuthorityFenceWriter(
		client, "dipole:test:fence", "dipole:test:fence:receipt:", time.Hour, func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.GetReceipt(context.Background(), "missing"); err == nil {
		t.Fatal("missing receipt must fail")
	}
	server.Set("dipole:test:fence:receipt:T-bad", `{"schema_version":"dipole.realtime.delivery-fence-receipt.v1","transition_id":"T-bad","transition_id":"T-other"}`)
	if _, err := writer.GetReceipt(context.Background(), "T-bad"); err == nil {
		t.Fatal("duplicate or malformed receipt must fail")
	}
	if _, err := writer.GetReceipt(context.Background(), "../escape"); err == nil {
		t.Fatal("invalid transition ID must fail before Redis lookup")
	}
}

func TestRedisAuthorityFenceWriterIsIdempotentAndRejectsDrift(t *testing.T) {
	now := time.Date(2026, 8, 28, 2, 0, 0, 0, time.UTC)
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	writer, err := NewRedisAuthorityFenceWriter(
		client, "dipole:test:fence", "dipole:test:fence:receipt:", time.Hour, func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}
	request := FenceTransitionRequest{
		TransitionID: "T-idempotent", Action: FenceTransitionBootstrap,
		OperatorID: "operator-a", Reason: "baseline",
		TargetAuthority: AuthorityGo, LeaseUntil: now.Add(10 * time.Minute),
	}
	first, err := writer.Apply(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Apply(context.Background(), FenceTransitionRequest{
		TransitionID: "T-direct-switch", Action: FenceTransitionActivate,
		OperatorID: "operator-a", Reason: "skip freeze",
		ExpectedSHA256: first.NextSHA256, TargetAuthority: AuthorityCPP,
		LeaseUntil: now.Add(10 * time.Minute),
	}); err == nil {
		t.Fatal("active-to-active authority switch must fail")
	}
	now = now.Add(20 * time.Minute)
	second, err := writer.Apply(context.Background(), request)
	if err != nil || second != first {
		t.Fatalf("idempotent replay = %+v, %v", second, err)
	}

	drifted := request
	drifted.Reason = "different request"
	if _, err := writer.Apply(context.Background(), drifted); err == nil {
		t.Fatal("transition ID request drift must fail")
	}
	if _, err := writer.Apply(context.Background(), FenceTransitionRequest{
		TransitionID: "T-stale", Action: FenceTransitionFreeze,
		OperatorID: "operator-a", Reason: "stale",
		ExpectedSHA256: strings.Repeat("0", 64), LeaseUntil: now.Add(10 * time.Minute),
	}); err == nil {
		t.Fatal("stale expected lease hash must fail")
	}
}

func TestRedisAuthorityFenceWriterRejectsUnsafeTransitions(t *testing.T) {
	now := time.Date(2026, 8, 28, 2, 0, 0, 0, time.UTC)
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	writer, err := NewRedisAuthorityFenceWriter(
		client, "dipole:test:fence", "dipole:test:fence:receipt:", time.Hour, func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []FenceTransitionRequest{
		{TransitionID: "T-cpp-bootstrap", Action: FenceTransitionBootstrap, OperatorID: "operator-a", Reason: "unsafe", TargetAuthority: AuthorityCPP, LeaseUntil: now.Add(time.Minute)},
		{TransitionID: "T-short", Action: FenceTransitionBootstrap, OperatorID: "operator-a", Reason: "short", TargetAuthority: AuthorityGo, LeaseUntil: now.Add(time.Second)},
		{TransitionID: "T-long", Action: FenceTransitionBootstrap, OperatorID: "operator-a", Reason: "long", TargetAuthority: AuthorityGo, LeaseUntil: now.Add(2 * time.Hour)},
	} {
		if _, err := writer.Apply(context.Background(), request); err == nil {
			t.Fatalf("unsafe transition %q must fail", request.TransitionID)
		}
	}
}

func assertFenceReceipt(t *testing.T, receipt FenceTransitionReceipt, action FenceTransitionAction, authority Authority, phase FencePhase, epoch uint64) {
	t.Helper()
	if receipt.SchemaVersion != FenceTransitionReceiptSchemaV1 || receipt.Action != action ||
		receipt.Authority != authority || receipt.Phase != phase || receipt.Epoch != epoch ||
		receipt.RequestSHA256 == "" || receipt.NextSHA256 == "" || receipt.ReasonSHA256 == "" {
		t.Fatalf("receipt = %+v", receipt)
	}
}

func readFenceRecordForTest(t *testing.T, client redis.Cmdable, key string) FenceRecord {
	t.Helper()
	payload, err := client.Get(context.Background(), key).Bytes()
	if err != nil {
		t.Fatalf("read fence record: %v", err)
	}
	record, err := decodeFenceRecord(payload)
	if err != nil {
		t.Fatalf("decode fence record: %v", err)
	}
	return record
}
