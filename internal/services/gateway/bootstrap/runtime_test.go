package bootstrap

import "testing"

func TestAgentArtifactApplicationOmitsDisabledClient(t *testing.T) {
	if application := agentArtifactApplication(nil); application != nil {
		t.Fatalf("disabled Artifact client=%T, want nil", application)
	}
}
