package ai

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
)

type baselineEvalSuite struct {
	SchemaVersion string             `json:"schema_version"`
	BaselineID    string             `json:"baseline_id"`
	Cases         []baselineEvalCase `json:"cases"`
}

type baselineEvalCase struct {
	ID        string           `json:"id"`
	Category  string           `json:"category"`
	Operation string           `json:"operation"`
	Input     map[string]any   `json:"input"`
	Expected  baselineExpected `json:"expected"`
}

type baselineExpected struct {
	Outcome    string   `json:"outcome"`
	Trajectory []string `json:"trajectory"`
	Permission string   `json:"permission"`
	KnownGap   string   `json:"known_gap,omitempty"`
}

func TestGoEinoBaselineContract(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "..", "contracts", "agent-evals", "v1", "go-eino-baseline.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Go/Eino baseline: %v", err)
	}

	var suite baselineEvalSuite
	if err := json.Unmarshal(data, &suite); err != nil {
		t.Fatalf("decode Go/Eino baseline: %v", err)
	}
	if suite.SchemaVersion != "dipole.agent.eval.v1" {
		t.Fatalf("unexpected eval schema version %q", suite.SchemaVersion)
	}
	if len(suite.Cases) == 0 {
		t.Fatal("Go/Eino baseline has no cases")
	}

	categories := map[string]bool{}
	caseIDs := map[string]bool{}
	for _, evalCase := range suite.Cases {
		evalCase := evalCase
		t.Run(evalCase.ID, func(t *testing.T) {
			if caseIDs[evalCase.ID] {
				t.Fatalf("duplicate eval case ID %q", evalCase.ID)
			}
			caseIDs[evalCase.ID] = true
			categories[evalCase.Category] = true

			actual := runBaselineEvalCase(t, evalCase)
			if !reflect.DeepEqual(actual, evalCase.Expected) {
				t.Fatalf("unexpected eval result\nwant: %+v\n got: %+v", evalCase.Expected, actual)
			}
		})
	}

	for _, category := range []string{"event", "reply", "trajectory", "permission"} {
		if !categories[category] {
			t.Errorf("baseline is missing %s cases", category)
		}
	}
}

func TestAgentEvalSchemaIsVersionedJSON(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "..", "contracts", "agent-evals", "v1", "schema.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Agent eval schema: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("decode Agent eval schema: %v", err)
	}
	if document["$id"] != "https://dipole.local/contracts/agent-evals/v1/schema.json" {
		t.Fatalf("unexpected Agent eval schema ID %q", document["$id"])
	}
}

type recordingEvalAgent struct {
	mode       string
	calls      int
	toolSender *stubAgentCapability
}

func (a *recordingEvalAgent) Reply(ctx context.Context, _ []*schema.Message) (*schema.Message, error) {
	a.calls++
	if a.mode == "tool" {
		tool := NewSystemMessageTool(a.toolSender, "UAI")
		if _, err := tool.(*systemMessageTool).InvokableRun(ctx, `{"user_uuid":"U100","content":"tool reply"}`); err != nil {
			return nil, err
		}
		return schema.AssistantMessage("", nil), nil
	}
	return schema.AssistantMessage("plain reply", nil), nil
}

func runBaselineEvalCase(t *testing.T, evalCase baselineEvalCase) baselineExpected {
	t.Helper()

	switch evalCase.Operation {
	case "handle_direct_message":
		return runHandleDirectMessageEval(t, evalCase)
	case "read_conversation":
		return runReadConversationEval(t, evalCase)
	case "get_user_profile":
		return runUserProfileEval(t, evalCase)
	case "send_system_message":
		return runSystemMessageEval(t, evalCase)
	default:
		t.Fatalf("unsupported baseline operation %q", evalCase.Operation)
		return baselineExpected{}
	}
}

func runHandleDirectMessageEval(t *testing.T, evalCase baselineEvalCase) baselineExpected {
	t.Helper()

	mode := evalString(t, evalCase.Input, "mode")
	logs := &stubCallLogRepository{beginReturn: mode != "duplicate"}
	fallbackCommands := &stubAgentCommands{message: &model.Message{UUID: "M-REPLY"}}
	toolSender := &stubAgentCapability{sentMessage: &model.Message{UUID: "M-TOOL", MessageType: model.MessageTypeSystem}}
	agent := &recordingEvalAgent{mode: mode, toolSender: toolSender}
	service := &Service{
		config: config.AI{Enabled: true, AssistantUUID: "UAI"},
		contextBuilder: &stubContextBuilder{context: &ConversationContext{
			EndUser:  &model.User{UUID: evalString(t, evalCase.Input, "sender_uuid")},
			Messages: []*schema.Message{schema.UserMessage("baseline")},
		}},
		logs:     logs,
		commands: fallbackCommands,
		agent:    agent,
	}

	err := service.HandleDirectMessage(context.Background(), &model.Message{
		UUID:            "M-TRIGGER",
		ConversationKey: model.DirectConversationKey(evalString(t, evalCase.Input, "sender_uuid"), "UAI"),
		SenderUUID:      evalString(t, evalCase.Input, "sender_uuid"),
		TargetType:      model.MessageTargetDirect,
		TargetUUID:      evalString(t, evalCase.Input, "target_uuid"),
		MessageType:     model.MessageTypeText,
		Content:         "baseline",
	})
	if err != nil {
		t.Fatalf("handle baseline direct message: %v", err)
	}

	if logs.beginLog == nil {
		return baselineExpected{Outcome: "ignored", Trajectory: []string{"trigger.ignored"}, Permission: "not_applicable"}
	}
	if agent.calls == 0 {
		return baselineExpected{Outcome: "deduplicated", Trajectory: []string{"trigger.deduplicated"}, Permission: "principal_targeted"}
	}
	if mode == "tool" {
		if fallbackCommands.command.Content != "" || toolSender.targetUUID != "U100" {
			t.Fatalf("tool reply did not suppress fallback or target principal: fallback=%q target=%q", fallbackCommands.command.Content, toolSender.targetUUID)
		}
		return baselineExpected{
			Outcome: "succeeded", Trajectory: []string{"trigger.accepted", "agent.reply", "tool.send_system_message", "run.succeeded"},
			Permission: "model_argument_matches_principal",
		}
	}
	if fallbackCommands.command.Invocation.PrincipalUUID != evalString(t, evalCase.Input, "sender_uuid") || len(logs.successArgs) == 0 {
		t.Fatal("plain reply did not target the triggering principal or complete its call log")
	}
	return baselineExpected{
		Outcome: "succeeded", Trajectory: []string{"trigger.accepted", "agent.reply", "message.send_assistant", "run.succeeded"},
		Permission: "principal_targeted",
	}
}

func runReadConversationEval(t *testing.T, evalCase baselineEvalCase) baselineExpected {
	t.Helper()

	userUUID := evalString(t, evalCase.Input, "user_uuid")
	targetUUID := evalString(t, evalCase.Input, "target_uuid")
	allowed := evalBool(t, evalCase.Input, "allowed")
	capability := &stubAgentCapability{}
	if allowed {
		capability.read = &application.AgentConversationReadV1{
			Found: true, TargetUUID: targetUUID, TargetType: model.MessageTargetDirect,
			Messages: []*model.Message{{UUID: "M1", SenderUUID: "U100", MessageType: model.MessageTypeText, Content: "evidence"}},
		}
	} else {
		capability.read = &application.AgentConversationReadV1{Found: false}
	}
	tool := NewReadConversationTool(capability)
	arguments, _ := json.Marshal(map[string]any{"target_uuid": targetUUID})
	ctx := toolTestContext(userUUID)
	result, err := tool.(*readConversationTool).InvokableRun(ctx, string(arguments))
	if err != nil {
		t.Fatalf("run read_conversation baseline: %v", err)
	}
	var payload toolReadConversationResult
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("decode read_conversation baseline result: %v", err)
	}
	if allowed {
		if !payload.Found || len(payload.Messages) != 1 {
			t.Fatalf("allowed conversation was not read: payload=%+v", payload)
		}
		return baselineExpected{Outcome: "found", Trajectory: []string{"tool.read_conversation", "conversation.authorized", "messages.read"}, Permission: "allowed"}
	}
	if payload.Found || len(payload.Messages) != 0 {
		t.Fatalf("denied conversation returned messages: payload=%+v", payload)
	}
	return baselineExpected{Outcome: "not_found", Trajectory: []string{"tool.read_conversation", "conversation.denied"}, Permission: "denied"}
}

func runUserProfileEval(t *testing.T, evalCase baselineEvalCase) baselineExpected {
	t.Helper()

	principal := evalString(t, evalCase.Input, "principal_uuid")
	capability := &stubAgentCapability{users: map[string]*model.User{principal: {UUID: principal, Nickname: "baseline user"}}}
	tool := NewUserProfileTool(capability, "UAI")
	argumentUser := evalString(t, evalCase.Input, "argument_user_uuid")
	ctx := toolTestContext(principal)
	result, err := tool.(*userProfileTool).InvokableRun(ctx, `{"user_uuid":"`+argumentUser+`"}`)
	if err != nil {
		t.Fatalf("run get_user_profile baseline: %v", err)
	}
	if capability.profileRequested != principal || capability.profileRequested == argumentUser || result == "" {
		t.Fatalf("execution principal was not enforced, requested=%q", capability.profileRequested)
	}
	return baselineExpected{
		Outcome: "found", Trajectory: []string{"tool.get_user_profile", "identity.execution_context", "user.read"},
		Permission: "principal_enforced",
	}
}

func runSystemMessageEval(t *testing.T, evalCase baselineEvalCase) baselineExpected {
	t.Helper()

	capability := &stubAgentCapability{sentMessage: &model.Message{UUID: "M-SYSTEM", MessageType: model.MessageTypeSystem}}
	tool := NewSystemMessageTool(capability, "UAI")
	principal := evalString(t, evalCase.Input, "principal_uuid")
	argumentUser := evalString(t, evalCase.Input, "argument_user_uuid")
	ctx := toolTestContext(principal)
	_, err := tool.(*systemMessageTool).InvokableRun(ctx, `{"user_uuid":"`+argumentUser+`","content":"baseline"}`)
	if err != nil {
		t.Fatalf("run send_system_message baseline: %v", err)
	}
	if capability.targetUUID != principal || capability.targetUUID == argumentUser {
		t.Fatalf("execution principal was not enforced as message target, target=%q", capability.targetUUID)
	}
	return baselineExpected{
		Outcome: "sent", Trajectory: []string{"tool.send_system_message", "identity.execution_context", "message.send_system"},
		Permission: "principal_enforced",
	}
}

func evalString(t *testing.T, input map[string]any, key string) string {
	t.Helper()
	value, ok := input[key].(string)
	if !ok || value == "" {
		t.Fatalf("baseline input %q must be a non-empty string", key)
	}
	return value
}

func evalBool(t *testing.T, input map[string]any, key string) bool {
	t.Helper()
	value, ok := input[key].(bool)
	if !ok {
		t.Fatalf("baseline input %q must be a boolean", key)
	}
	return value
}
