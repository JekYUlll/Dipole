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
	AgentDefinitionCatalog application.AgentDefinitionCatalogStoreV1
	AgentApprovalGrants    application.AgentApprovalGrantStoreV1
	AgentPromotions        application.AgentRuntimePromotionGrantStoreV1
	AgentPromotionControls application.AgentRuntimePromotionControlStoreV1
	AgentReadinessEvidence application.AgentMCPReadinessEvidenceStoreV1
	AgentSubscriptions     application.AgentEventSubscriptionStoreV1
	AgentRepairs           application.AgentWorkflowRepairAuditStoreV1
	AgentArtifacts         application.AgentArtifactStoreV1
	AgentMemories          application.AgentMemoryStoreV1
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
	repos := &Repositories{}
	adapter, err := sqlcRepository.NewAICallLogRepository(generated.New(db))
	if err != nil {
		return nil, fmt.Errorf("create sqlc AI call log repository: %w", err)
	}
	repos.AICallLogs = adapter
	agentPolicy, err := sqlcRepository.NewAgentPolicyRepository(generated.New(db))
	if err != nil {
		return nil, fmt.Errorf("create sqlc Agent Policy repository: %w", err)
	}
	repos.AgentPolicy = agentPolicy
	repos.AgentDefinitionCatalog = agentPolicy
	repos.AgentApprovalGrants = agentPolicy
	repos.AgentPromotions = agentPolicy
	repos.AgentSubscriptions = agentPolicy
	repos.AgentRepairs = agentPolicy
	agentArtifacts, err := sqlcRepository.NewAgentArtifactRepository(generated.New(db))
	if err != nil {
		return nil, fmt.Errorf("create sqlc Agent Artifact repository: %w", err)
	}
	repos.AgentArtifacts = agentArtifacts
	agentMemories, err := sqlcRepository.NewAgentMemoryRepository(generated.New(db))
	if err != nil {
		return nil, fmt.Errorf("create sqlc Agent Memory repository: %w", err)
	}
	repos.AgentMemories = agentMemories
	agentToolAudits, err := sqlcRepository.NewAgentToolInvocationRepository(generated.New(db))
	if err != nil {
		return nil, fmt.Errorf("create sqlc Agent Tool invocation repository: %w", err)
	}
	repos.AgentToolAudits = agentToolAudits
	agentToolRounds, err := sqlcRepository.NewAgentMCPToolRoundRepository(generated.New(db))
	if err != nil {
		return nil, fmt.Errorf("create sqlc Agent MCP Tool round repository: %w", err)
	}
	repos.AgentToolRounds = agentToolRounds
	adminAdapter, err := sqlcRepository.NewAdminRepository(generated.New(db))
	if err != nil {
		return nil, fmt.Errorf("create sqlc admin repository: %w", err)
	}
	repos.Admin = adminAdapter
	fileAdapter, err := sqlcRepository.NewFileRepository(generated.New(db))
	if err != nil {
		return nil, fmt.Errorf("create sqlc file repository: %w", err)
	}
	repos.Files = fileAdapter
	userAdapter, err := sqlcRepository.NewUserRepository(generated.New(db))
	if err != nil {
		return nil, fmt.Errorf("create sqlc user repository: %w", err)
	}
	repos.Users = NewCachedUserStore(userAdapter)
	contactAdapter, err := sqlcRepository.NewContactRepository(generated.New(db))
	if err != nil {
		return nil, fmt.Errorf("create sqlc contact repository: %w", err)
	}
	repos.Contacts = NewCachedContactStore(contactAdapter)
	mysqlStore, err := mysqlData.NewStore(db)
	if err != nil {
		return nil, fmt.Errorf("create sqlc transaction store: %w", err)
	}
	promotionControls, err := sqlcRepository.NewAgentRuntimePromotionControlRepository(mysqlStore)
	if err != nil {
		return nil, fmt.Errorf("create sqlc Agent Runtime promotion control repository: %w", err)
	}
	repos.AgentPromotionControls = promotionControls
	readinessEvidence, err := sqlcRepository.NewAgentMCPReadinessEvidenceRepository(generated.New(db))
	if err != nil {
		return nil, fmt.Errorf("create sqlc Agent MCP readiness evidence repository: %w", err)
	}
	repos.AgentReadinessEvidence = readinessEvidence
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
	groupAdapter, err := sqlcRepository.NewGroupRepository(mysqlStore)
	if err != nil {
		return nil, fmt.Errorf("create sqlc group repository: %w", err)
	}
	repos.Groups = NewCachedGroupStore(groupAdapter)
	conversationAdapter, err := sqlcRepository.NewConversationRepository(generated.New(db))
	if err != nil {
		return nil, fmt.Errorf("create sqlc conversation repository: %w", err)
	}
	repos.Conversations = conversationAdapter
	outboxAdapter, err := sqlcRepository.NewOutboxRepository(mysqlStore)
	if err != nil {
		return nil, fmt.Errorf("create sqlc outbox relay repository: %w", err)
	}
	repos.Outbox = outboxAdapter
	return repos, nil
}
