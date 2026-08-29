package coreapplication

import "testing"

func TestNewConversationApplicationBuildsLocalAdapter(t *testing.T) {
	if NewConversationApplication(nil, nil, nil, ConversationDependencies{}) == nil {
		t.Fatal("NewConversationApplication() returned nil")
	}
}
