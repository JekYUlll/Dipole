package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const FenceObservationSchemaV1 = "dipole.realtime.delivery-fence-observation.v1"

type FenceObservationStatus string
type FenceReasonCode string

const (
	FenceObservationAuthorized FenceObservationStatus = "authorized"
	FenceObservationDenied     FenceObservationStatus = "denied"

	FenceReasonAuthorized        FenceReasonCode = "authorized"
	FenceReasonRedisUnavailable  FenceReasonCode = "redis_unavailable"
	FenceReasonMissing           FenceReasonCode = "missing"
	FenceReasonInvalidRecord     FenceReasonCode = "invalid_record"
	FenceReasonEpochMismatch     FenceReasonCode = "epoch_mismatch"
	FenceReasonFrozen            FenceReasonCode = "frozen"
	FenceReasonAuthorityMismatch FenceReasonCode = "authority_mismatch"
	FenceReasonExpired           FenceReasonCode = "expired"
)

type FenceObservation struct {
	SchemaVersion            string                 `json:"schema_version"`
	ObserverID               string                 `json:"observer_id"`
	Component                string                 `json:"component"`
	ExpectedAuthority        Authority              `json:"expected_authority"`
	ExpectedEpoch            uint64                 `json:"expected_epoch"`
	ObservedAuthority        Authority              `json:"observed_authority"`
	ObservedEpoch            uint64                 `json:"observed_epoch"`
	ObservedPhase            FencePhase             `json:"observed_phase"`
	ObservedLeaseUntilUnixMS int64                  `json:"observed_lease_until_unix_ms"`
	ObservedLeaseSHA256      string                 `json:"observed_lease_sha256"`
	Status                   FenceObservationStatus `json:"status"`
	ReasonCode               FenceReasonCode        `json:"reason_code"`
	ObservedAtUnixMS         int64                  `json:"observed_at_unix_ms"`
	ExpiresAtUnixMS          int64                  `json:"expires_at_unix_ms"`
}

type RedisObservedAuthorityFence struct {
	reader            *RedisAuthorityFence
	client            redis.Cmdable
	observationPrefix string
	component         string
	observerID        string
	ttl               time.Duration
	now               func() time.Time
}

func NewRedisObservedAuthorityFence(
	reader *RedisAuthorityFence,
	client redis.Cmdable,
	observationPrefix, component, observerID string,
	ttl time.Duration,
	now func() time.Time,
) (*RedisObservedAuthorityFence, error) {
	observationPrefix = strings.TrimSpace(observationPrefix)
	component = strings.TrimSpace(component)
	observerID = strings.TrimSpace(observerID)
	if reader == nil || client == nil || observationPrefix == "" || now == nil {
		return nil, fmt.Errorf("delivery authority observation configuration is invalid")
	}
	if !fenceTransitionIDPattern.MatchString(component) || !fenceTransitionIDPattern.MatchString(observerID) {
		return nil, fmt.Errorf("delivery authority observation identity is invalid")
	}
	if ttl < 5*time.Second || ttl > time.Minute {
		return nil, fmt.Errorf("delivery authority observation TTL must be between 5 seconds and 1 minute")
	}
	return &RedisObservedAuthorityFence{
		reader: reader, client: client, observationPrefix: observationPrefix,
		component: component, observerID: observerID, ttl: ttl, now: now,
	}, nil
}

func (f *RedisObservedAuthorityFence) Assert(ctx context.Context, local Authority) error {
	record, leaseSHA256, reason, fenceErr := f.reader.inspect(ctx, local)
	now := f.now().UTC()
	status := FenceObservationAuthorized
	if fenceErr != nil {
		status = FenceObservationDenied
	}
	observation := FenceObservation{
		SchemaVersion: FenceObservationSchemaV1, ObserverID: f.observerID, Component: f.component,
		ExpectedAuthority: local, ExpectedEpoch: f.reader.expectedEpoch,
		ObservedAuthority: record.Authority, ObservedEpoch: record.Epoch, ObservedPhase: record.Phase,
		ObservedLeaseUntilUnixMS: record.LeaseUntilUnixMS, ObservedLeaseSHA256: leaseSHA256,
		Status: status, ReasonCode: reason, ObservedAtUnixMS: now.UnixMilli(),
		ExpiresAtUnixMS: now.Add(f.ttl).UnixMilli(),
	}
	payload, err := json.Marshal(observation)
	if err != nil {
		return errors.Join(fenceErr, fmt.Errorf("encode delivery authority observation: %w", err))
	}
	key := f.observationPrefix + f.component + ":" + f.observerID
	if err := f.client.Set(ctx, key, payload, f.ttl).Err(); err != nil {
		return errors.Join(fenceErr, fmt.Errorf("write delivery authority observation: %w", err))
	}
	return fenceErr
}

func RunAuthorityFenceHeartbeat(
	ctx context.Context,
	fence AuthorityFence,
	authority Authority,
	interval, timeout time.Duration,
	onError func(error),
) {
	if ctx == nil || fence == nil || interval <= 0 || timeout <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkCtx, cancel := context.WithTimeout(ctx, timeout)
			err := fence.Assert(checkCtx, authority)
			cancel()
			if err != nil && onError != nil {
				onError(err)
			}
		}
	}
}
