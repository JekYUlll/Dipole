package delivery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	FenceTransitionSchemaV1        = "dipole.realtime.delivery-fence-transition.v1"
	FenceTransitionReceiptSchemaV1 = "dipole.realtime.delivery-fence-receipt.v1"
)

type FenceTransitionAction string

const (
	FenceTransitionBootstrap FenceTransitionAction = "bootstrap"
	FenceTransitionFreeze    FenceTransitionAction = "freeze"
	FenceTransitionActivate  FenceTransitionAction = "activate"
	FenceTransitionRenew     FenceTransitionAction = "renew"
)

type FenceTransitionRequest struct {
	TransitionID    string
	Action          FenceTransitionAction
	OperatorID      string
	Reason          string
	ExpectedSHA256  string
	TargetAuthority Authority
	LeaseUntil      time.Time
}

type FenceTransitionReceipt struct {
	SchemaVersion    string                `json:"schema_version"`
	TransitionID     string                `json:"transition_id"`
	RequestSHA256    string                `json:"request_sha256"`
	Action           FenceTransitionAction `json:"action"`
	OperatorID       string                `json:"operator_id"`
	ReasonSHA256     string                `json:"reason_sha256"`
	PreviousSHA256   string                `json:"previous_sha256"`
	NextSHA256       string                `json:"next_sha256"`
	Authority        Authority             `json:"authority"`
	Phase            FencePhase            `json:"phase"`
	Epoch            uint64                `json:"epoch"`
	LeaseUntilUnixMS int64                 `json:"lease_until_unix_ms"`
	AppliedAtUnixMS  int64                 `json:"applied_at_unix_ms"`
}

type RedisAuthorityFenceWriter struct {
	client        redis.Cmdable
	key           string
	receiptPrefix string
	receiptTTL    time.Duration
	now           func() time.Time
}

var fenceTransitionIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:@-]{0,63}$`)

const applyFenceTransitionScript = `
local existing_receipt = redis.call('GET', KEYS[2])
if existing_receipt then
  return {2, existing_receipt}
end
local current = redis.call('GET', KEYS[1])
if ARGV[1] == 'absent' then
  if current then return {0, current} end
elseif not current or current ~= ARGV[2] then
  return {0, current or ''}
end
redis.call('SET', KEYS[1], ARGV[3])
redis.call('PEXPIREAT', KEYS[1], ARGV[4])
redis.call('SET', KEYS[2], ARGV[5], 'PX', ARGV[6])
return {1, ARGV[5]}
`

func NewRedisAuthorityFenceWriter(client redis.Cmdable, key, receiptPrefix string, receiptTTL time.Duration, now func() time.Time) (*RedisAuthorityFenceWriter, error) {
	key = strings.TrimSpace(key)
	receiptPrefix = strings.TrimSpace(receiptPrefix)
	if client == nil || key == "" || receiptPrefix == "" || key == receiptPrefix {
		return nil, fmt.Errorf("delivery authority fence writer configuration is invalid")
	}
	if receiptTTL < time.Hour || receiptTTL > 30*24*time.Hour || now == nil {
		return nil, fmt.Errorf("delivery authority fence receipt retention or clock is invalid")
	}
	return &RedisAuthorityFenceWriter{client: client, key: key, receiptPrefix: receiptPrefix, receiptTTL: receiptTTL, now: now}, nil
}

func (w *RedisAuthorityFenceWriter) Apply(ctx context.Context, request FenceTransitionRequest) (FenceTransitionReceipt, error) {
	now := w.now().UTC()
	request, requestHash, err := validateFenceTransitionRequest(request)
	if err != nil {
		return FenceTransitionReceipt{}, err
	}
	receiptKey := w.receiptPrefix + request.TransitionID
	if existing, err := w.client.Get(ctx, receiptKey).Bytes(); err == nil {
		return decodeMatchingFenceReceipt(existing, requestHash)
	} else if err != redis.Nil {
		return FenceTransitionReceipt{}, fmt.Errorf("read delivery authority fence receipt: %w", err)
	}
	duration := request.LeaseUntil.Sub(now)
	if duration < 5*time.Second || duration > time.Hour {
		return FenceTransitionReceipt{}, fmt.Errorf("delivery authority transition lease must be between 5 seconds and 1 hour")
	}

	var previousRaw []byte
	var previous FenceRecord
	if request.Action != FenceTransitionBootstrap {
		previousRaw, err = w.client.Get(ctx, w.key).Bytes()
		if err != nil {
			return FenceTransitionReceipt{}, fmt.Errorf("read current delivery authority fence: %w", err)
		}
		if hashBytes(previousRaw) != request.ExpectedSHA256 {
			return FenceTransitionReceipt{}, fmt.Errorf("delivery authority fence expected hash is stale")
		}
		previous, err = decodeFenceRecord(previousRaw)
		if err != nil {
			return FenceTransitionReceipt{}, err
		}
	} else if _, err := w.client.Get(ctx, w.key).Result(); err != redis.Nil {
		if err == nil {
			return FenceTransitionReceipt{}, fmt.Errorf("delivery authority fence already exists")
		}
		return FenceTransitionReceipt{}, fmt.Errorf("check delivery authority fence bootstrap: %w", err)
	}

	next, err := nextFenceRecord(previous, request)
	if err != nil {
		return FenceTransitionReceipt{}, err
	}
	nextRaw, err := json.Marshal(next)
	if err != nil {
		return FenceTransitionReceipt{}, fmt.Errorf("encode next delivery authority fence: %w", err)
	}
	receipt := FenceTransitionReceipt{
		SchemaVersion: FenceTransitionReceiptSchemaV1, TransitionID: request.TransitionID,
		RequestSHA256: requestHash, Action: request.Action, OperatorID: request.OperatorID,
		ReasonSHA256: hashBytes([]byte(request.Reason)), PreviousSHA256: hashBytes(previousRaw),
		NextSHA256: hashBytes(nextRaw), Authority: next.Authority, Phase: next.Phase, Epoch: next.Epoch,
		LeaseUntilUnixMS: next.LeaseUntilUnixMS, AppliedAtUnixMS: now.UnixMilli(),
	}
	if request.Action == FenceTransitionBootstrap {
		receipt.PreviousSHA256 = ""
	}
	receiptRaw, err := json.Marshal(receipt)
	if err != nil {
		return FenceTransitionReceipt{}, fmt.Errorf("encode delivery authority fence receipt: %w", err)
	}
	mode := "match"
	if request.Action == FenceTransitionBootstrap {
		mode = "absent"
	}
	result, err := w.client.Eval(ctx, applyFenceTransitionScript,
		[]string{w.key, receiptKey}, mode, string(previousRaw), string(nextRaw),
		next.LeaseUntilUnixMS, string(receiptRaw), w.receiptTTL.Milliseconds()).Result()
	if err != nil {
		return FenceTransitionReceipt{}, fmt.Errorf("apply delivery authority fence transition: %w", err)
	}
	values, ok := result.([]any)
	if !ok || len(values) != 2 {
		return FenceTransitionReceipt{}, fmt.Errorf("delivery authority fence transition returned invalid result")
	}
	code, ok := values[0].(int64)
	if !ok {
		return FenceTransitionReceipt{}, fmt.Errorf("delivery authority fence transition returned invalid status")
	}
	returned, ok := values[1].(string)
	if !ok {
		return FenceTransitionReceipt{}, fmt.Errorf("delivery authority fence transition returned invalid receipt")
	}
	switch code {
	case 1:
		return receipt, nil
	case 2:
		return decodeMatchingFenceReceipt([]byte(returned), requestHash)
	default:
		return FenceTransitionReceipt{}, fmt.Errorf("delivery authority fence compare-and-set conflict")
	}
}

func (w *RedisAuthorityFenceWriter) GetReceipt(ctx context.Context, transitionID string) (FenceTransitionReceipt, error) {
	transitionID = strings.TrimSpace(transitionID)
	if !fenceTransitionIDPattern.MatchString(transitionID) {
		return FenceTransitionReceipt{}, fmt.Errorf("delivery authority transition ID is invalid")
	}
	payload, err := w.client.Get(ctx, w.receiptPrefix+transitionID).Bytes()
	if err != nil {
		if err == redis.Nil {
			return FenceTransitionReceipt{}, fmt.Errorf("delivery authority fence receipt is missing")
		}
		return FenceTransitionReceipt{}, fmt.Errorf("read delivery authority fence receipt: %w", err)
	}
	receipt, err := decodeFenceReceipt(payload)
	if err != nil {
		return FenceTransitionReceipt{}, err
	}
	if receipt.TransitionID != transitionID {
		return FenceTransitionReceipt{}, fmt.Errorf("delivery authority fence receipt transition ID does not match key")
	}
	return receipt, nil
}

func validateFenceTransitionRequest(request FenceTransitionRequest) (FenceTransitionRequest, string, error) {
	request.TransitionID = strings.TrimSpace(request.TransitionID)
	request.OperatorID = strings.TrimSpace(request.OperatorID)
	request.Reason = strings.TrimSpace(request.Reason)
	request.ExpectedSHA256 = strings.ToLower(strings.TrimSpace(request.ExpectedSHA256))
	if !fenceTransitionIDPattern.MatchString(request.TransitionID) || !fenceTransitionIDPattern.MatchString(request.OperatorID) {
		return request, "", fmt.Errorf("delivery authority transition and operator IDs are invalid")
	}
	if request.Reason == "" || len(request.Reason) > 256 {
		return request, "", fmt.Errorf("delivery authority transition reason is invalid")
	}
	switch request.Action {
	case FenceTransitionBootstrap:
		if request.ExpectedSHA256 != "" || request.TargetAuthority != AuthorityGo {
			return request, "", fmt.Errorf("delivery authority bootstrap requires absent state and Go authority")
		}
	case FenceTransitionFreeze, FenceTransitionRenew:
		if !validSHA256(request.ExpectedSHA256) || request.TargetAuthority != "" {
			return request, "", fmt.Errorf("delivery authority freeze/renew request is invalid")
		}
	case FenceTransitionActivate:
		if !validSHA256(request.ExpectedSHA256) {
			return request, "", fmt.Errorf("delivery authority activate expected hash is invalid")
		}
		if _, err := ParseAuthority(string(request.TargetAuthority)); err != nil {
			return request, "", err
		}
	default:
		return request, "", fmt.Errorf("delivery authority transition action %q is unsupported", request.Action)
	}
	canonical := struct {
		SchemaVersion    string                `json:"schema_version"`
		TransitionID     string                `json:"transition_id"`
		Action           FenceTransitionAction `json:"action"`
		OperatorID       string                `json:"operator_id"`
		Reason           string                `json:"reason"`
		ExpectedSHA256   string                `json:"expected_sha256"`
		TargetAuthority  Authority             `json:"target_authority"`
		LeaseUntilUnixMS int64                 `json:"lease_until_unix_ms"`
	}{FenceTransitionSchemaV1, request.TransitionID, request.Action, request.OperatorID, request.Reason,
		request.ExpectedSHA256, request.TargetAuthority, request.LeaseUntil.UTC().UnixMilli()}
	payload, err := json.Marshal(canonical)
	if err != nil {
		return request, "", err
	}
	request.LeaseUntil = request.LeaseUntil.UTC()
	return request, hashBytes(payload), nil
}

func nextFenceRecord(previous FenceRecord, request FenceTransitionRequest) (FenceRecord, error) {
	leaseUntil := request.LeaseUntil.UnixMilli()
	switch request.Action {
	case FenceTransitionBootstrap:
		return FenceRecord{SchemaVersion: FenceSchemaV1, Epoch: 1, Authority: AuthorityGo, Phase: FencePhaseActive, LeaseUntilUnixMS: leaseUntil}, nil
	case FenceTransitionFreeze:
		if previous.Phase != FencePhaseActive || previous.Epoch == ^uint64(0) {
			return FenceRecord{}, fmt.Errorf("only active non-terminal epoch can be frozen")
		}
		previous.Epoch++
		previous.Phase = FencePhaseFrozen
		previous.LeaseUntilUnixMS = leaseUntil
		return previous, nil
	case FenceTransitionActivate:
		if previous.Phase != FencePhaseFrozen {
			return FenceRecord{}, fmt.Errorf("only frozen delivery authority can be activated")
		}
		previous.Authority = request.TargetAuthority
		previous.Phase = FencePhaseActive
		previous.LeaseUntilUnixMS = leaseUntil
		return previous, nil
	case FenceTransitionRenew:
		if request.LeaseUntil.UnixMilli() <= previous.LeaseUntilUnixMS {
			return FenceRecord{}, fmt.Errorf("delivery authority renewal must extend the existing lease")
		}
		previous.LeaseUntilUnixMS = leaseUntil
		return previous, nil
	default:
		return FenceRecord{}, fmt.Errorf("delivery authority transition action %q is unsupported", request.Action)
	}
}

func decodeMatchingFenceReceipt(payload []byte, requestHash string) (FenceTransitionReceipt, error) {
	receipt, err := decodeFenceReceipt(payload)
	if err != nil {
		return FenceTransitionReceipt{}, err
	}
	if receipt.RequestSHA256 != requestHash {
		return FenceTransitionReceipt{}, fmt.Errorf("delivery authority fence transition ID conflicts with another request")
	}
	return receipt, nil
}

func decodeFenceReceipt(payload []byte) (FenceTransitionReceipt, error) {
	receipt, err := DecodeStrictJSON[FenceTransitionReceipt](payload)
	if err != nil {
		return FenceTransitionReceipt{}, fmt.Errorf("decode delivery authority fence receipt: %w", err)
	}
	if receipt.SchemaVersion != FenceTransitionReceiptSchemaV1 {
		return FenceTransitionReceipt{}, fmt.Errorf("delivery authority fence receipt schema is invalid")
	}
	if !fenceTransitionIDPattern.MatchString(receipt.TransitionID) ||
		!fenceTransitionIDPattern.MatchString(receipt.OperatorID) || !validSHA256(receipt.RequestSHA256) ||
		!validSHA256(receipt.ReasonSHA256) || !validSHA256(receipt.NextSHA256) ||
		(receipt.PreviousSHA256 != "" && !validSHA256(receipt.PreviousSHA256)) || receipt.Epoch == 0 ||
		receipt.LeaseUntilUnixMS <= 0 || receipt.AppliedAtUnixMS <= 0 {
		return FenceTransitionReceipt{}, fmt.Errorf("delivery authority fence receipt is invalid")
	}
	if _, err := ParseAuthority(string(receipt.Authority)); err != nil {
		return FenceTransitionReceipt{}, err
	}
	if receipt.Phase != FencePhaseActive && receipt.Phase != FencePhaseFrozen {
		return FenceTransitionReceipt{}, fmt.Errorf("delivery authority fence receipt phase is invalid")
	}
	switch receipt.Action {
	case FenceTransitionBootstrap, FenceTransitionFreeze, FenceTransitionActivate, FenceTransitionRenew:
	default:
		return FenceTransitionReceipt{}, fmt.Errorf("delivery authority fence receipt action is invalid")
	}
	return receipt, nil
}

func hashBytes(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
