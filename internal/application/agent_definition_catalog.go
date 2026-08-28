package application

import (
	"context"
	"errors"
	"time"
)

var ErrAgentDefinitionCatalogInvalid = errors.New("Agent Definition catalog request is invalid")
var ErrAgentDefinitionCatalogConflict = errors.New("Agent Definition catalog authority changed")

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

type AgentDefinitionCatalogPageV1 struct {
	Definitions        []AgentDefinitionCatalogItemV1
	NextDefinitionUUID string
	NextVersion        uint64
}

type AgentDefinitionCatalogServiceV1 interface {
	List(ctx context.Context, principalUUID string, request AgentDefinitionCatalogListRequestV1) (*AgentDefinitionCatalogPageV1, error)
}

type AgentDefinitionCatalogStoreV1 interface {
	ListOwnedActiveDefinitions(ctx context.Context, tenantID, ownerUUID, afterDefinitionUUID string, afterVersion uint64, activeAt time.Time, limit int) ([]AgentDefinitionVersionV1, error)
}
