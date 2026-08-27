package application

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	AgentMemoryContentMaxBytesV1        = 16 * 1024
	AgentMemoryCompactContentMaxBytesV1 = 4 * 1024
)

var (
	ErrAgentMemoryInvalid = errors.New("Agent Memory is invalid")
	ErrAgentMemoryDenied  = errors.New("Agent Memory access denied")
)

type AgentMemoryTypeV1 string
type AgentMemoryStatusV1 string

const (
	AgentMemoryTypeWorking       AgentMemoryTypeV1 = "working"
	AgentMemoryTypeEpisodic      AgentMemoryTypeV1 = "episodic"
	AgentMemoryTypeSemantic      AgentMemoryTypeV1 = "semantic"
	AgentMemoryTypeProcedural    AgentMemoryTypeV1 = "procedural"
	AgentMemoryTypeObservational AgentMemoryTypeV1 = "observational"

	AgentMemoryStatusActive  AgentMemoryStatusV1 = "active"
	AgentMemoryStatusRevoked AgentMemoryStatusV1 = "revoked"
)

type AgentMemoryProvenanceV1 struct {
	SourceType string `json:"source_type"`
	SourceID   string `json:"source_id"`
	URI        string `json:"uri,omitempty"`
	Sequence   string `json:"sequence,omitempty"`
}

type AgentMemoryV1 struct {
	MemoryUUID     string                  `json:"memory_uuid"`
	TenantID       string                  `json:"tenant_id"`
	PrincipalUUID  string                  `json:"principal_uuid"`
	AgentUUID      string                  `json:"agent_uuid"`
	MemoryType     AgentMemoryTypeV1       `json:"memory_type"`
	Status         AgentMemoryStatusV1     `json:"status"`
	ResourceType   string                  `json:"resource_type"`
	ResourceID     string                  `json:"resource_id"`
	Content        string                  `json:"content"`
	CompactContent string                  `json:"compact_content,omitempty"`
	Priority       int32                   `json:"priority"`
	Provenance     AgentMemoryProvenanceV1 `json:"provenance"`
	ValidFrom      time.Time               `json:"valid_from"`
	ExpiresAt      *time.Time              `json:"expires_at,omitempty"`
	RevokedAt      *time.Time              `json:"revoked_at,omitempty"`
	CreatedAt      time.Time               `json:"created_at,omitempty"`
}

type AgentMemoryQueryV1 struct {
	TenantID      string
	PrincipalUUID string
	AgentUUID     string
	ResourceType  string
	ResourceID    string
	CreatedBefore time.Time
	At            time.Time
	Limit         int
}

type AgentMemoryStoreV1 interface {
	CreateMemory(ctx context.Context, memory AgentMemoryV1) error
	ListContextMemories(ctx context.Context, query AgentMemoryQueryV1) ([]AgentMemoryV1, error)
	RevokeMemory(ctx context.Context, memoryUUID string, revokedAt time.Time) error
}

type AgentMemoryContextResolverV1 interface {
	ResolveContextMemories(ctx context.Context, taskUUID, runUUID, resourceType, resourceID string, limit int) ([]AgentMemoryV1, error)
}

func (m AgentMemoryV1) Validate() error {
	if anyBlank(m.MemoryUUID, m.TenantID, m.PrincipalUUID, m.AgentUUID, m.ResourceType, m.ResourceID, m.Content,
		m.Provenance.SourceType, m.Provenance.SourceID) || m.ValidFrom.IsZero() || m.Priority < 0 || m.Priority > 1000 ||
		utf8.RuneCountInString(strings.TrimSpace(m.MemoryUUID)) > 64 || utf8.RuneCountInString(strings.TrimSpace(m.TenantID)) > 64 ||
		utf8.RuneCountInString(strings.TrimSpace(m.PrincipalUUID)) > 64 || utf8.RuneCountInString(strings.TrimSpace(m.AgentUUID)) > 24 ||
		utf8.RuneCountInString(strings.TrimSpace(m.ResourceType)) > 64 || utf8.RuneCountInString(strings.TrimSpace(m.ResourceID)) > 128 ||
		len(m.Content) > AgentMemoryContentMaxBytesV1 || len(m.CompactContent) > AgentMemoryCompactContentMaxBytesV1 ||
		utf8.RuneCountInString(strings.TrimSpace(m.Provenance.SourceType)) > 64 || utf8.RuneCountInString(strings.TrimSpace(m.Provenance.SourceID)) > 128 ||
		utf8.RuneCountInString(strings.TrimSpace(m.Provenance.URI)) > 512 || utf8.RuneCountInString(strings.TrimSpace(m.Provenance.Sequence)) > 128 ||
		!validAgentMemoryTypeV1(m.MemoryType) || !validAgentMemoryStatusV1(m.Status) ||
		(m.ExpiresAt != nil && !m.ExpiresAt.After(m.ValidFrom)) ||
		(m.Status == AgentMemoryStatusActive && m.RevokedAt != nil) || (m.Status == AgentMemoryStatusRevoked && m.RevokedAt == nil) {
		return ErrAgentMemoryInvalid
	}
	return nil
}

func validAgentMemoryTypeV1(value AgentMemoryTypeV1) bool {
	switch value {
	case AgentMemoryTypeWorking, AgentMemoryTypeEpisodic, AgentMemoryTypeSemantic, AgentMemoryTypeProcedural, AgentMemoryTypeObservational:
		return true
	default:
		return false
	}
}

func validAgentMemoryStatusV1(value AgentMemoryStatusV1) bool {
	return value == AgentMemoryStatusActive || value == AgentMemoryStatusRevoked
}
