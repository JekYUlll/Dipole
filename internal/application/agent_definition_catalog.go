package application

import (
	"context"
	"errors"
	"time"
)

var ErrAgentDefinitionCatalogInvalid = errors.New("Agent Definition catalog request is invalid")
var ErrAgentDefinitionCatalogConflict = errors.New("Agent Definition catalog authority changed")

const (
	// AgentDefinitionCatalogProfileReadOnly preserves the existing owner-scoped
	// conversation-read Definition for callers that omit a profile.
	AgentDefinitionCatalogProfileReadOnly = "read_only"
	// AgentDefinitionCatalogProfileSubscriptionAutoReply is an explicit opt-in
	// for the bounded autonomous subscription reply capability.
	AgentDefinitionCatalogProfileSubscriptionAutoReply = "subscription_autoreply"
)

type AgentDefinitionCatalogItemV1 struct {
	DefinitionUUID     string
	Version            uint64
	AgentUUID          string
	ConversationScopes []string
	ValidFrom          time.Time
	ExpiresAt          *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type AgentDefinitionCatalogListRequestV1 struct {
	TenantID            string
	AfterDefinitionUUID string
	AfterVersion        uint64
	Limit               int
}

type AgentDefinitionCatalogCreateRequestV1 struct {
	TenantID string
	Profile  string
}

type AgentDefinitionCatalogPageV1 struct {
	Definitions        []AgentDefinitionCatalogItemV1
	NextDefinitionUUID string
	NextVersion        uint64
}

type AgentDefinitionCatalogServiceV1 interface {
	Create(ctx context.Context, principalUUID string, request AgentDefinitionCatalogCreateRequestV1) (*AgentDefinitionVersionV1, error)
	List(ctx context.Context, principalUUID string, request AgentDefinitionCatalogListRequestV1) (*AgentDefinitionCatalogPageV1, error)
}

type AgentDefinitionCatalogStoreV1 interface {
	CreateDefinitionVersion(ctx context.Context, definition AgentDefinitionVersionV1) error
	GetDefinitionVersion(ctx context.Context, definitionUUID string, version uint64) (*AgentDefinitionVersionV1, error)
	ListOwnedActiveDefinitions(ctx context.Context, tenantID, ownerUUID, afterDefinitionUUID string, afterVersion uint64, activeAt time.Time, limit int) ([]AgentDefinitionVersionV1, error)
}
