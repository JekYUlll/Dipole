// Package app owns process-level dependency composition shared by transports.
package app

import (
	"database/sql"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	mysqlData "github.com/JekYUlll/Dipole/internal/data/mysql"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/data/mysql/repository"
)

// Repositories contains one repository instance for each application process.
type Repositories struct {
	Users                  application.UserStore
	Messages               application.MessageStore
	Files                  application.FileMetadataStore
	Conversations          application.ConversationStore
	Contacts               application.ContactStore
	Groups                 application.GroupStore
	Admin                  application.AdminOverviewStore
	Sync                   application.SyncStore
	Search                 application.SearchIndex
	AICallLogs             application.AICallLogStore
	AgentPolicy            application.AgentPolicyStoreV1
	AgentTaskTimeline      application.AgentTaskTimelineStoreV1
	AgentDefinitionCatalog application.AgentDefinitionCatalogStoreV1
	AgentApprovalGrants    application.AgentApprovalGrantStoreV1
	AgentPromotions        application.AgentRuntimePromotionGrantStoreV1
	AgentPromotionControls application.AgentRuntimePromotionControlStoreV1
	AgentReadinessEvidence application.AgentMCPReadinessEvidenceStoreV1
	AgentSubscriptions     application.AgentEventSubscriptionStoreV1
	AgentRepairs           application.AgentWorkflowRepairAuditStoreV1
	AgentArtifacts         application.AgentArtifactStoreV1
	AgentMemories          application.AgentMemoryStoreV1
	AgentMemoryOwners      application.AgentMemoryOwnerStoreV1
	AgentMemoryPromotions  application.AgentMemoryCandidatePromotionStoreV1
	AgentToolAudits        application.AgentToolInvocationStoreV1
	AgentToolRounds        application.AgentMCPToolRoundStoreV1
	Outbox                 application.OutboxRelayStore
}

type MessageProcessRepositories struct {
	Messages             application.MessageStore
	Outbox               application.OutboxRelayStore
	ConversationSequence *sqlcRepository.ConversationSequenceRepository
}

type SyncProcessRepositories struct {
	Sync       application.SyncStore
	Projection application.SyncProjectionStore
}

// CoreProcessRepositories contains only repositories owned by the Core service.
// The aggregate Repositories type below remains as a compatibility composition
// root for the embedded server during the migration.
type CoreProcessRepositories struct {
	Users         application.UserStore
	Files         application.FileMetadataStore
	Conversations application.ConversationStore
	Contacts      application.ContactStore
	Groups        application.GroupStore
	Admin         application.AdminOverviewStore
}

// AgentProcessRepositories contains repositories owned by the Agent runtime.
// The Go Core still consumes selected Agent ports for the compatibility RPC
// surface, while the process boundary remains explicit for later extraction.
type AgentProcessRepositories struct {
	AICallLogs        application.AICallLogStore
	Policy            application.AgentPolicyStoreV1
	TaskTimeline      application.AgentTaskTimelineStoreV1
	DefinitionCatalog application.AgentDefinitionCatalogStoreV1
	ApprovalGrants    application.AgentApprovalGrantStoreV1
	Promotions        application.AgentRuntimePromotionGrantStoreV1
	PromotionControls application.AgentRuntimePromotionControlStoreV1
	ReadinessEvidence application.AgentMCPReadinessEvidenceStoreV1
	Subscriptions     application.AgentEventSubscriptionStoreV1
	Repairs           application.AgentWorkflowRepairAuditStoreV1
	Artifacts         application.AgentArtifactStoreV1
	Memories          application.AgentMemoryStoreV1
	MemoryOwners      application.AgentMemoryOwnerStoreV1
	MemoryPromotions  application.AgentMemoryCandidatePromotionStoreV1
	ToolAudits        application.AgentToolInvocationStoreV1
	ToolRounds        application.AgentMCPToolRoundStoreV1
}

func NewAgentProcessRepositories(db *sql.DB) (*AgentProcessRepositories, error) {
	if db == nil {
		return nil, fmt.Errorf("agent repository composition requires database/sql connection")
	}
	mysqlStore, err := mysqlData.NewStore(db)
	if err != nil {
		return nil, fmt.Errorf("create agent transaction store: %w", err)
	}
	return newAgentProcessRepositories(db, mysqlStore)
}

func newAgentProcessRepositories(db *sql.DB, mysqlStore *mysqlData.Store) (*AgentProcessRepositories, error) {
	queries := generated.New(db)
	aiCallLogs, err := sqlcRepository.NewAICallLogRepository(queries)
	if err != nil {
		return nil, fmt.Errorf("create sqlc AI call log repository: %w", err)
	}
	policy, err := sqlcRepository.NewAgentPolicyRepositoryWithTransactions(mysqlStore)
	if err != nil {
		return nil, fmt.Errorf("create sqlc Agent Policy repository: %w", err)
	}
	artifacts, err := sqlcRepository.NewAgentArtifactRepository(queries)
	if err != nil {
		return nil, fmt.Errorf("create sqlc Agent Artifact repository: %w", err)
	}
	memories, err := sqlcRepository.NewAgentMemoryRepositoryWithTransactions(mysqlStore)
	if err != nil {
		return nil, fmt.Errorf("create sqlc Agent Memory repository: %w", err)
	}
	toolAudits, err := sqlcRepository.NewAgentToolInvocationRepository(queries)
	if err != nil {
		return nil, fmt.Errorf("create sqlc Agent Tool invocation repository: %w", err)
	}
	toolRounds, err := sqlcRepository.NewAgentMCPToolRoundRepository(queries)
	if err != nil {
		return nil, fmt.Errorf("create sqlc Agent MCP Tool round repository: %w", err)
	}
	promotionControls, err := sqlcRepository.NewAgentRuntimePromotionControlRepository(mysqlStore)
	if err != nil {
		return nil, fmt.Errorf("create sqlc Agent Runtime promotion control repository: %w", err)
	}
	readinessEvidence, err := sqlcRepository.NewAgentMCPReadinessEvidenceRepository(queries)
	if err != nil {
		return nil, fmt.Errorf("create sqlc Agent MCP readiness evidence repository: %w", err)
	}
	return &AgentProcessRepositories{
		AICallLogs: aiCallLogs, Policy: policy, TaskTimeline: policy,
		DefinitionCatalog: policy, ApprovalGrants: policy, Promotions: policy,
		Subscriptions: policy, Repairs: policy, Artifacts: artifacts,
		Memories: memories, MemoryOwners: memories, MemoryPromotions: memories,
		ToolAudits: toolAudits, ToolRounds: toolRounds,
		PromotionControls: promotionControls, ReadinessEvidence: readinessEvidence,
	}, nil
}

func NewCoreProcessRepositories(db *sql.DB) (*CoreProcessRepositories, error) {
	if db == nil {
		return nil, fmt.Errorf("core repository composition requires database/sql connection")
	}
	mysqlStore, err := mysqlData.NewStore(db)
	if err != nil {
		return nil, fmt.Errorf("create core transaction store: %w", err)
	}
	return newCoreProcessRepositories(db, mysqlStore)
}

func newCoreProcessRepositories(db *sql.DB, mysqlStore *mysqlData.Store) (*CoreProcessRepositories, error) {
	queries := generated.New(db)
	admin, err := sqlcRepository.NewAdminRepository(queries)
	if err != nil {
		return nil, fmt.Errorf("create sqlc admin repository: %w", err)
	}
	files, err := sqlcRepository.NewFileRepository(queries)
	if err != nil {
		return nil, fmt.Errorf("create sqlc file repository: %w", err)
	}
	users, err := sqlcRepository.NewUserRepository(queries)
	if err != nil {
		return nil, fmt.Errorf("create sqlc user repository: %w", err)
	}
	contacts, err := sqlcRepository.NewContactRepository(queries)
	if err != nil {
		return nil, fmt.Errorf("create sqlc contact repository: %w", err)
	}
	groups, err := sqlcRepository.NewGroupRepository(mysqlStore)
	if err != nil {
		return nil, fmt.Errorf("create sqlc group repository: %w", err)
	}
	conversations, err := sqlcRepository.NewConversationRepository(queries)
	if err != nil {
		return nil, fmt.Errorf("create sqlc conversation repository: %w", err)
	}
	return &CoreProcessRepositories{
		Admin: admin, Files: files,
		Users: NewCachedUserStore(users), Contacts: NewCachedContactStore(contacts),
		Groups: NewCachedGroupStore(groups), Conversations: conversations,
	}, nil
}

func NewSyncProcessRepositories(db *sql.DB) (*SyncProcessRepositories, error) {
	return NewSyncProcessRepositoriesWithHydrator(db, nil)
}

func NewSyncProcessRepositoriesWithHydrator(db *sql.DB, hydrator application.SyncMessageHydrator) (*SyncProcessRepositories, error) {
	if db == nil {
		return nil, fmt.Errorf("sync repository composition requires database/sql connection")
	}
	queries := generated.New(db)
	var syncStore *sqlcRepository.SyncRepository
	var err error
	if hydrator == nil {
		syncStore, err = sqlcRepository.NewSyncRepository(queries)
	} else {
		syncStore, err = sqlcRepository.NewSyncRepositoryWithHydrator(queries, hydrator)
	}
	if err != nil {
		return nil, fmt.Errorf("create sync repository: %w", err)
	}
	mysqlStore, err := mysqlData.NewStore(db)
	if err != nil {
		return nil, fmt.Errorf("create sync transaction store: %w", err)
	}
	projection, err := sqlcRepository.NewSyncProjectionRepository(mysqlStore)
	if err != nil {
		return nil, fmt.Errorf("create sync projection repository: %w", err)
	}
	return &SyncProcessRepositories{Sync: syncStore, Projection: projection}, nil
}

func NewMessageProcessRepositories(db *sql.DB) (*MessageProcessRepositories, error) {
	return NewMessageProcessRepositoriesWithInboxWrites(db, true)
}

func NewMessageProcessRepositoriesWithInboxWrites(db *sql.DB, enabled bool) (*MessageProcessRepositories, error) {
	if db == nil {
		return nil, fmt.Errorf("message repository composition requires database/sql connection")
	}
	mysqlStore, err := mysqlData.NewStore(db)
	if err != nil {
		return nil, fmt.Errorf("create message transaction store: %w", err)
	}
	messages, err := sqlcRepository.NewMessageRepositoryWithInboxWrites(mysqlStore, enabled)
	if err != nil {
		return nil, fmt.Errorf("create message repository: %w", err)
	}
	outbox, err := sqlcRepository.NewOutboxRepository(mysqlStore)
	if err != nil {
		return nil, fmt.Errorf("create message outbox repository: %w", err)
	}
	return &MessageProcessRepositories{
		Messages: messages, Outbox: outbox,
		ConversationSequence: sqlcRepository.NewConversationSequenceRepository(generated.New(db)),
	}, nil
}

func NewRepositories(db *sql.DB) (*Repositories, error) {
	if db == nil {
		return nil, fmt.Errorf("repository composition requires database/sql connection")
	}
	mysqlStore, err := mysqlData.NewStore(db)
	if err != nil {
		return nil, fmt.Errorf("create sqlc transaction store: %w", err)
	}
	repos := &Repositories{}
	coreRepos, err := newCoreProcessRepositories(db, mysqlStore)
	if err != nil {
		return nil, fmt.Errorf("compose Core repositories: %w", err)
	}
	repos.Users = coreRepos.Users
	repos.Files = coreRepos.Files
	repos.Conversations = coreRepos.Conversations
	repos.Contacts = coreRepos.Contacts
	repos.Groups = coreRepos.Groups
	repos.Admin = coreRepos.Admin
	agentRepos, err := newAgentProcessRepositories(db, mysqlStore)
	if err != nil {
		return nil, fmt.Errorf("compose Agent repositories: %w", err)
	}
	repos.AICallLogs = agentRepos.AICallLogs
	repos.AgentPolicy = agentRepos.Policy
	repos.AgentTaskTimeline = agentRepos.TaskTimeline
	repos.AgentDefinitionCatalog = agentRepos.DefinitionCatalog
	repos.AgentApprovalGrants = agentRepos.ApprovalGrants
	repos.AgentPromotions = agentRepos.Promotions
	repos.AgentPromotionControls = agentRepos.PromotionControls
	repos.AgentReadinessEvidence = agentRepos.ReadinessEvidence
	repos.AgentSubscriptions = agentRepos.Subscriptions
	repos.AgentRepairs = agentRepos.Repairs
	repos.AgentArtifacts = agentRepos.Artifacts
	repos.AgentMemories = agentRepos.Memories
	repos.AgentMemoryOwners = agentRepos.MemoryOwners
	repos.AgentMemoryPromotions = agentRepos.MemoryPromotions
	repos.AgentToolAudits = agentRepos.ToolAudits
	repos.AgentToolRounds = agentRepos.ToolRounds
	messageAdapter, err := sqlcRepository.NewMessageRepository(mysqlStore)
	if err != nil {
		return nil, fmt.Errorf("create sqlc message repository: %w", err)
	}
	repos.Messages = messageAdapter
	syncAdapter, err := sqlcRepository.NewSyncRepository(generated.New(db))
	if err != nil {
		return nil, fmt.Errorf("create sqlc sync repository: %w", err)
	}
	repos.Sync = syncAdapter
	searchAdapter, err := sqlcRepository.NewSearchIndexRepository(generated.New(db))
	if err != nil {
		return nil, fmt.Errorf("create sqlc search index repository: %w", err)
	}
	repos.Search = searchAdapter
	outboxAdapter, err := sqlcRepository.NewOutboxRepository(mysqlStore)
	if err != nil {
		return nil, fmt.Errorf("create sqlc outbox relay repository: %w", err)
	}
	repos.Outbox = outboxAdapter
	return repos, nil
}
