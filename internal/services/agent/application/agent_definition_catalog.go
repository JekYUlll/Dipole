package agentapplication

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/JekYUlll/Dipole/internal/application"
)

type PersistentAgentDefinitionCatalogV1 struct {
	store     application.AgentDefinitionCatalogStoreV1
	agentUUID string
	now       func() time.Time
}

func NewPersistentAgentDefinitionCatalogV1(store application.AgentDefinitionCatalogStoreV1, agentUUID string, now func() time.Time) (*PersistentAgentDefinitionCatalogV1, error) {
	agentUUID = strings.TrimSpace(agentUUID)
	if store == nil || agentUUID == "" || utf8.RuneCountInString(agentUUID) > 24 {
		return nil, errors.New("Agent Definition catalog store and Agent identity are required")
	}
	if now == nil {
		now = time.Now
	}
	return &PersistentAgentDefinitionCatalogV1{store: store, agentUUID: agentUUID, now: now}, nil
}

func (s *PersistentAgentDefinitionCatalogV1) Create(ctx context.Context, principalUUID string, request application.AgentDefinitionCatalogCreateRequestV1) (*application.AgentDefinitionVersionV1, error) {
	principalUUID = strings.TrimSpace(principalUUID)
	request.TenantID = strings.TrimSpace(request.TenantID)
	if principalUUID == "" || request.TenantID == "" || utf8.RuneCountInString(principalUUID) > 24 || utf8.RuneCountInString(request.TenantID) > 64 {
		return nil, application.ErrAgentDefinitionCatalogInvalid
	}
	definition, err := userDefinitionForProfileV1(request.TenantID, principalUUID, s.agentUUID, strings.TrimSpace(request.Profile), s.now().UTC())
	if err != nil {
		return nil, application.ErrAgentDefinitionCatalogInvalid
	}
	if err := s.store.CreateDefinitionVersion(ctx, definition); err != nil {
		existing, lookupErr := s.store.GetDefinitionVersion(ctx, definition.DefinitionUUID, definition.Version)
		if lookupErr != nil || existing == nil || !sameUserDefinitionV1(*existing, definition) {
			return nil, application.ErrAgentDefinitionCatalogConflict
		}
		return existing, nil
	}
	stored, err := s.store.GetDefinitionVersion(ctx, definition.DefinitionUUID, definition.Version)
	if err != nil || stored == nil || !sameUserDefinitionV1(*stored, definition) {
		return nil, application.ErrAgentDefinitionCatalogConflict
	}
	return stored, nil
}

func userDefinitionForProfileV1(tenantID, ownerUUID, agentUUID, profile string, validFrom time.Time) (application.AgentDefinitionVersionV1, error) {
	switch profile {
	case "", application.AgentDefinitionCatalogProfileReadOnly:
		return userReadDefinitionV1(tenantID, ownerUUID, agentUUID, validFrom), nil
	case application.AgentDefinitionCatalogProfileSubscriptionAutoReply:
		return userSubscriptionAutoReplyDefinitionV1(tenantID, ownerUUID, agentUUID, validFrom), nil
	default:
		return application.AgentDefinitionVersionV1{}, application.ErrAgentDefinitionCatalogInvalid
	}
}

func userReadDefinitionV1(tenantID, ownerUUID, agentUUID string, validFrom time.Time) application.AgentDefinitionVersionV1 {
	digest := sha256.Sum256([]byte("dipole.agent.user-read-definition.v1\n" + tenantID + "\n" + ownerUUID + "\n" + agentUUID))
	return application.AgentDefinitionVersionV1{
		DefinitionUUID: "user:" + hex.EncodeToString(digest[:])[:59], Version: 1, TenantID: tenantID, OwnerUUID: ownerUUID, AgentUUID: agentUUID,
		Status: application.AgentDefinitionStatusActive, Permissions: []string{application.AgentPermissionConversationRead},
		Scopes:    []application.AgentResourceScopeV1{{ResourceType: application.AgentResourceTypeConversation, ResourceID: application.AgentResourceWildcard, Actions: []string{application.AgentResourceActionRead}}},
		ValidFrom: validFrom,
	}
}

func userSubscriptionAutoReplyDefinitionV1(tenantID, ownerUUID, agentUUID string, validFrom time.Time) application.AgentDefinitionVersionV1 {
	digest := sha256.Sum256([]byte("dipole.agent.user-subscription-autoreply-definition.v1\n" + tenantID + "\n" + ownerUUID + "\n" + agentUUID))
	return application.AgentDefinitionVersionV1{
		DefinitionUUID: "user:" + hex.EncodeToString(digest[:])[:59], Version: 1, TenantID: tenantID, OwnerUUID: ownerUUID, AgentUUID: agentUUID,
		Status:      application.AgentDefinitionStatusActive,
		Permissions: []string{application.AgentPermissionConversationRead, application.AgentPermissionMessageWrite},
		Scopes: []application.AgentResourceScopeV1{{
			ResourceType: application.AgentResourceTypeConversation, ResourceID: application.AgentResourceWildcard,
			Actions: []string{application.AgentResourceActionRead, application.AgentResourceActionWrite},
		}},
		ValidFrom: validFrom,
	}
}

func sameUserDefinitionV1(left, right application.AgentDefinitionVersionV1) bool {
	if left.DefinitionUUID != right.DefinitionUUID || left.Version != right.Version || left.TenantID != right.TenantID || left.OwnerUUID != right.OwnerUUID || left.AgentUUID != right.AgentUUID ||
		left.Status != right.Status || left.RevokedAt != nil || right.RevokedAt != nil || len(left.Permissions) != len(right.Permissions) || len(left.Scopes) != len(right.Scopes) {
		return false
	}
	for index := range left.Permissions {
		if left.Permissions[index] != right.Permissions[index] {
			return false
		}
	}
	for index := range left.Scopes {
		if left.Scopes[index].ResourceType != right.Scopes[index].ResourceType || left.Scopes[index].ResourceID != right.Scopes[index].ResourceID || len(left.Scopes[index].Actions) != len(right.Scopes[index].Actions) {
			return false
		}
		for actionIndex := range left.Scopes[index].Actions {
			if left.Scopes[index].Actions[actionIndex] != right.Scopes[index].Actions[actionIndex] {
				return false
			}
		}
	}
	return true
}

func (s *PersistentAgentDefinitionCatalogV1) List(ctx context.Context, principalUUID string, request application.AgentDefinitionCatalogListRequestV1) (*application.AgentDefinitionCatalogPageV1, error) {
	principalUUID = strings.TrimSpace(principalUUID)
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.AfterDefinitionUUID = strings.TrimSpace(request.AfterDefinitionUUID)
	if principalUUID == "" || request.TenantID == "" || utf8.RuneCountInString(principalUUID) > 24 || utf8.RuneCountInString(request.TenantID) > 64 ||
		utf8.RuneCountInString(request.AfterDefinitionUUID) > 64 || request.Limit < 0 || request.Limit > 100 ||
		(request.AfterDefinitionUUID == "") != (request.AfterVersion == 0) {
		return nil, application.ErrAgentDefinitionCatalogInvalid
	}
	if request.Limit == 0 {
		request.Limit = 50
	}
	activeAt := s.now().UTC()
	definitions, err := s.store.ListOwnedActiveDefinitions(ctx, request.TenantID, principalUUID, request.AfterDefinitionUUID, request.AfterVersion, activeAt, request.Limit+1)
	if err != nil {
		return nil, fmt.Errorf("list owned active Agent Definitions: %w", err)
	}
	sort.Slice(definitions, func(left, right int) bool {
		if definitions[left].DefinitionUUID == definitions[right].DefinitionUUID {
			return definitions[left].Version < definitions[right].Version
		}
		return definitions[left].DefinitionUUID < definitions[right].DefinitionUUID
	})
	page := &application.AgentDefinitionCatalogPageV1{Definitions: make([]application.AgentDefinitionCatalogItemV1, 0, min(len(definitions), request.Limit))}
	for _, definition := range definitions {
		scopes, valid := catalogConversationScopesV1(definition, request.TenantID, principalUUID, activeAt)
		if !valid {
			return nil, application.ErrAgentDefinitionCatalogConflict
		}
		page.Definitions = append(page.Definitions, application.AgentDefinitionCatalogItemV1{
			DefinitionUUID: definition.DefinitionUUID, Version: definition.Version, AgentUUID: definition.AgentUUID,
			ConversationScopes: scopes, ValidFrom: definition.ValidFrom, ExpiresAt: definition.ExpiresAt,
			CreatedAt: definition.CreatedAt, UpdatedAt: definition.UpdatedAt,
		})
	}
	if len(page.Definitions) > request.Limit {
		page.Definitions = page.Definitions[:request.Limit]
		last := page.Definitions[len(page.Definitions)-1]
		page.NextDefinitionUUID, page.NextVersion = last.DefinitionUUID, last.Version
	}
	return page, nil
}

func catalogConversationScopesV1(definition application.AgentDefinitionVersionV1, tenantID, ownerUUID string, activeAt time.Time) ([]string, bool) {
	if definition.Validate() != nil || definition.TenantID != tenantID || definition.OwnerUUID != ownerUUID ||
		definition.Status != application.AgentDefinitionStatusActive || definition.RevokedAt != nil || activeAt.Before(definition.ValidFrom) ||
		(definition.ExpiresAt != nil && !activeAt.Before(*definition.ExpiresAt)) {
		return nil, false
	}
	hasPermission := false
	for _, permission := range definition.Permissions {
		if strings.TrimSpace(permission) == application.AgentPermissionConversationRead {
			hasPermission = true
			break
		}
	}
	if !hasPermission {
		return nil, false
	}
	unique := make(map[string]struct{})
	for _, scope := range definition.Scopes {
		if scope.ResourceType != application.AgentResourceTypeConversation {
			continue
		}
		for _, action := range scope.Actions {
			if action == application.AgentResourceActionRead || action == application.AgentResourceWildcard {
				unique[scope.ResourceID] = struct{}{}
				break
			}
		}
	}
	if len(unique) == 0 {
		return nil, false
	}
	scopes := make([]string, 0, len(unique))
	for scope := range unique {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)
	return scopes, true
}
