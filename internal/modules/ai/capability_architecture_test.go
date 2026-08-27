package ai

import (
	"os"
	"strings"
	"testing"
)

func TestAgentRuntimeDependsOnCapabilityPortInsteadOfRepositories(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"context_builder.go", "tools.go"} {
		payload, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		source := string(payload)
		if !strings.Contains(source, "application.AgentCapabilityV1") {
			t.Errorf("%s does not depend on AgentCapabilityV1", path)
		}
		for _, forbidden := range []string{"type userReader interface", "type messageReader interface", "type conversationReader interface", "type systemMessageSender interface"} {
			if strings.Contains(source, forbidden) {
				t.Errorf("%s reintroduced repository-shaped Agent dependency %q", path, forbidden)
			}
		}
	}
}
