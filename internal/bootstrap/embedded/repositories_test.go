package embedded

import (
	"database/sql"
	"testing"

	agentmysql "github.com/JekYUlll/Dipole/internal/services/agent/infrastructure/mysql"
	coremysql "github.com/JekYUlll/Dipole/internal/services/core/infrastructure/mysql"
	messagemysql "github.com/JekYUlll/Dipole/internal/services/message/infrastructure/mysql"
	syncmysql "github.com/JekYUlll/Dipole/internal/services/sync/infrastructure/mysql"
)

func TestNewRepositoriesRequiresDatabase(t *testing.T) {
	if _, err := NewRepositories(nil); err == nil {
		t.Fatal("expected missing database connection to fail")
	}
}

func TestNewMessageProcessRepositoriesRequiresDatabase(t *testing.T) {
	if _, err := NewMessageProcessRepositories(nil); err == nil {
		t.Fatal("expected nil database to fail")
	}
}

func TestNewCoreProcessRepositoriesOwnsCoreStores(t *testing.T) {
	if _, err := coremysql.NewProcessRepositories(nil); err == nil {
		t.Fatal("expected nil database to fail")
	}
	repos, err := coremysql.NewProcessRepositories(&sql.DB{})
	if err != nil {
		t.Fatalf("new core process repositories: %v", err)
	}
	if _, ok := repos.Users.(*coremysql.CachedUserStore); !ok {
		t.Fatalf("expected cached user store, got %T", repos.Users)
	}
	if _, ok := repos.Groups.(*coremysql.CachedGroupStore); !ok {
		t.Fatalf("expected cached group store, got %T", repos.Groups)
	}
	if _, ok := repos.Conversations.(*coremysql.ConversationRepository); !ok {
		t.Fatalf("expected sqlc conversation repository, got %T", repos.Conversations)
	}
}

func TestNewAgentProcessRepositoriesOwnsAgentStores(t *testing.T) {
	if _, err := agentmysql.NewProcessRepositories(nil); err == nil {
		t.Fatal("expected nil database to fail")
	}
	repos, err := agentmysql.NewProcessRepositories(&sql.DB{})
	if err != nil {
		t.Fatalf("new agent process repositories: %v", err)
	}
	if _, ok := repos.Policy.(*agentmysql.AgentPolicyRepository); !ok {
		t.Fatalf("expected sqlc Agent Policy repository, got %T", repos.Policy)
	}
	if _, ok := repos.Artifacts.(*agentmysql.AgentArtifactRepository); !ok {
		t.Fatalf("expected sqlc Agent Artifact repository, got %T", repos.Artifacts)
	}
}

func TestNewSyncProcessRepositoriesOwnsOnlySyncStore(t *testing.T) {
	if _, err := syncmysql.NewProcessRepositories(nil, nil); err == nil {
		t.Fatal("expected nil database to fail")
	}
	repos, err := syncmysql.NewProcessRepositories(&sql.DB{}, nil)
	if err != nil {
		t.Fatalf("new sync process repositories: %v", err)
	}
	if _, ok := repos.Sync.(*syncmysql.SyncRepository); !ok {
		t.Fatalf("expected sqlc sync repository, got %T", repos.Sync)
	}
	if _, ok := repos.Projection.(*syncmysql.SyncProjectionRepository); !ok {
		t.Fatalf("expected sqlc Sync projection repository, got %T", repos.Projection)
	}
}

func TestNewMessageProcessRepositoriesBuildsOnlyOwnedAdapters(t *testing.T) {
	repos, err := NewMessageProcessRepositories(&sql.DB{})
	if err != nil {
		t.Fatalf("new message process repositories: %v", err)
	}
	if _, ok := repos.Messages.(*messagemysql.MessageRepository); !ok {
		t.Fatalf("expected sqlc message repository, got %T", repos.Messages)
	}
	if _, ok := repos.Outbox.(*messagemysql.OutboxRepository); !ok {
		t.Fatalf("expected sqlc outbox repository, got %T", repos.Outbox)
	}
}

func TestNewRepositoriesBuildsSQLCRepositorySet(t *testing.T) {
	repos, err := NewRepositories(&sql.DB{})
	if err != nil {
		t.Fatalf("compose repositories: %v", err)
	}
	for name, process := range map[string]any{
		"core process": repos.CoreProcess, "message process": repos.MessageProcess,
		"sync process": repos.SyncProcess, "agent process": repos.AgentProcess,
	} {
		if process == nil {
			t.Errorf("repository %s composition is nil", name)
		}
	}
	coreRepos := repos.CoreProcess
	required := map[string]any{
		"users": coreRepos.Users, "messages": repos.MessageProcess.Messages, "files": coreRepos.Files,
		"conversations": coreRepos.Conversations, "contacts": coreRepos.Contacts,
		"groups": coreRepos.Groups, "admin": coreRepos.Admin, "sync": repos.SyncProcess.Sync,
		"ai_call_logs": repos.AgentProcess.AICallLogs, "agent_policy": repos.AgentProcess.Policy, "outbox": repos.MessageProcess.Outbox,
	}
	for name, repository := range required {
		if repository == nil {
			t.Errorf("repository %s is nil", name)
		}
	}
	if _, ok := repos.AgentProcess.AICallLogs.(*agentmysql.AICallLogRepository); !ok {
		t.Fatalf("expected sqlc AI call log repository, got %T", repos.AgentProcess.AICallLogs)
	}
	if _, ok := repos.AgentProcess.Policy.(*agentmysql.AgentPolicyRepository); !ok {
		t.Fatalf("expected sqlc Agent Policy repository, got %T", repos.AgentProcess.Policy)
	}
	if _, ok := coreRepos.Admin.(*coremysql.AdminRepository); !ok {
		t.Fatalf("expected sqlc admin repository, got %T", coreRepos.Admin)
	}
	if _, ok := coreRepos.Files.(*coremysql.FileRepository); !ok {
		t.Fatalf("expected sqlc file repository, got %T", coreRepos.Files)
	}
	if _, ok := coreRepos.Users.(*coremysql.CachedUserStore); !ok {
		t.Fatalf("expected cached user store, got %T", coreRepos.Users)
	}
	if _, ok := coreRepos.Contacts.(*coremysql.CachedContactStore); !ok {
		t.Fatalf("expected cached contact store, got %T", coreRepos.Contacts)
	}
	if _, ok := coreRepos.Groups.(*coremysql.CachedGroupStore); !ok {
		t.Fatalf("expected cached group store, got %T", coreRepos.Groups)
	}
	if _, ok := coreRepos.Conversations.(*coremysql.ConversationRepository); !ok {
		t.Fatalf("expected sqlc conversation repository, got %T", coreRepos.Conversations)
	}
	if _, ok := repos.MessageProcess.Messages.(*messagemysql.MessageRepository); !ok {
		t.Fatalf("expected sqlc message repository, got %T", repos.MessageProcess.Messages)
	}
	if _, ok := repos.SyncProcess.Sync.(*syncmysql.SyncRepository); !ok {
		t.Fatalf("expected sqlc sync repository, got %T", repos.SyncProcess.Sync)
	}
	if _, ok := repos.MessageProcess.Outbox.(*messagemysql.OutboxRepository); !ok {
		t.Fatalf("expected sqlc outbox repository, got %T", repos.MessageProcess.Outbox)
	}
}
