package app

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/JekYUlll/Dipole/internal/application"
)

type PersistentAgentMemoryOwnerControlV1 struct {
	store application.AgentMemoryOwnerStoreV1
	now   func() time.Time
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
