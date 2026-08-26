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
