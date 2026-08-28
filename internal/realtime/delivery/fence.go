package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const FenceSchemaV1 = "dipole.realtime.delivery-fence.v1"

type FencePhase string

const (
	FencePhaseActive FencePhase = "active"
	FencePhaseFrozen FencePhase = "frozen"
)

type FenceRecord struct {
	SchemaVersion    string     `json:"schema_version"`
	Epoch            uint64     `json:"epoch"`
	Authority        Authority  `json:"authority"`
	Phase            FencePhase `json:"phase"`
	LeaseUntilUnixMS int64      `json:"lease_until_unix_ms"`
}

type AuthorityFence interface {
	Assert(ctx context.Context, local Authority) error
}

type RedisAuthorityFence struct {
	client        redis.Cmdable
	key           string
	expectedEpoch uint64
	now           func() time.Time
}

func NewRedisAuthorityFence(client redis.Cmdable, key string, expectedEpoch uint64, now func() time.Time) (*RedisAuthorityFence, error) {
	if client == nil {
		return nil, fmt.Errorf("delivery authority fence requires Redis client")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("delivery authority fence requires Redis key")
	}
	if expectedEpoch == 0 {
		return nil, fmt.Errorf("delivery authority fence requires non-zero expected epoch")
	}
	if now == nil {
		return nil, fmt.Errorf("delivery authority fence requires clock")
	}
	return &RedisAuthorityFence{client: client, key: key, expectedEpoch: expectedEpoch, now: now}, nil
}

func (f *RedisAuthorityFence) Assert(ctx context.Context, local Authority) error {
	payload, err := f.client.Get(ctx, f.key).Bytes()
	if err != nil {
		return fmt.Errorf("read delivery authority fence: %w", err)
	}
	record, err := decodeFenceRecord(payload)
	if err != nil {
		return err
	}
	if record.Epoch != f.expectedEpoch {
		return fmt.Errorf("delivery authority fence epoch %d does not match expected epoch %d", record.Epoch, f.expectedEpoch)
	}
	if record.Phase != FencePhaseActive {
		return fmt.Errorf("delivery authority fence phase %q denies delivery", record.Phase)
	}
	if record.Authority != local {
		return fmt.Errorf("delivery authority fence grants %q, local authority is %q", record.Authority, local)
	}
	if record.LeaseUntilUnixMS <= f.now().UnixMilli() {
		return fmt.Errorf("delivery authority fence lease expired")
	}
	return nil
}

func decodeFenceRecord(payload []byte) (FenceRecord, error) {
	if err := rejectDuplicateFenceFields(payload); err != nil {
		return FenceRecord{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var record FenceRecord
	if err := decoder.Decode(&record); err != nil {
		return FenceRecord{}, fmt.Errorf("decode delivery authority fence: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return FenceRecord{}, fmt.Errorf("decode delivery authority fence: trailing data")
	}
	if record.SchemaVersion != FenceSchemaV1 {
		return FenceRecord{}, fmt.Errorf("unsupported delivery authority fence schema %q", record.SchemaVersion)
	}
	if record.Epoch == 0 {
		return FenceRecord{}, fmt.Errorf("delivery authority fence epoch must be non-zero")
	}
	if _, err := ParseAuthority(string(record.Authority)); err != nil {
		return FenceRecord{}, err
	}
	if record.Phase != FencePhaseActive && record.Phase != FencePhaseFrozen {
		return FenceRecord{}, fmt.Errorf("unsupported delivery authority fence phase %q", record.Phase)
	}
	if record.LeaseUntilUnixMS <= 0 {
		return FenceRecord{}, fmt.Errorf("delivery authority fence lease must be positive")
	}
	return record, nil
}

func rejectDuplicateFenceFields(payload []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("decode delivery authority fence: %w", err)
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return fmt.Errorf("decode delivery authority fence: expected object")
	}
	seen := make(map[string]struct{}, 5)
	for decoder.More() {
		fieldToken, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("decode delivery authority fence: %w", err)
		}
		field, ok := fieldToken.(string)
		if !ok {
			return fmt.Errorf("decode delivery authority fence: expected field name")
		}
		if _, exists := seen[field]; exists {
			return fmt.Errorf("decode delivery authority fence: duplicate field %q", field)
		}
		seen[field] = struct{}{}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return fmt.Errorf("decode delivery authority fence: %w", err)
		}
	}
	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("decode delivery authority fence: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("decode delivery authority fence: trailing data")
	}
	return nil
}
