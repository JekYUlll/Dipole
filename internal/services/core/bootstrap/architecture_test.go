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

func TestCoreRuntimeKeepsOAuthCallbackHandoffExplicitAndMTLSBound(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("runtime.go"))
	if err != nil {
		t.Fatalf("read Core runtime: %v", err)
	}
	text := string(source)
	for _, requirement := range []string{
		"rpcCfg.AgentOAuthCallbackHandoffEnabled",
		"Agent OAuth callback handoff requires internal RPC mTLS",
		"WithOAuthCallbackHandoffs",
		"NewOAuthCallbackHandoffServer",
	} {
		if !strings.Contains(text, requirement) {
			t.Fatalf("Core OAuth callback handoff composition must include %q", requirement)
		}
	}
}

func TestCoreRuntimeKeepsWorkflowRepairExecuteExplicitAndMTLSBound(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("runtime.go"))
	if err != nil {
		t.Fatalf("read Core runtime: %v", err)
	}
	text := string(source)
	if !strings.Contains(text, "rpcCfg.AgentWorkflowRepairExecuteEnabled") {
		t.Fatal("Core Workflow repair executor injection must require an explicit gate")
	}
	if !strings.Contains(text, "Agent Workflow repair execute requires internal RPC mTLS") {
		t.Fatal("Core Workflow repair executor injection must require mTLS")
	}
	if !strings.Contains(text, "WithWorkflowRepairExecutor") {
		t.Fatal("Core must inject the Workflow repair executor into the Agent adapter")
	}
}

func TestCoreRuntimeComposesAgentDefinitionAndSubscriptionControlPlane(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("runtime.go"))
	if err != nil {
		t.Fatalf("read Core runtime: %v", err)
	}
	text := string(source)
	for _, requirement := range []string{
		"NewPersistentAgentEventSubscriptionResolverV1",
		"NewPersistentAgentEventSubscriptionControlV1",
		"NewPersistentAgentDefinitionCatalogV1",
		"agentServer.WithEventSubscriptions(subscriptionResolver)",
		"agentServer.WithEventSubscriptionControls(subscriptionControls)",
		"agentServer.WithDefinitionCatalog(definitionCatalog)",
	} {
		if !strings.Contains(text, requirement) {
			t.Fatalf("standalone Core Agent control plane must compose %q", requirement)
		}
	}
}

func TestCoreRuntimeWaitsForConversationProjectionKafkaAssignment(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("runtime.go"))
	if err != nil {
		t.Fatalf("read Core runtime: %v", err)
	}
	text := string(source)
	for _, requirement := range []string{
		"append(corekafka.ConversationProjectionTopics(), applicationPort.AgentTaskWaitingEventTypeV1)",
		"EnsureTopics(topics)",
		"KafkaConsumerReadinessProbe(\"kafka-assignment\", platformKafka.Subscriber)",
	} {
		if !strings.Contains(text, requirement) {
			t.Fatalf("standalone Core projection startup must include %q", requirement)
		}
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
