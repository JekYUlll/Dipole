package agentapplication

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/JekYUlll/Dipole/internal/application"
)

type PersistentAgentMemoryOwnerControlV1 struct {
	store application.AgentMemoryOwnerStoreV1
	now   func() time.Time
}

func (s *PersistentAgentMemoryOwnerControlV1) EraseOwnedMemory(ctx context.Context, request application.AgentMemoryOwnerErasureRequestV1) (*application.AgentMemoryOwnerErasureReceiptV1, error) {
	request.TenantID, request.PrincipalUUID, request.MemoryUUID = strings.TrimSpace(request.TenantID), strings.TrimSpace(request.PrincipalUUID), strings.TrimSpace(request.MemoryUUID)
	if request.TenantID == "" || request.PrincipalUUID == "" || request.MemoryUUID == "" || utf8.RuneCountInString(request.TenantID) > 64 || utf8.RuneCountInString(request.PrincipalUUID) > 64 || utf8.RuneCountInString(request.MemoryUUID) > 64 {
		return nil, application.ErrAgentMemoryInvalid
	}
	item, err := s.store.GetOwnedMemory(ctx, request.TenantID, request.PrincipalUUID, request.MemoryUUID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, application.ErrAgentMemoryDenied
	}
	erasedAt := s.now().UTC()
	if erasedAt.IsZero() {
		return nil, application.ErrAgentMemoryInvalid
	}
	receipt, err := s.store.EraseOwnedMemoryRoot(ctx, request.TenantID, request.PrincipalUUID, request.MemoryUUID, request.PrincipalUUID, application.AgentMemoryErasureReasonOwnerRequest, erasedAt)
	if err != nil {
		return nil, err
	}
	if receipt == nil || receipt.MemoryRootUUID != item.MemoryRootUUID || receipt.ErasedByUUID != request.PrincipalUUID || receipt.Reason != application.AgentMemoryErasureReasonOwnerRequest || receipt.Versions < 1 || receipt.ErasedAt.IsZero() {
		return nil, application.ErrAgentMemoryConflict
	}
	return receipt, nil
}

func (s *PersistentAgentMemoryOwnerControlV1) CorrectOwnedMemory(ctx context.Context, request application.AgentMemoryOwnerCorrectionRequestV1) (*application.AgentMemoryOwnerCorrectionResultV1, error) {
	request.TenantID, request.PrincipalUUID, request.MemoryUUID = strings.TrimSpace(request.TenantID), strings.TrimSpace(request.PrincipalUUID), strings.TrimSpace(request.MemoryUUID)
	request.Reason = strings.TrimSpace(request.Reason)
	if request.TenantID == "" || request.PrincipalUUID == "" || request.MemoryUUID == "" || strings.TrimSpace(request.Content) == "" || request.Reason == "" || request.ExpectedVersion == 0 ||
		utf8.RuneCountInString(request.TenantID) > 64 || utf8.RuneCountInString(request.PrincipalUUID) > 64 || utf8.RuneCountInString(request.MemoryUUID) > 64 ||
		len(request.Content) > application.AgentMemoryContentMaxBytesV1 || len(request.CompactContent) > application.AgentMemoryCompactContentMaxBytesV1 ||
		utf8.RuneCountInString(request.Reason) > application.AgentMemoryCorrectionReasonMaxRunesV1 || hasAgentMemoryControlRuneV1(request.Reason) {
		return nil, application.ErrAgentMemoryInvalid
	}
	source, err := s.store.GetOwnedMemory(ctx, request.TenantID, request.PrincipalUUID, request.MemoryUUID)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, application.ErrAgentMemoryDenied
	}
	canonicalSource := application.CanonicalAgentMemoryLineageV1(*source)
	if canonicalSource.Validate() != nil || canonicalSource.TenantID != request.TenantID || canonicalSource.PrincipalUUID != request.PrincipalUUID ||
		canonicalSource.MemoryVersion != request.ExpectedVersion {
		return nil, application.ErrAgentMemoryConflict
	}
	correctedAt := s.now().UTC()
	if correctedAt.IsZero() || (canonicalSource.Status == application.AgentMemoryStatusActive && canonicalSource.ExpiresAt != nil && !canonicalSource.ExpiresAt.After(correctedAt)) {
		return nil, application.ErrAgentMemoryConflict
	}
	nextVersion := canonicalSource.MemoryVersion + 1
	if nextVersion <= canonicalSource.MemoryVersion {
		return nil, application.ErrAgentMemoryConflict
	}
	corrected := application.AgentMemoryV1{
		MemoryUUID: stableAgentMemoryCorrectionUUIDV1(request), TenantID: canonicalSource.TenantID, PrincipalUUID: canonicalSource.PrincipalUUID,
		AgentUUID: canonicalSource.AgentUUID, MemoryType: canonicalSource.MemoryType, Status: application.AgentMemoryStatusActive,
		ResourceType: canonicalSource.ResourceType, ResourceID: canonicalSource.ResourceID, Content: request.Content, CompactContent: request.CompactContent,
		Priority: canonicalSource.Priority, Provenance: application.AgentMemoryProvenanceV1{
			SourceType: application.AgentMemorySourceOwnerCorrectionV1, SourceID: canonicalSource.MemoryUUID, Sequence: strconv.FormatUint(uint64(nextVersion), 10),
		},
		ValidFrom: correctedAt, ExpiresAt: canonicalSource.ExpiresAt, MemoryRootUUID: canonicalSource.MemoryRootUUID, MemoryVersion: nextVersion,
		SupersedesMemoryUUID: canonicalSource.MemoryUUID, CorrectedByUUID: request.PrincipalUUID, CorrectionReason: request.Reason,
	}
	if corrected.Validate() != nil {
		return nil, application.ErrAgentMemoryInvalid
	}
	result, err := s.store.CorrectOwnedMemory(ctx, application.AgentMemoryOwnerCorrectionWriteV1{
		TenantID: request.TenantID, PrincipalUUID: request.PrincipalUUID, SourceMemoryUUID: request.MemoryUUID,
		ExpectedVersion: request.ExpectedVersion, Corrected: corrected, CorrectedAt: correctedAt,
	})
	if err != nil {
		return nil, err
	}
	if !exactAgentMemoryOwnerCorrectionV1(result, request, corrected) {
		return nil, application.ErrAgentMemoryConflict
	}
	return result, nil
}

func stableAgentMemoryCorrectionUUIDV1(request application.AgentMemoryOwnerCorrectionRequestV1) string {
	hash := sha256.New()
	for _, value := range []string{request.TenantID, request.PrincipalUUID, request.MemoryUUID, strconv.FormatUint(uint64(request.ExpectedVersion), 10), request.Content, request.CompactContent, request.Reason} {
		_, _ = fmt.Fprintf(hash, "%d:%s|", len(value), value)
	}
	return fmt.Sprintf("MEM-CORR-%x", hash.Sum(nil)[:16])
}

func exactAgentMemoryOwnerCorrectionV1(result *application.AgentMemoryOwnerCorrectionResultV1, request application.AgentMemoryOwnerCorrectionRequestV1, expected application.AgentMemoryV1) bool {
	if result == nil {
		return false
	}
	previous, corrected := result.Previous, result.Corrected
	return previous.Validate() == nil && corrected.Validate() == nil && previous.TenantID == request.TenantID && previous.PrincipalUUID == request.PrincipalUUID &&
		previous.MemoryUUID == request.MemoryUUID && previous.MemoryVersion == request.ExpectedVersion && previous.Status == application.AgentMemoryStatusRevoked &&
		previous.RevokedByUUID == request.PrincipalUUID && previous.RevokeReason == "superseded by "+expected.MemoryUUID &&
		corrected.MemoryUUID == expected.MemoryUUID && corrected.TenantID == expected.TenantID && corrected.PrincipalUUID == expected.PrincipalUUID &&
		corrected.AgentUUID == expected.AgentUUID && corrected.MemoryType == expected.MemoryType && corrected.Status == application.AgentMemoryStatusActive &&
		corrected.ResourceType == expected.ResourceType && corrected.ResourceID == expected.ResourceID && corrected.Content == request.Content &&
		corrected.CompactContent == request.CompactContent && corrected.Priority == expected.Priority && corrected.MemoryRootUUID == expected.MemoryRootUUID &&
		corrected.MemoryVersion == expected.MemoryVersion && corrected.SupersedesMemoryUUID == expected.SupersedesMemoryUUID &&
		corrected.CorrectedByUUID == request.PrincipalUUID && corrected.CorrectionReason == request.Reason &&
		corrected.Provenance.SourceType == application.AgentMemorySourceOwnerCorrectionV1 && corrected.Provenance.SourceID == request.MemoryUUID &&
		corrected.Provenance.Sequence == expected.Provenance.Sequence
}

func hasAgentMemoryControlRuneV1(value string) bool {
	return strings.ContainsFunc(value, unicode.IsControl)
}

func NewPersistentAgentMemoryOwnerControlV1(store application.AgentMemoryOwnerStoreV1, now func() time.Time) (*PersistentAgentMemoryOwnerControlV1, error) {
	if store == nil {
		return nil, errors.New("Agent Memory owner store is required")
	}
	if now == nil {
		now = time.Now
	}
	return &PersistentAgentMemoryOwnerControlV1{store: store, now: now}, nil
}

func (s *PersistentAgentMemoryOwnerControlV1) ListOwnedMemories(ctx context.Context, request application.AgentMemoryOwnerListRequestV1) (*application.AgentMemoryOwnerPageV1, error) {
	request.TenantID, request.PrincipalUUID, request.AfterUUID = strings.TrimSpace(request.TenantID), strings.TrimSpace(request.PrincipalUUID), strings.TrimSpace(request.AfterUUID)
	if request.TenantID == "" || request.PrincipalUUID == "" || utf8.RuneCountInString(request.TenantID) > 64 ||
		utf8.RuneCountInString(request.PrincipalUUID) > 64 || utf8.RuneCountInString(request.AfterUUID) > 64 ||
		request.Limit < 0 || request.Limit > 100 || (request.AfterCreatedAt.IsZero() != (request.AfterUUID == "")) {
		return nil, application.ErrAgentMemoryInvalid
	}
	if request.Limit == 0 {
		request.Limit = 50
	}
	if request.AfterCreatedAt.IsZero() {
		request.AfterCreatedAt = s.now().UTC()
	} else {
		request.AfterCreatedAt = request.AfterCreatedAt.UTC()
	}
	storeRequest := request
	storeRequest.Limit++
	items, err := s.store.ListOwnedMemories(ctx, storeRequest)
	if err != nil {
		return nil, err
	}
	for index, item := range items {
		if item.Validate() != nil || item.TenantID != request.TenantID || item.PrincipalUUID != request.PrincipalUUID || item.CreatedAt.IsZero() {
			return nil, application.ErrAgentMemoryConflict
		}
		if index > 0 && !agentMemoryOwnerOrderV1(items[index-1], item) {
			return nil, application.ErrAgentMemoryConflict
		}
	}
	page := &application.AgentMemoryOwnerPageV1{Memories: items}
	if len(page.Memories) > request.Limit {
		page.Memories = page.Memories[:request.Limit]
		last := page.Memories[len(page.Memories)-1]
		page.NextCreatedAt, page.NextMemoryUUID = last.CreatedAt.UTC(), last.MemoryUUID
	}
	return page, nil
}

func (s *PersistentAgentMemoryOwnerControlV1) RevokeOwnedMemory(ctx context.Context, request application.AgentMemoryOwnerRevokeRequestV1) (*application.AgentMemoryV1, error) {
	request.TenantID, request.PrincipalUUID = strings.TrimSpace(request.TenantID), strings.TrimSpace(request.PrincipalUUID)
	request.MemoryUUID, request.Reason = strings.TrimSpace(request.MemoryUUID), strings.TrimSpace(request.Reason)
	if request.TenantID == "" || request.PrincipalUUID == "" || request.MemoryUUID == "" || request.Reason == "" ||
		utf8.RuneCountInString(request.TenantID) > 64 || utf8.RuneCountInString(request.PrincipalUUID) > 64 ||
		utf8.RuneCountInString(request.MemoryUUID) > 64 || utf8.RuneCountInString(request.Reason) > application.AgentMemoryRevokeReasonMaxRunesV1 {
		return nil, application.ErrAgentMemoryInvalid
	}
	item, err := s.store.GetOwnedMemory(ctx, request.TenantID, request.PrincipalUUID, request.MemoryUUID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, application.ErrAgentMemoryDenied
	}
	if item.Status == application.AgentMemoryStatusRevoked {
		if exactAgentMemoryOwnerRevocationV1(item, request) {
			return item, nil
		}
		return nil, application.ErrAgentMemoryConflict
	}
	if item.Validate() != nil || item.TenantID != request.TenantID || item.PrincipalUUID != request.PrincipalUUID {
		return nil, application.ErrAgentMemoryConflict
	}
	revokedAt := s.now().UTC()
	if revokedAt.IsZero() {
		return nil, application.ErrAgentMemoryInvalid
	}
	if err = s.store.RevokeOwnedMemory(ctx, request.TenantID, request.PrincipalUUID, request.MemoryUUID, request.PrincipalUUID, request.Reason, revokedAt); err != nil {
		if !errors.Is(err, application.ErrAgentMemoryConflict) {
			return nil, err
		}
	}
	stored, err := s.store.GetOwnedMemory(ctx, request.TenantID, request.PrincipalUUID, request.MemoryUUID)
	if err != nil {
		return nil, err
	}
	if !exactAgentMemoryOwnerRevocationV1(stored, request) {
		return nil, application.ErrAgentMemoryConflict
	}
	return stored, nil
}

func exactAgentMemoryOwnerRevocationV1(item *application.AgentMemoryV1, request application.AgentMemoryOwnerRevokeRequestV1) bool {
	return item != nil && item.Validate() == nil && item.TenantID == request.TenantID && item.PrincipalUUID == request.PrincipalUUID &&
		item.MemoryUUID == request.MemoryUUID && item.Status == application.AgentMemoryStatusRevoked &&
		item.RevokedByUUID == request.PrincipalUUID && item.RevokeReason == request.Reason
}

func agentMemoryOwnerOrderV1(previous, current application.AgentMemoryV1) bool {
	if previous.CreatedAt.Equal(current.CreatedAt) {
		return previous.MemoryUUID < current.MemoryUUID
	}
	return previous.CreatedAt.After(current.CreatedAt)
}
