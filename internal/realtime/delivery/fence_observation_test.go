package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisObservedAuthorityFenceWritesAuthorizedObservationWithTTL(t *testing.T) {
	now := time.Date(2026, 8, 28, 2, 0, 0, 0, time.UTC)
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	record := FenceRecord{
		SchemaVersion: FenceSchemaV1, Epoch: 7, Authority: AuthorityGo,
		Phase: FencePhaseActive, LeaseUntilUnixMS: now.Add(time.Minute).UnixMilli(),
	}
	writeFenceRecord(t, client, "dipole:test:authority", record)
	reader, err := NewRedisAuthorityFence(client, "dipole:test:authority", 7, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	fence, err := NewRedisObservedAuthorityFence(
		reader, client, "dipole:test:authority:observation:", "gateway", "gateway-a", 15*time.Second,
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := fence.Assert(context.Background(), AuthorityGo); err != nil {
		t.Fatalf("Assert(): %v", err)
	}
	observation := readFenceObservation(t, client, "dipole:test:authority:observation:gateway:gateway-a")
	if observation.Status != FenceObservationAuthorized || observation.ReasonCode != FenceReasonAuthorized {
		t.Fatalf("unexpected observation: %+v", observation)
	}
	if observation.ExpectedAuthority != AuthorityGo || observation.ExpectedEpoch != 7 || observation.ObservedEpoch != 7 {
		t.Fatalf("unexpected authority observation: %+v", observation)
	}
	rawLease, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if observation.ObservedLeaseSHA256 != hashBytes(rawLease) {
		t.Fatalf("observed lease SHA-256 = %q", observation.ObservedLeaseSHA256)
	}
	if observation.ExpiresAtUnixMS != now.Add(15*time.Second).UnixMilli() {
		t.Fatalf("expires_at_unix_ms = %d", observation.ExpiresAtUnixMS)
	}
	if ttl := server.TTL("dipole:test:authority:observation:gateway:gateway-a"); ttl != 15*time.Second {
		t.Fatalf("TTL = %s", ttl)
	}
}

func TestRedisObservedAuthorityFenceRecordsInvalidLeaseReason(t *testing.T) {
	now := time.Date(2026, 8, 28, 2, 0, 0, 0, time.UTC)
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	server.Set("dipole:test:authority", `{"schema_version":"dipole.realtime.delivery-fence.v1","epoch":10,"authority":"go","phase":"active","lease_until_unix_ms":9999999999999,"extra":true}`)
	reader, err := NewRedisAuthorityFence(client, "dipole:test:authority", 10, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	fence, err := NewRedisObservedAuthorityFence(
		reader, client, "dipole:test:authority:observation:", "gateway", "gateway-invalid", 15*time.Second,
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := fence.Assert(context.Background(), AuthorityGo); err == nil {
		t.Fatal("invalid lease must fail closed")
	}
	observation := readFenceObservation(t, client, "dipole:test:authority:observation:gateway:gateway-invalid")
	if observation.Status != FenceObservationDenied || observation.ReasonCode != FenceReasonInvalidRecord {
		t.Fatalf("unexpected invalid-record observation: %+v", observation)
	}
	if len(observation.ObservedLeaseSHA256) != 64 {
		t.Fatalf("invalid lease SHA-256 = %q", observation.ObservedLeaseSHA256)
	}
}

func TestRedisObservedAuthorityFenceRecordsDenial(t *testing.T) {
	now := time.Date(2026, 8, 28, 2, 0, 0, 0, time.UTC)
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	writeFenceRecord(t, client, "dipole:test:authority", FenceRecord{
		SchemaVersion: FenceSchemaV1, Epoch: 8, Authority: AuthorityGo,
		Phase: FencePhaseFrozen, LeaseUntilUnixMS: now.Add(time.Minute).UnixMilli(),
	})
	reader, err := NewRedisAuthorityFence(client, "dipole:test:authority", 8, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	fence, err := NewRedisObservedAuthorityFence(
		reader, client, "dipole:test:authority:observation:", "gateway", "gateway-b", 15*time.Second,
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := fence.Assert(context.Background(), AuthorityGo); err == nil {
		t.Fatal("frozen authority must fail closed")
	}
	observation := readFenceObservation(t, client, "dipole:test:authority:observation:gateway:gateway-b")
	if observation.Status != FenceObservationDenied || observation.ReasonCode != FenceReasonFrozen {
		t.Fatalf("unexpected observation: %+v", observation)
	}
	if observation.ObservedPhase != FencePhaseFrozen || observation.ObservedEpoch != 8 {
		t.Fatalf("missing observed lease state: %+v", observation)
	}
}

func TestRedisObservedAuthorityFenceFailsClosedWhenObservationCannotBeWritten(t *testing.T) {
	now := time.Date(2026, 8, 28, 2, 0, 0, 0, time.UTC)
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	writeFenceRecord(t, client, "dipole:test:authority", FenceRecord{
		SchemaVersion: FenceSchemaV1, Epoch: 9, Authority: AuthorityGo,
		Phase: FencePhaseActive, LeaseUntilUnixMS: now.Add(time.Minute).UnixMilli(),
	})
	reader, err := NewRedisAuthorityFence(client, "dipole:test:authority", 9, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	fence, err := NewRedisObservedAuthorityFence(
		reader, client, "dipole:test:authority:observation:", "gateway", "gateway-c", 15*time.Second,
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}
	client.AddHook(failObservationSetHook{})

	if err := fence.Assert(context.Background(), AuthorityGo); err == nil {
		t.Fatal("observation write failure must fail closed")
	}
}

type failObservationSetHook struct{}

func (failObservationSetHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (failObservationSetHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if cmd.Name() == "set" {
			return errors.New("observation unavailable")
		}
		return next(ctx, cmd)
	}
}

func (failObservationSetHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func TestNewRedisObservedAuthorityFenceValidatesIdentityAndTTL(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	reader, err := NewRedisAuthorityFence(client, "key", 1, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, prefix, component, observer string
		ttl                               time.Duration
	}{
		{name: "empty prefix", component: "gateway", observer: "node-a", ttl: 15 * time.Second},
		{name: "invalid component", prefix: "prefix:", component: "bad/component", observer: "node-a", ttl: 15 * time.Second},
		{name: "invalid observer", prefix: "prefix:", component: "gateway", observer: "bad observer", ttl: 15 * time.Second},
		{name: "short ttl", prefix: "prefix:", component: "gateway", observer: "node-a", ttl: 4 * time.Second},
		{name: "long ttl", prefix: "prefix:", component: "gateway", observer: "node-a", ttl: 61 * time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewRedisObservedAuthorityFence(reader, client, test.prefix, test.component, test.observer, test.ttl, time.Now); err == nil {
				t.Fatal("invalid observation configuration must fail")
			}
		})
	}
}

type countingFence struct {
	calls atomic.Int64
}

func (f *countingFence) Assert(context.Context, Authority) error {
	f.calls.Add(1)
	return nil
}

func TestRunAuthorityFenceHeartbeatChecksWithoutMessagesAndStops(t *testing.T) {
	fence := &countingFence{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		RunAuthorityFenceHeartbeat(ctx, fence, AuthorityGo, 5*time.Millisecond, 20*time.Millisecond, nil)
	}()
	eventually(t, time.Second, func() bool { return fence.calls.Load() >= 2 })
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("heartbeat did not stop")
	}
	stoppedAt := fence.calls.Load()
	time.Sleep(20 * time.Millisecond)
	if fence.calls.Load() != stoppedAt {
		t.Fatal("heartbeat continued after cancellation")
	}
}

func readFenceObservation(t *testing.T, client redis.Cmdable, key string) FenceObservation {
	t.Helper()
	payload, err := client.Get(context.Background(), key).Bytes()
	if err != nil {
		t.Fatalf("read observation: %v", err)
	}
	var observation FenceObservation
	if err := json.Unmarshal(payload, &observation); err != nil {
		t.Fatalf("decode observation: %v", err)
	}
	return observation
}

func eventually(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition was not met")
}
