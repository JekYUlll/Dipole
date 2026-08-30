package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentApprovedCapabilityProjectionRemainsProductionDefaultOff(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	bootstrap, err := os.ReadFile(filepath.Join(root, "internal", "bootstrap", "embedded", "runtime", "runtime.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bootstrap), "if rpcCfg.AgentMemoryPromotionReceiptCommitEnabled {") ||
		!strings.Contains(string(bootstrap), "Agent Memory promotion receipt commit requires internal RPC mTLS") {
		t.Fatal("production Bootstrap must guard receipt promotion authority behind explicit mTLS opt-in")
	}
	entrypoint, err := os.ReadFile(filepath.Join(root, "services", "agent-runtime", "src", "index.ts"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"McpMessageWriteProjection", `capabilityId: "message.system.send"`} {
		if strings.Contains(string(entrypoint), forbidden) {
			t.Fatalf("production Agent Runtime must not register %s", forbidden)
		}
	}
}
