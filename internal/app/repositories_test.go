package app

import (
	"database/sql"
	"testing"

	sqlcRepository "github.com/JekYUlll/Dipole/internal/data/mysql/repository"
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
	if _, err := NewCoreProcessRepositories(nil); err == nil {
		t.Fatal("expected nil database to fail")
	}
	repos, err := NewCoreProcessRepositories(&sql.DB{})
	if err != nil {
		t.Fatalf("new core process repositories: %v", err)
	}
	if _, ok := repos.Users.(*CachedUserStore); !ok {
		t.Fatalf("expected cached user store, got %T", repos.Users)
	}
	if _, ok := repos.Groups.(*CachedGroupStore); !ok {
		t.Fatalf("expected cached group store, got %T", repos.Groups)
	}
	if _, ok := repos.Conversations.(*sqlcRepository.ConversationRepository); !ok {
		t.Fatalf("expected sqlc conversation repository, got %T", repos.Conversations)
	}
}

func TestNewAgentProcessRepositoriesOwnsAgentStores(t *testing.T) {
	if _, err := NewAgentProcessRepositories(nil); err == nil {
		t.Fatal("expected nil database to fail")
	}
	repos, err := NewAgentProcessRepositories(&sql.DB{})
	if err != nil {
		t.Fatalf("new agent process repositories: %v", err)
	}
	if _, ok := repos.Policy.(*sqlcRepository.AgentPolicyRepository); !ok {
		t.Fatalf("expected sqlc Agent Policy repository, got %T", repos.Policy)
	}
	if _, ok := repos.Artifacts.(*sqlcRepository.AgentArtifactRepository); !ok {
		t.Fatalf("expected sqlc Agent Artifact repository, got %T", repos.Artifacts)
	}
}

func TestNewSyncProcessRepositoriesOwnsOnlySyncStore(t *testing.T) {
	if _, err := NewSyncProcessRepositories(nil); err == nil {
		t.Fatal("expected nil database to fail")
	}
	repos, err := NewSyncProcessRepositories(&sql.DB{})
	if err != nil {
		t.Fatalf("new sync process repositories: %v", err)
	}
	if _, ok := repos.Sync.(*sqlcRepository.SyncRepository); !ok {
		t.Fatalf("expected sqlc sync repository, got %T", repos.Sync)
	}
	if _, ok := repos.Projection.(*sqlcRepository.SyncProjectionRepository); !ok {
		t.Fatalf("expected sqlc Sync projection repository, got %T", repos.Projection)
	}
}

func TestNewMessageProcessRepositoriesBuildsOnlyOwnedAdapters(t *testing.T) {
	repos, err := NewMessageProcessRepositories(&sql.DB{})
	if err != nil {
		t.Fatalf("new message process repositories: %v", err)
	}
	if _, ok := repos.Messages.(*sqlcRepository.MessageRepository); !ok {
		t.Fatalf("expected sqlc message repository, got %T", repos.Messages)
	}
	if _, ok := repos.Outbox.(*sqlcRepository.OutboxRepository); !ok {
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
	required := map[string]any{
		"users": repos.Users, "messages": repos.Messages, "files": repos.Files,
		"conversations": repos.Conversations, "contacts": repos.Contacts,
		"groups": repos.Groups, "admin": repos.Admin, "sync": repos.Sync,
		"ai_call_logs": repos.AICallLogs, "agent_policy": repos.AgentPolicy, "outbox": repos.Outbox,
	}
	for name, repository := range required {
		if repository == nil {
			t.Errorf("repository %s is nil", name)
		}
	}
	if _, ok := repos.AICallLogs.(*sqlcRepository.AICallLogRepository); !ok {
		t.Fatalf("expected sqlc AI call log repository, got %T", repos.AICallLogs)
	}
	if _, ok := repos.AgentPolicy.(*sqlcRepository.AgentPolicyRepository); !ok {
		t.Fatalf("expected sqlc Agent Policy repository, got %T", repos.AgentPolicy)
	}
	if _, ok := repos.Admin.(*sqlcRepository.AdminRepository); !ok {
		t.Fatalf("expected sqlc admin repository, got %T", repos.Admin)
	}
	if _, ok := repos.Files.(*sqlcRepository.FileRepository); !ok {
		t.Fatalf("expected sqlc file repository, got %T", repos.Files)
	}
	if _, ok := repos.Users.(*CachedUserStore); !ok {
		t.Fatalf("expected cached user store, got %T", repos.Users)
	}
	if _, ok := repos.Contacts.(*CachedContactStore); !ok {
		t.Fatalf("expected cached contact store, got %T", repos.Contacts)
	}
	if _, ok := repos.Groups.(*CachedGroupStore); !ok {
		t.Fatalf("expected cached group store, got %T", repos.Groups)
	}
	if _, ok := repos.Conversations.(*sqlcRepository.ConversationRepository); !ok {
		t.Fatalf("expected sqlc conversation repository, got %T", repos.Conversations)
	}
	if _, ok := repos.Messages.(*sqlcRepository.MessageRepository); !ok {
		t.Fatalf("expected sqlc message repository, got %T", repos.Messages)
	}
	if _, ok := repos.Sync.(*sqlcRepository.SyncRepository); !ok {
		t.Fatalf("expected sqlc sync repository, got %T", repos.Sync)
	}
	if _, ok := repos.Outbox.(*sqlcRepository.OutboxRepository); !ok {
		t.Fatalf("expected sqlc outbox repository, got %T", repos.Outbox)
	}
}
