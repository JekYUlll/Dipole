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
	required := map[string]any{
		"users": repos.Users, "messages": repos.Messages, "files": repos.Files,
		"conversations": repos.Conversations, "contacts": repos.Contacts,
		"groups": repos.Groups, "admin": repos.Admin, "sync": repos.Sync,
		"ai_call_logs": repos.AICallLogs, "outbox": repos.Outbox,
	}
	for name, repository := range required {
		if repository == nil {
			t.Errorf("repository %s is nil", name)
		}
	}
	if _, ok := repos.AICallLogs.(*sqlcRepository.AICallLogRepository); !ok {
		t.Fatalf("expected sqlc AI call log repository, got %T", repos.AICallLogs)
	}
	if _, ok := repos.Admin.(*sqlcRepository.AdminRepository); !ok {
		t.Fatalf("expected sqlc admin repository, got %T", repos.Admin)
	}
	if _, ok := repos.Files.(*sqlcRepository.FileRepository); !ok {
		t.Fatalf("expected sqlc file repository, got %T", repos.Files)
	}
	if cached, ok := repos.Users.(*CachedUserStore); !ok {
		t.Fatalf("expected cached user store, got %T", repos.Users)
	} else if _, ok := cached.backend.(*sqlcRepository.UserRepository); !ok {
		t.Fatalf("expected sqlc user backend, got %T", cached.backend)
	}
	if cached, ok := repos.Contacts.(*CachedContactStore); !ok {
		t.Fatalf("expected cached contact store, got %T", repos.Contacts)
	} else if _, ok := cached.backend.(*sqlcRepository.ContactRepository); !ok {
		t.Fatalf("expected sqlc contact backend, got %T", cached.backend)
	}
	if cached, ok := repos.Groups.(*CachedGroupStore); !ok {
		t.Fatalf("expected cached group store, got %T", repos.Groups)
	} else if _, ok := cached.backend.(*sqlcRepository.GroupRepository); !ok {
		t.Fatalf("expected sqlc group backend, got %T", cached.backend)
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
