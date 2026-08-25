package app

import "testing"

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
	}
	for name, repo := range required {
		if repo == nil {
			t.Errorf("repository %s is nil", name)
		}
	}
}
