package delivery

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestAuthorityFenceSharedVectors(t *testing.T) {
	type vector struct {
		Name              string `json:"name"`
		Payload           string `json:"payload"`
		ExpectedAuthority string `json:"expected_authority"`
		ExpectedEpoch     uint64 `json:"expected_epoch"`
		NowUnixMS         int64  `json:"now_unix_ms"`
		Authorized        bool   `json:"authorized"`
	}
	var vectors struct {
		Cases []vector `json:"cases"`
	}
	payload, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "realtime-delivery-fence", "v1", "testdata", "authority.v1.json"))
	if err != nil {
		t.Fatalf("read shared vectors: %v", err)
	}
	if err := json.Unmarshal(payload, &vectors); err != nil {
		t.Fatalf("decode shared vectors: %v", err)
	}
	for _, test := range vectors.Cases {
		t.Run(test.Name, func(t *testing.T) {
			authority, err := ParseAuthority(test.ExpectedAuthority)
			if err != nil {
				t.Fatal(err)
			}
			record, err := decodeFenceRecord([]byte(test.Payload))
			if err == nil {
				_, err = validateFenceRecord(record, authority, test.ExpectedEpoch, time.UnixMilli(test.NowUnixMS))
			}
			if (err == nil) != test.Authorized {
				t.Fatalf("authorized = %v, error = %v", err == nil, err)
			}
		})
	}
}

func TestRedisAuthorityFenceAssert(t *testing.T) {
	now := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	fence, err := NewRedisAuthorityFence(client, "dipole:test:authority", 17, func() time.Time { return now })
	if err != nil {
		t.Fatalf("NewRedisAuthorityFence(): %v", err)
	}
	writeFenceRecord(t, client, "dipole:test:authority", FenceRecord{
		SchemaVersion:    FenceSchemaV1,
		Epoch:            17,
		Authority:        AuthorityGo,
		Phase:            FencePhaseActive,
		LeaseUntilUnixMS: now.Add(time.Minute).UnixMilli(),
	})
	if err := fence.Assert(context.Background(), AuthorityGo); err != nil {
		t.Fatalf("Assert(valid): %v", err)
	}

	for _, test := range []struct {
		name   string
		mutate func(*FenceRecord)
	}{
		{name: "authority mismatch", mutate: func(record *FenceRecord) { record.Authority = AuthorityCPP }},
		{name: "epoch mismatch", mutate: func(record *FenceRecord) { record.Epoch++ }},
		{name: "frozen", mutate: func(record *FenceRecord) { record.Phase = FencePhaseFrozen }},
		{name: "expired", mutate: func(record *FenceRecord) { record.LeaseUntilUnixMS = now.UnixMilli() }},
		{name: "unknown schema", mutate: func(record *FenceRecord) { record.SchemaVersion = "v2" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			record := FenceRecord{
				SchemaVersion:    FenceSchemaV1,
				Epoch:            17,
				Authority:        AuthorityGo,
				Phase:            FencePhaseActive,
				LeaseUntilUnixMS: now.Add(time.Minute).UnixMilli(),
			}
			test.mutate(&record)
			writeFenceRecord(t, client, "dipole:test:authority", record)
			if err := fence.Assert(context.Background(), AuthorityGo); err == nil {
				t.Fatal("Assert() must fail closed")
			}
		})
	}
}

func TestRedisAuthorityFenceRejectsUnavailableOrInvalidRecord(t *testing.T) {
	now := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	fence, err := NewRedisAuthorityFence(client, "dipole:test:authority", 1, func() time.Time { return now })
	if err != nil {
		t.Fatalf("NewRedisAuthorityFence(): %v", err)
	}

	if err := fence.Assert(context.Background(), AuthorityGo); err == nil {
		t.Fatal("missing record must fail closed")
	}
	server.Set("dipole:test:authority", `{"schema_version":"dipole.realtime.delivery-fence.v1","epoch":1,"authority":"go","phase":"active","lease_until_unix_ms":9999999999999,"extra":true}`)
	if err := fence.Assert(context.Background(), AuthorityGo); err == nil {
		t.Fatal("unknown record field must fail closed")
	}
	server.Set("dipole:test:authority", `{`)
	if err := fence.Assert(context.Background(), AuthorityGo); err == nil {
		t.Fatal("malformed record must fail closed")
	}
	server.Set("dipole:test:authority", `{"schema_version":"dipole.realtime.delivery-fence.v1","epoch":1,"authority":"go","authority":"go","phase":"active","lease_until_unix_ms":9999999999999}`)
	if err := fence.Assert(context.Background(), AuthorityGo); err == nil {
		t.Fatal("duplicate record field must fail closed")
	}
	server.Close()
	if err := fence.Assert(context.Background(), AuthorityGo); err == nil {
		t.Fatal("Redis failure must fail closed")
	}
}

func TestNewRedisAuthorityFenceValidatesConfiguration(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	if _, err := NewRedisAuthorityFence(nil, "key", 1, time.Now); err == nil {
		t.Fatal("nil client must fail")
	}
	if _, err := NewRedisAuthorityFence(client, "", 1, time.Now); err == nil {
		t.Fatal("empty key must fail")
	}
	if _, err := NewRedisAuthorityFence(client, "key", 0, time.Now); err == nil {
		t.Fatal("zero epoch must fail")
	}
	if _, err := NewRedisAuthorityFence(client, "key", 1, nil); err == nil {
		t.Fatal("nil clock must fail")
	}
}

func writeFenceRecord(t *testing.T, client redis.Cmdable, key string, record FenceRecord) {
	t.Helper()
	payload, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal fence record: %v", err)
	}
	if err := client.Set(context.Background(), key, payload, 0).Err(); err != nil {
		t.Fatalf("write fence record: %v", err)
	}
}
