package bootstrap

import "testing"

func TestAgentArtifactApplicationOmitsDisabledClient(t *testing.T) {
	if application := agentArtifactApplication(nil); application != nil {
		t.Fatalf("disabled Artifact client=%T, want nil", application)
	}
}

func TestAgentControlSecretPrefersDedicatedConfiguration(t *testing.T) {
	if got := agentControlSecret("control-secret", "rpc-secret"); got != "control-secret" {
		t.Fatalf("agent control secret=%q, want dedicated secret", got)
	}
}

func TestAgentControlSecretFallsBackToInternalRPCSecret(t *testing.T) {
	if got := agentControlSecret("", "rpc-secret"); got != "rpc-secret" {
		t.Fatalf("agent control secret=%q, want internal rpc fallback", got)
	}
}
