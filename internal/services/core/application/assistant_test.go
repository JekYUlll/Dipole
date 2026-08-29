package coreapplication

import "testing"

func TestEnsureAIAssistantUserRequiresStore(t *testing.T) {
	if err := EnsureAIAssistantUser(nil); err == nil {
		t.Fatal("expected missing assistant user store error")
	}
}
