package gateway

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	agentv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/agent/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrAgentMemoryInvalid     = errors.New("Agent Memory request is invalid")
	ErrAgentMemoryDenied      = errors.New("Agent Memory access denied")
	ErrAgentMemoryConflict    = errors.New("Agent Memory changed concurrently")
	ErrAgentMemoryUnavailable = errors.New("Agent Memory control is unavailable")
)

type AgentMemoryProvenance struct {
	SourceType string `json:"sourceType"`
	SourceID   string `json:"sourceId"`
	Sequence   string `json:"sequence,omitempty"`
}

type AgentMemory struct {
	MemoryID         string                `json:"memoryId"`
	AgentID          string                `json:"agentId"`
	MemoryType       string                `json:"memoryType"`
	Status           string                `json:"status"`
	ResourceType     string                `json:"resourceType"`
	ResourceID       string                `json:"resourceId"`
	Content          string                `json:"content"`
	CompactContent   string                `json:"compactContent,omitempty"`
	Priority         int32                 `json:"priority"`
	Provenance       AgentMemoryProvenance `json:"provenance"`
	ValidFromUnixMS  int64                 `json:"validFromUnixMs"`
	ExpiresAtUnixMS  int64                 `json:"expiresAtUnixMs,omitempty"`
	RevokedAtUnixMS  int64                 `json:"revokedAtUnixMs,omitempty"`
	RevokedByID      string                `json:"revokedById,omitempty"`
	RevokeReason     string                `json:"revokeReason,omitempty"`
	CreatedAtUnixMS  int64                 `json:"createdAtUnixMs"`
	MemoryRootID     string                `json:"memoryRootId"`
	MemoryVersion    uint32                `json:"memoryVersion"`
	SupersedesID     string                `json:"supersedesMemoryId,omitempty"`
	CorrectedByID    string                `json:"correctedById,omitempty"`
	CorrectionReason string                `json:"correctionReason,omitempty"`
}

type AgentMemoryCorrection struct {
	Previous  AgentMemory `json:"previous"`
	Corrected AgentMemory `json:"corrected"`
}

type AgentMemoryPage struct {
	Memories   []AgentMemory `json:"memories"`
	NextCursor string        `json:"nextCursor,omitempty"`
}

type AgentMemoryControlApplication interface {
	List(ctx context.Context, principalUUID, after string, limit int) (*AgentMemoryPage, error)
	Revoke(ctx context.Context, principalUUID, memoryID, reason string) (*AgentMemory, error)
	Correct(ctx context.Context, principalUUID, memoryID string, expectedVersion uint32, content, compactContent, reason string) (*AgentMemoryCorrection, error)
}

type agentMemoryRPC interface {
	ListOwnedMemories(context.Context, *agentv1.ListOwnedMemoriesRequest, ...grpc.CallOption) (*agentv1.ListOwnedMemoriesResponse, error)
	RevokeOwnedMemory(context.Context, *agentv1.RevokeOwnedMemoryRequest, ...grpc.CallOption) (*agentv1.AgentOwnedMemory, error)
	CorrectOwnedMemory(context.Context, *agentv1.CorrectOwnedMemoryRequest, ...grpc.CallOption) (*agentv1.CorrectOwnedMemoryResponse, error)
}

type AgentMemoryControlClient struct {
	rpc      agentMemoryRPC
	tenantID string
	timeout  time.Duration
}

func NewAgentMemoryControlClient(rpc agentMemoryRPC, tenantID string, timeout time.Duration) (*AgentMemoryControlClient, error) {
	tenantID = strings.TrimSpace(tenantID)
	if rpc == nil || tenantID == "" || len([]rune(tenantID)) > 64 {
		return nil, ErrAgentMemoryInvalid
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &AgentMemoryControlClient{rpc: rpc, tenantID: tenantID, timeout: timeout}, nil
}

func (c *AgentMemoryControlClient) List(ctx context.Context, principalUUID, after string, limit int) (*AgentMemoryPage, error) {
	afterCreatedAt, afterMemoryID, err := decodeAgentMemoryCursor(after)
	if err != nil || strings.TrimSpace(principalUUID) == "" || limit < 1 || limit > 100 {
		return nil, ErrAgentMemoryInvalid
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	request := &agentv1.ListOwnedMemoriesRequest{
		Context: grpccommon.RequestContextFrom(ctx, principalUUID, "dipole-gateway"), TenantId: c.tenantID,
		AfterMemoryId: afterMemoryID, Limit: uint32(limit),
	}
	if !afterCreatedAt.IsZero() {
		request.AfterCreatedAtUnixMs = afterCreatedAt.UnixMilli()
	}
	response, err := c.rpc.ListOwnedMemories(callCtx, request)
	if err != nil {
		return nil, mapAgentMemoryRPCError(err)
	}
	if response == nil || (response.GetNextCreatedAtUnixMs() == 0) != (response.GetNextMemoryId() == "") {
		return nil, ErrAgentMemoryUnavailable
	}
	page := &AgentMemoryPage{Memories: make([]AgentMemory, 0, len(response.GetMemories()))}
	for _, raw := range response.GetMemories() {
		item, mapErr := agentMemoryFromProto(raw)
		if mapErr != nil {
			return nil, ErrAgentMemoryUnavailable
		}
		page.Memories = append(page.Memories, item)
	}
	if response.GetNextMemoryId() != "" {
		page.NextCursor, err = encodeAgentMemoryCursor(time.UnixMilli(response.GetNextCreatedAtUnixMs()).UTC(), response.GetNextMemoryId())
		if err != nil {
			return nil, ErrAgentMemoryUnavailable
		}
	}
	return page, nil
}

func (c *AgentMemoryControlClient) Revoke(ctx context.Context, principalUUID, memoryID, reason string) (*AgentMemory, error) {
	principalUUID, memoryID, reason = strings.TrimSpace(principalUUID), strings.TrimSpace(memoryID), strings.TrimSpace(reason)
	if principalUUID == "" || !validAgentSubscriptionPublicID(memoryID, 64) || reason == "" || len([]rune(reason)) > 1000 {
		return nil, ErrAgentMemoryInvalid
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.rpc.RevokeOwnedMemory(callCtx, &agentv1.RevokeOwnedMemoryRequest{
		Context: grpccommon.RequestContextFrom(ctx, principalUUID, "dipole-gateway"), TenantId: c.tenantID, MemoryId: memoryID, Reason: reason,
	})
	if err != nil {
		return nil, mapAgentMemoryRPCError(err)
	}
	item, err := agentMemoryFromProto(response)
	if err != nil || item.MemoryID != memoryID || item.Status != "revoked" || item.RevokedByID != principalUUID || item.RevokeReason != reason {
		return nil, ErrAgentMemoryUnavailable
	}
	return &item, nil
}

func (c *AgentMemoryControlClient) Correct(ctx context.Context, principalUUID, memoryID string, expectedVersion uint32, content, compactContent, reason string) (*AgentMemoryCorrection, error) {
	principalUUID, memoryID = strings.TrimSpace(principalUUID), strings.TrimSpace(memoryID)
	content, compactContent, reason = strings.TrimSpace(content), strings.TrimSpace(compactContent), strings.TrimSpace(reason)
	if principalUUID == "" || !validAgentSubscriptionPublicID(memoryID, 64) || expectedVersion == 0 ||
		content == "" || len([]byte(content)) > 16*1024 || len([]byte(compactContent)) > 4*1024 ||
		reason == "" || utf8.RuneCountInString(reason) > 1000 || strings.IndexFunc(reason, unicode.IsControl) >= 0 {
		return nil, ErrAgentMemoryInvalid
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.rpc.CorrectOwnedMemory(callCtx, &agentv1.CorrectOwnedMemoryRequest{
		Context: grpccommon.RequestContextFrom(ctx, principalUUID, "dipole-gateway"), TenantId: c.tenantID,
		MemoryId: memoryID, ExpectedVersion: expectedVersion, Content: content, CompactContent: compactContent, Reason: reason,
	})
	if err != nil {
		return nil, mapAgentMemoryRPCError(err)
	}
	if response == nil {
		return nil, ErrAgentMemoryUnavailable
	}
	previous, previousErr := agentMemoryFromProto(response.GetPrevious())
	corrected, correctedErr := agentMemoryFromProto(response.GetCorrected())
	if previousErr != nil || correctedErr != nil || previous.MemoryID != memoryID || previous.MemoryVersion != expectedVersion ||
		previous.Status != "revoked" || previous.RevokedByID != principalUUID || corrected.Status != "active" ||
		corrected.MemoryRootID != previous.MemoryRootID || corrected.MemoryVersion != expectedVersion+1 ||
		corrected.SupersedesID != memoryID || corrected.CorrectedByID != principalUUID || corrected.CorrectionReason != reason ||
		corrected.Content != content || corrected.CompactContent != compactContent {
		return nil, ErrAgentMemoryUnavailable
	}
	return &AgentMemoryCorrection{Previous: previous, Corrected: corrected}, nil
}

func agentMemoryFromProto(raw *agentv1.AgentOwnedMemory) (AgentMemory, error) {
	if raw == nil || !validAgentSubscriptionPublicID(raw.GetMemoryId(), 64) || !validAgentSubscriptionPublicID(raw.GetAgentId(), 24) ||
		!validAgentMemoryType(raw.GetMemoryType()) || (raw.GetStatus() != "active" && raw.GetStatus() != "revoked") ||
		!validAgentSubscriptionPublicID(raw.GetResourceType(), 64) || !validAgentSubscriptionPublicID(raw.GetResourceId(), 128) ||
		strings.TrimSpace(raw.GetContent()) == "" || len(raw.GetContent()) > 16*1024 || len(raw.GetCompactContent()) > 4*1024 ||
		raw.GetPriority() < 0 || raw.GetPriority() > 1000 || raw.GetValidFromUnixMs() <= 0 || raw.GetCreatedAtUnixMs() <= 0 ||
		raw.GetProvenance() == nil || !validAgentSubscriptionPublicID(raw.GetProvenance().GetSourceType(), 64) ||
		!validAgentSubscriptionPublicID(raw.GetProvenance().GetSourceId(), 128) || raw.GetProvenance().GetUri() != "" {
		return AgentMemory{}, ErrAgentMemoryUnavailable
	}
	if raw.GetExpiresAtUnixMs() != 0 && raw.GetExpiresAtUnixMs() <= raw.GetValidFromUnixMs() {
		return AgentMemory{}, ErrAgentMemoryUnavailable
	}
	if (raw.GetStatus() == "active" && (raw.GetRevokedAtUnixMs() != 0 || raw.GetRevokedById() != "" || raw.GetRevokeReason() != "")) ||
		(raw.GetStatus() == "revoked" && (raw.GetRevokedAtUnixMs() <= 0 || !validAgentSubscriptionPublicID(raw.GetRevokedById(), 64) || strings.TrimSpace(raw.GetRevokeReason()) == "")) {
		return AgentMemory{}, ErrAgentMemoryUnavailable
	}
	if !validAgentSubscriptionPublicID(raw.GetMemoryRootId(), 64) || raw.GetMemoryVersion() == 0 {
		return AgentMemory{}, ErrAgentMemoryUnavailable
	}
	if raw.GetMemoryVersion() == 1 {
		if raw.GetMemoryRootId() != raw.GetMemoryId() || raw.GetSupersedesMemoryId() != "" || raw.GetCorrectedById() != "" || raw.GetCorrectionReason() != "" {
			return AgentMemory{}, ErrAgentMemoryUnavailable
		}
	} else if !validAgentSubscriptionPublicID(raw.GetSupersedesMemoryId(), 64) ||
		!validAgentSubscriptionPublicID(raw.GetCorrectedById(), 64) || strings.TrimSpace(raw.GetCorrectionReason()) == "" ||
		raw.GetProvenance().GetSourceType() != "owner_correction" || raw.GetProvenance().GetSourceId() != raw.GetSupersedesMemoryId() ||
		raw.GetProvenance().GetSequence() != strconv.FormatUint(uint64(raw.GetMemoryVersion()), 10) {
		return AgentMemory{}, ErrAgentMemoryUnavailable
	}
	return AgentMemory{
		MemoryID: raw.GetMemoryId(), AgentID: raw.GetAgentId(), MemoryType: raw.GetMemoryType(), Status: raw.GetStatus(),
		ResourceType: raw.GetResourceType(), ResourceID: raw.GetResourceId(), Content: raw.GetContent(), CompactContent: raw.GetCompactContent(),
		Priority: raw.GetPriority(), Provenance: AgentMemoryProvenance{
			SourceType: raw.GetProvenance().GetSourceType(), SourceID: raw.GetProvenance().GetSourceId(), Sequence: raw.GetProvenance().GetSequence(),
		}, ValidFromUnixMS: raw.GetValidFromUnixMs(), ExpiresAtUnixMS: raw.GetExpiresAtUnixMs(), RevokedAtUnixMS: raw.GetRevokedAtUnixMs(),
		RevokedByID: raw.GetRevokedById(), RevokeReason: raw.GetRevokeReason(), CreatedAtUnixMS: raw.GetCreatedAtUnixMs(),
		MemoryRootID: raw.GetMemoryRootId(), MemoryVersion: raw.GetMemoryVersion(), SupersedesID: raw.GetSupersedesMemoryId(),
		CorrectedByID: raw.GetCorrectedById(), CorrectionReason: raw.GetCorrectionReason(),
	}, nil
}

func validAgentMemoryType(value string) bool {
	switch value {
	case "working", "episodic", "semantic", "procedural", "observational":
		return true
	default:
		return false
	}
}

func encodeAgentMemoryCursor(createdAt time.Time, memoryID string) (string, error) {
	if createdAt.IsZero() || !validAgentSubscriptionPublicID(memoryID, 64) {
		return "", ErrAgentMemoryInvalid
	}
	encoded, err := json.Marshal(struct {
		CreatedAtUnixMS int64  `json:"createdAtUnixMs"`
		MemoryID        string `json:"memoryId"`
	}{CreatedAtUnixMS: createdAt.UTC().UnixMilli(), MemoryID: memoryID})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func decodeAgentMemoryCursor(cursor string) (time.Time, string, error) {
	if cursor == "" {
		return time.Time{}, "", nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil || len(decoded) > 256 {
		return time.Time{}, "", ErrAgentMemoryInvalid
	}
	var value struct {
		CreatedAtUnixMS int64  `json:"createdAtUnixMs"`
		MemoryID        string `json:"memoryId"`
	}
	if decodeStrictAgentSubscriptionJSON(decoded, &value) != nil || value.CreatedAtUnixMS <= 0 || !validAgentSubscriptionPublicID(value.MemoryID, 64) {
		return time.Time{}, "", ErrAgentMemoryInvalid
	}
	createdAt := time.UnixMilli(value.CreatedAtUnixMS).UTC()
	canonical, err := encodeAgentMemoryCursor(createdAt, value.MemoryID)
	if err != nil || canonical != cursor {
		return time.Time{}, "", ErrAgentMemoryInvalid
	}
	return createdAt, value.MemoryID, nil
}

func mapAgentMemoryRPCError(err error) error {
	switch status.Code(err) {
	case codes.InvalidArgument, codes.FailedPrecondition:
		return ErrAgentMemoryInvalid
	case codes.PermissionDenied, codes.Unauthenticated:
		return ErrAgentMemoryDenied
	case codes.Aborted, codes.AlreadyExists:
		return ErrAgentMemoryConflict
	default:
		return ErrAgentMemoryUnavailable
	}
}
