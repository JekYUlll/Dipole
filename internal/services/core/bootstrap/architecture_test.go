package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoreRPCBootstrapOwnsPlatformTransport(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("rpc.go"))
	if err != nil {
		t.Fatalf("read Core RPC bootstrap: %v", err)
	}
	text := string(source)
	if strings.Contains(text, "internal/bootstrap\"") {
		t.Fatalf("Core RPC bootstrap must not import legacy bootstrap directly")
	}
	if !strings.Contains(text, "internal/platform/rpc") {
		t.Fatalf("Core RPC bootstrap must use platform RPC transport")
	}
}

func TestCoreRuntimeKeepsOAuthCallbackConsumptionExplicitAndMTLSBound(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("runtime.go"))
	if err != nil {
		t.Fatalf("read Core runtime: %v", err)
	}
	text := string(source)
	if !strings.Contains(text, "rpcCfg.AgentOAuthAuthorizationTransactionConsumeEnabled") {
		t.Fatal("Core OAuth transaction store injection must require an explicit gate")
	}
	if !strings.Contains(text, "Agent OAuth authorization transaction consumption requires internal RPC mTLS") {
		t.Fatal("Core OAuth transaction store injection must require mTLS")
	}
	if !strings.Contains(text, "NewOAuthAuthorizationTransactionServer") {
		t.Fatal("Core must compose the restricted OAuth transaction adapter")
	}
}

func TestCoreRuntimeComposesAgentTaskTimeline(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("runtime.go"))
	if err != nil {
		t.Fatalf("read Core runtime: %v", err)
	}
	text := string(source)
	if !strings.Contains(text, "agentServer.WithTaskTimeline(agentRepos.TaskTimeline)") {
		t.Fatal("standalone Core runtime must configure the Agent Task Timeline store")
	}
}

func TestCoreServiceEntrypointUsesOwnedRuntime(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("entrypoint.go"))
	if err != nil {
		t.Fatalf("read Core entrypoint: %v", err)
	}
	if !strings.Contains(string(source), "return InitializeCoreService(ctx)") {
		t.Fatalf("Core service entrypoint must initialize its owned runtime")
	}
	if strings.Contains(string(source), "internal/bootstrap") {
		t.Fatalf("Core service entrypoint must not import legacy bootstrap")
	}
	if strings.Contains(string(source), "legacybootstrap.RunServer") {
		t.Fatalf("Core service entrypoint must own its HTTP/TLS server startup")
	}
}

func TestEmbeddedRollbackBridgeOwnsAggregateDependency(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("embedded_compat.go"))
	if err != nil {
		t.Fatalf("read Core embedded compatibility bridge: %v", err)
	}
	if !strings.Contains(string(source), "internal/services/core/bootstrap/embedded/runtime") {
		t.Fatalf("Core embedded compatibility bridge must point to the aggregate runtime")
	}

	for _, path := range []string{"runtime.go", "rpc.go", "messaging.go"} {
		serviceSource, err := os.ReadFile(filepath.Join(path))
		if err != nil {
			t.Fatalf("read Core bootstrap source %s: %v", path, err)
		}
		if strings.Contains(string(serviceSource), "internal/bootstrap/embedded") {
			t.Fatalf("Core bootstrap file %s must not depend on embedded composition", path)
		}
	}
}
