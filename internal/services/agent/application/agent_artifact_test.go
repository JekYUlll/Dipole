package agentapplication

import (
	"testing"

	"github.com/JekYUlll/Dipole/internal/application"
)

func TestActiveReportArtifactPolicy(t *testing.T) {
	task := &application.AgentTaskV1{}
	run := &application.AgentRunV1{RuntimeID: "dipole-agent", Mode: "active", Status: application.AgentRunStatusRunning}
	artifact := &application.AgentArtifactV1{ArtifactType: "conversation_digest", MediaType: "text/markdown"}
	if !agentArtifactCreateAllowedV1(task, run, artifact, nil) {
		t.Fatal("active report draft should be allowed")
	}
	artifact.ArtifactType = "promotion_evaluation"
	if agentArtifactCreateAllowedV1(task, run, artifact, nil) {
		t.Fatal("active report cannot publish promotion artifacts")
	}
	artifact.ArtifactType = "conversation_digest"
	run.Status = application.AgentRunStatusCompleted
	if agentArtifactCreateAllowedV1(task, run, artifact, nil) {
		t.Fatal("completed run cannot create a new report version")
	}
	run.Status = application.AgentRunStatusRunning
	run.RuntimeID = "untrusted"
	if agentArtifactCreateAllowedV1(task, run, artifact, nil) {
		t.Fatal("untrusted runtime cannot create report artifacts")
	}
}
