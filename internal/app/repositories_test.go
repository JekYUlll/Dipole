package app

import (
	"database/sql"
	"testing"

	sqlcRepository "github.com/JekYUlll/Dipole/internal/data/mysql/repository"
	gormRepository "github.com/JekYUlll/Dipole/internal/repository"
)

func TestNewRepositoriesBuildsApplicationRepositorySet(t *testing.T) {
	repos := NewRepositories()

	if repos == nil {
		t.Fatal("NewRepositories() returned nil")
	}

	required := map[string]any{
		"users":         repos.Users,
		"messages":      repos.Messages,
		"files":         repos.Files,
		"conversations": repos.Conversations,
		"contacts":      repos.Contacts,
		"groups":        repos.Groups,
		"admin":         repos.Admin,
		"sync":          repos.Sync,
		"ai_call_logs":  repos.AICallLogs,
		"outbox":        repos.Outbox,
	}
	for name, repo := range required {
		if repo == nil {
			t.Errorf("repository %s is nil", name)
		}
	}
}

func TestNewRepositoriesWithOptionsSelectsAICallLogAdapter(t *testing.T) {
	t.Parallel()

	t.Run("gorm default", func(t *testing.T) {
		repos, err := NewRepositoriesWithOptions(RepositoryOptions{})
		if err != nil {
			t.Fatalf("compose repositories: %v", err)
		}
		if _, ok := repos.AICallLogs.(*gormRepository.AICallLogRepository); !ok {
			t.Fatalf("expected GORM adapter, got %T", repos.AICallLogs)
		}
		if _, ok := repos.Admin.(*gormRepository.AdminRepository); !ok {
			t.Fatalf("expected GORM admin adapter, got %T", repos.Admin)
		}
		if _, ok := repos.Files.(*gormRepository.FileRepository); !ok {
			t.Fatalf("expected GORM file adapter, got %T", repos.Files)
		}
		cached, ok := repos.Users.(*CachedUserStore)
		if !ok {
			t.Fatalf("expected cached user store, got %T", repos.Users)
		}
		if _, ok := cached.backend.(*gormRepository.UserRepository); !ok {
			t.Fatalf("expected GORM user backend, got %T", cached.backend)
		}
		cachedContacts, ok := repos.Contacts.(*CachedContactStore)
		if !ok {
			t.Fatalf("expected cached contact store, got %T", repos.Contacts)
		}
		if _, ok := cachedContacts.backend.(*gormRepository.ContactRepository); !ok {
			t.Fatalf("expected GORM contact backend, got %T", cachedContacts.backend)
		}
		cachedGroups, ok := repos.Groups.(*CachedGroupStore)
		if !ok {
			t.Fatalf("expected cached group store, got %T", repos.Groups)
		}
		if _, ok := cachedGroups.backend.(*gormRepository.GroupRepository); !ok {
			t.Fatalf("expected GORM group backend, got %T", cachedGroups.backend)
		}
	})

	t.Run("sqlc", func(t *testing.T) {
		repos, err := NewRepositoriesWithOptions(RepositoryOptions{
			MySQLAdapter: MySQLAdapterSQLC,
			SQLDB:        &sql.DB{},
		})
		if err != nil {
			t.Fatalf("compose repositories: %v", err)
		}
		if _, ok := repos.AICallLogs.(*sqlcRepository.AICallLogRepository); !ok {
			t.Fatalf("expected sqlc adapter, got %T", repos.AICallLogs)
		}
		if _, ok := repos.Admin.(*sqlcRepository.AdminRepository); !ok {
			t.Fatalf("expected sqlc admin adapter, got %T", repos.Admin)
		}
		if _, ok := repos.Files.(*sqlcRepository.FileRepository); !ok {
			t.Fatalf("expected sqlc file adapter, got %T", repos.Files)
		}
		cached, ok := repos.Users.(*CachedUserStore)
		if !ok {
			t.Fatalf("expected cached user store, got %T", repos.Users)
		}
		if _, ok := cached.backend.(*sqlcRepository.UserRepository); !ok {
			t.Fatalf("expected sqlc user backend, got %T", cached.backend)
		}
		cachedContacts, ok := repos.Contacts.(*CachedContactStore)
		if !ok {
			t.Fatalf("expected cached contact store, got %T", repos.Contacts)
		}
		if _, ok := cachedContacts.backend.(*sqlcRepository.ContactRepository); !ok {
			t.Fatalf("expected sqlc contact backend, got %T", cachedContacts.backend)
		}
		cachedGroups, ok := repos.Groups.(*CachedGroupStore)
		if !ok {
			t.Fatalf("expected cached group store, got %T", repos.Groups)
		}
		if _, ok := cachedGroups.backend.(*sqlcRepository.GroupRepository); !ok {
			t.Fatalf("expected sqlc group backend, got %T", cachedGroups.backend)
		}
	})

	t.Run("sqlc requires connection", func(t *testing.T) {
		if _, err := NewRepositoriesWithOptions(RepositoryOptions{MySQLAdapter: MySQLAdapterSQLC}); err == nil {
			t.Fatal("expected missing SQL connection to fail")
		}
	})

	t.Run("unknown adapter", func(t *testing.T) {
		if _, err := NewRepositoriesWithOptions(RepositoryOptions{MySQLAdapter: "unknown"}); err == nil {
			t.Fatal("expected unknown adapter to fail")
		}
	})
}
