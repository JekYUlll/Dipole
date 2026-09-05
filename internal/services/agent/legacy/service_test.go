package ai

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/correlation"
	"github.com/JekYUlll/Dipole/internal/platform/eventlineage"
)

type stubContextBuilder struct {
	context *ConversationContext
	err     error
}

func (s *stubContextBuilder) BuildDirectContext(context.Context, string, string) (*ConversationContext, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.context, nil
}

func (s *stubContextBuilder) BuildGroupContext(context.Context, string, string, string) (*ConversationContext, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.context, nil
}

type stubGroupMessenger struct {
	senderUUID string
	groupUUID  string
	content    string
	err        error
	calls      int
}

func (s *stubGroupMessenger) SendGroupMessage(senderUUID, groupUUID, content, _ string) (*model.Message, []string, error) {
	s.calls++
	s.senderUUID = senderUUID
	s.groupUUID = groupUUID
	s.content = content
	if s.err != nil {
		return nil, nil, s.err
	}
	return &model.Message{UUID: "MG-REPLY"}, nil, nil
}

type stubCallLogRepository struct {
	beginReturn bool
	beginLog    *model.AICallLog
	successArgs []any
	failedArgs  []any
}

func (s *stubCallLogRepository) Begin(log *model.AICallLog) (bool, error) {
	s.beginLog = log
	return s.beginReturn, nil
}

func (s *stubCallLogRepository) MarkSucceeded(triggerMessageUUID, responseMessageUUID string, promptTokens, completionTokens, totalTokens int, latencyMS int64) error {
	s.successArgs = []any{triggerMessageUUID, responseMessageUUID, promptTokens, completionTokens, totalTokens}
	return nil
}

func (s *stubCallLogRepository) MarkFailed(triggerMessageUUID, errorMessage string, latencyMS int64) error {
	s.failedArgs = []any{triggerMessageUUID, errorMessage}
	return nil
}

type stubAgentCommands struct {
	command application.AgentMessageCommandV1
	ids     correlation.IDs
	lineage eventlineage.Lineage
	message *model.Message
	err     error
}

func (s *stubAgentCommands) SendMessage(ctx context.Context, command application.AgentMessageCommandV1) (*model.Message, error) {
	s.command = command
	s.ids = correlation.FromContext(ctx)
	s.lineage = eventlineage.FromContext(ctx)
	if s.err != nil {
		return nil, s.err
	}

	return s.message, nil
}

type stubAgent struct {
	reply *schema.Message
	err   error
	runFn func(ctx context.Context, messages []*schema.Message) (*schema.Message, error)
	calls int
}

type stubExecutionPolicy struct {
	startErr      error
	completeErr   error
	starts        int
	completions   int
	failures      int
	lastExecution application.AgentPolicyExecutionV1
}

func (s *stubExecutionPolicy) Start(_ context.Context, request application.AgentExecutionPolicyStartV1) (*application.AgentPolicyExecutionV1, error) {
	s.starts++
	if s.startErr != nil {
		return nil, s.startErr
	}
	s.lastExecution = application.AgentPolicyExecutionV1{
		TaskUUID: "TASK-1",
		Invocation: application.AgentInvocationV1{
			TenantID: request.TenantID, PrincipalUUID: request.PrincipalUUID, AgentUUID: request.AgentUUID,
			DelegatedByUUID: request.DelegatedByUUID, Permissions: embeddedAgentPermissionsV1(), ResourceScopes: embeddedAgentResourceScopesV1(),
			RequestID: request.RequestID, TraceID: request.TraceID, EventID: request.EventID,
		},
	}
	return &s.lastExecution, nil
}

func (s *stubExecutionPolicy) Complete(_ context.Context, execution application.AgentPolicyExecutionV1) error {
	s.completions++
	s.lastExecution = execution
	return s.completeErr
}

func (s *stubExecutionPolicy) Fail(_ context.Context, execution application.AgentPolicyExecutionV1) error {
	s.failures++
	s.lastExecution = execution
	return nil
}

func (s *stubAgent) Reply(ctx context.Context, messages []*schema.Message) (*schema.Message, error) {
	s.calls++
	if s.runFn != nil {
		return s.runFn(ctx, messages)
	}
	if s.err != nil {
		return nil, s.err
	}

	return s.reply, nil
}

func TestServiceHandleDirectMessageSuccess(t *testing.T) {
	t.Parallel()

	logs := &stubCallLogRepository{beginReturn: true}
	commands := &stubAgentCommands{
		message: &model.Message{UUID: "MAI200"},
	}
	policy := &stubExecutionPolicy{}
	service := &Service{
		config: config.AI{
			Enabled:       true,
			Provider:      "openai",
			Model:         "gpt-test",
			AssistantUUID: "UAI",
		},
		contextBuilder: &stubContextBuilder{
			context: &ConversationContext{
				Messages: []*schema.Message{schema.UserMessage("hello")},
			},
		},
		logs:     logs,
		commands: commands,
		policy:   policy,
		agent: &stubAgent{
			reply: &schema.Message{
				Role:    schema.Assistant,
				Content: "ai response",
				ResponseMeta: &schema.ResponseMeta{
					Usage: &schema.TokenUsage{
						PromptTokens:     10,
						CompletionTokens: 5,
						TotalTokens:      15,
					},
				},
			},
		},
	}

	ctx := correlation.WithContext(context.Background(), correlation.IDs{EventID: "E-TRIGGER"})
	err := service.HandleDirectMessage(ctx, &model.Message{
		UUID:            "M100",
		ConversationKey: model.DirectConversationKey("U100", "UAI"),
		SenderUUID:      "U100",
		TargetType:      model.MessageTargetDirect,
		TargetUUID:      "UAI",
		MessageType:     model.MessageTypeText,
		Content:         "hello",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if logs.beginLog == nil || logs.beginLog.TriggerMessageUUID != "M100" {
		t.Fatalf("expected ai call log begin with trigger M100, got %+v", logs.beginLog)
	}
	if commands.command.Invocation.AgentUUID != "UAI" || commands.command.Invocation.PrincipalUUID != "U100" {
		t.Fatalf("unexpected command identity: %+v", commands.command)
	}
	if commands.command.CommandID != "reply:M100" || commands.command.Kind != application.AgentMessageCommandAssistantReplyV1 || commands.command.Content != "ai response" {
		t.Fatalf("unexpected reply command: %+v", commands.command)
	}
	if commands.lineage.Origin != (eventlineage.Origin{Type: eventlineage.OriginAgent, ID: "UAI"}) || commands.lineage.AgentTaskID != "TASK-1" || commands.lineage.CausationEventID == "" {
		t.Fatalf("unexpected Agent command lineage: %+v", commands.lineage)
	}
	if len(logs.successArgs) == 0 {
		t.Fatalf("expected ai call success log to be recorded")
	}
	if policy.starts != 1 || policy.completions != 1 || policy.failures != 0 {
		t.Fatalf("unexpected success policy lifecycle: %+v", policy)
	}
}

func TestServiceDerivesTrustedExecutionContextFromTrigger(t *testing.T) {
	t.Parallel()

	var captured ExecutionContext
	service := &Service{
		config: config.AI{Enabled: true, AssistantUUID: "UAI"},
		contextBuilder: &stubContextBuilder{context: &ConversationContext{
			Messages: []*schema.Message{schema.UserMessage("hello")},
		}},
		logs:     &stubCallLogRepository{beginReturn: true},
		commands: &stubAgentCommands{message: &model.Message{UUID: "M-REPLY"}},
		policy:   &stubExecutionPolicy{},
		agent: &stubAgent{runFn: func(ctx context.Context, messages []*schema.Message) (*schema.Message, error) {
			var err error
			captured, err = requireExecutionContext(ctx)
			if err != nil {
				return nil, err
			}
			if len(messages) != 1 || messages[0].Role != schema.User {
				t.Fatalf("execution identity leaked into model messages: %+v", messages)
			}
			return schema.AssistantMessage("reply", nil), nil
		}},
	}

	ctx := correlation.WithContext(context.Background(), correlation.IDs{
		RequestID: "REQ-1",
		TraceID:   "TRACE-1",
		EventID:   "EVENT-1",
	})
	message := &model.Message{
		UUID:            "M-TRIGGER",
		ConversationKey: model.DirectConversationKey("U100", "UAI"),
		SenderUUID:      "U100",
		TargetType:      model.MessageTargetDirect,
		TargetUUID:      "UAI",
		MessageType:     model.MessageTypeText,
		Content:         "hello",
	}
	if err := service.HandleDirectMessage(ctx, message); err != nil {
		t.Fatalf("handle direct message: %v", err)
	}

	want := ExecutionContext{
		TenantID:           defaultAgentTenantID,
		PrincipalUserUUID:  "U100",
		AgentUUID:          "UAI",
		DelegatedByUUID:    "U100",
		TriggerMessageUUID: "M-TRIGGER",
		ConversationKey:    message.ConversationKey,
		RequestID:          "REQ-1",
		TraceID:            "TRACE-1",
		EventID:            "EVENT-1",
	}
	want = newExecutionContext(want, embeddedAgentPermissionsV1(), nil, embeddedAgentResourceScopesV1())
	if !reflect.DeepEqual(captured, want) {
		t.Fatalf("execution context = %+v, want %+v", captured, want)
	}
}

func TestServiceHandleDirectMessageSkipsNonAssistantTarget(t *testing.T) {
	t.Parallel()

	logs := &stubCallLogRepository{beginReturn: true}
	service := &Service{
		config: config.AI{
			Enabled:       true,
			AssistantUUID: "UAI",
		},
		contextBuilder: &stubContextBuilder{},
		logs:           logs,
		commands:       &stubAgentCommands{},
		policy:         &stubExecutionPolicy{},
		agent:          &stubAgent{},
	}

	if err := service.HandleDirectMessage(context.Background(), &model.Message{
		UUID:        "M100",
		SenderUUID:  "U100",
		TargetType:  model.MessageTargetDirect,
		TargetUUID:  "U200",
		MessageType: model.MessageTypeText,
		Content:     "hello",
	}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if logs.beginLog != nil {
		t.Fatalf("expected ai pipeline to skip non-assistant message")
	}
}

func TestServiceHandleDirectMessageMarksFailure(t *testing.T) {
	t.Parallel()

	logs := &stubCallLogRepository{beginReturn: true}
	policy := &stubExecutionPolicy{}
	service := &Service{
		config: config.AI{
			Enabled:       true,
			AssistantUUID: "UAI",
		},
		contextBuilder: &stubContextBuilder{
			context: &ConversationContext{
				Messages: []*schema.Message{schema.UserMessage("hello")},
			},
		},
		logs:     logs,
		commands: &stubAgentCommands{},
		policy:   policy,
		agent: &stubAgent{
			err: errors.New("llm timeout"),
		},
	}

	err := service.HandleDirectMessage(context.Background(), &model.Message{
		UUID:            "M100",
		ConversationKey: model.DirectConversationKey("U100", "UAI"),
		SenderUUID:      "U100",
		TargetType:      model.MessageTargetDirect,
		TargetUUID:      "UAI",
		MessageType:     model.MessageTypeText,
		Content:         "hello",
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if len(logs.failedArgs) == 0 {
		t.Fatalf("expected ai call failure log to be recorded")
	}
	if policy.starts != 1 || policy.completions != 0 || policy.failures != 1 {
		t.Fatalf("unexpected failure policy lifecycle: %+v", policy)
	}
}

func TestServiceHandleDirectMessageFailsClosedWhenPolicyStartIsDenied(t *testing.T) {
	t.Parallel()

	policy := &stubExecutionPolicy{startErr: application.ErrAgentExecutionPolicyDenied}
	logs := &stubCallLogRepository{beginReturn: true}
	agent := &stubAgent{reply: schema.AssistantMessage("must not run", nil)}
	service := &Service{
		config:         config.AI{Enabled: true, AssistantUUID: "UAI"},
		contextBuilder: &stubContextBuilder{context: &ConversationContext{Messages: []*schema.Message{schema.UserMessage("hello")}}},
		logs:           logs, commands: &stubAgentCommands{message: &model.Message{UUID: "M-REPLY"}}, policy: policy, agent: agent,
	}
	err := service.HandleDirectMessage(context.Background(), &model.Message{
		UUID: "M-POLICY-DENIED", ConversationKey: model.DirectConversationKey("U100", "UAI"), SenderUUID: "U100",
		TargetType: model.MessageTargetDirect, TargetUUID: "UAI", MessageType: model.MessageTypeText, Content: "hello",
	})
	if !errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
		t.Fatalf("expected policy denial, got %v", err)
	}
	if agent.calls != 0 || policy.starts != 1 || policy.completions != 0 || policy.failures != 0 || len(logs.failedArgs) == 0 {
		t.Fatalf("policy denial did not fail closed: policy=%+v logs=%+v", policy, logs.failedArgs)
	}
}

func TestServiceHandleDirectMessageUsesToolSentMessage(t *testing.T) {
	t.Parallel()

	logs := &stubCallLogRepository{beginReturn: true}
	toolMessage := &model.Message{
		UUID:        "MSYS100",
		MessageType: model.MessageTypeSystem,
	}
	commands := &stubAgentCommands{}
	service := &Service{
		config: config.AI{
			Enabled:       true,
			Provider:      "openai",
			Model:         "gpt-test",
			AssistantUUID: "UAI",
		},
		contextBuilder: &stubContextBuilder{
			context: &ConversationContext{
				Messages: []*schema.Message{schema.UserMessage("send a notice")},
			},
		},
		logs:     logs,
		commands: commands,
		policy:   &stubExecutionPolicy{},
		agent: &stubAgent{
			runFn: func(ctx context.Context, messages []*schema.Message) (*schema.Message, error) {
				recordToolSentMessage(ctx, toolMessage)
				return &schema.Message{
					Role:    schema.Assistant,
					Content: "",
				}, nil
			},
		},
	}

	err := service.HandleDirectMessage(context.Background(), &model.Message{
		UUID:            "M100",
		ConversationKey: model.DirectConversationKey("U100", "UAI"),
		SenderUUID:      "U100",
		TargetType:      model.MessageTargetDirect,
		TargetUUID:      "UAI",
		MessageType:     model.MessageTypeText,
		Content:         "send a notice",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if commands.command.Content != "" {
		t.Fatalf("expected no fallback assistant text command, got %q", commands.command.Content)
	}
	if len(logs.successArgs) == 0 {
		t.Fatalf("expected ai call success log to be recorded")
	}
	if logs.successArgs[1] != "MSYS100" {
		t.Fatalf("expected tool-sent message uuid to be recorded, got %+v", logs.successArgs)
	}
}

func newGroupReplyService(logs *stubCallLogRepository, groups *stubGroupMessenger, policy *stubExecutionPolicy, agent Agent) *Service {
	return &Service{
		config: config.AI{
			Enabled:           true,
			Provider:          "openai",
			Model:             "gpt-test",
			AssistantUUID:     "UAI",
			AssistantNickname: "Dipole AI",
		},
		contextBuilder: &stubContextBuilder{
			context: &ConversationContext{Messages: []*schema.Message{schema.UserMessage("@Dipole AI hello")}},
		},
		logs:          logs,
		commands:      &stubAgentCommands{},
		policy:        policy,
		agent:         agent,
		groupMessages: groups,
	}
}

func TestServiceHandleGroupMessageSuccess(t *testing.T) {
	t.Parallel()

	logs := &stubCallLogRepository{beginReturn: true}
	groups := &stubGroupMessenger{}
	policy := &stubExecutionPolicy{}
	service := newGroupReplyService(logs, groups, policy, &stubAgent{reply: schema.AssistantMessage("group reply", nil)})

	err := service.HandleGroupMessage(context.Background(), &model.Message{
		UUID:            "MG1",
		ConversationKey: model.GroupConversationKey("G100"),
		SenderUUID:      "U100",
		TargetType:      model.MessageTargetGroup,
		TargetUUID:      "G100",
		MessageType:     model.MessageTypeText,
		Content:         "@Dipole AI 总结一下",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if logs.beginLog == nil || logs.beginLog.TriggerMessageUUID != "MG1" {
		t.Fatalf("expected begin log for MG1, got %+v", logs.beginLog)
	}
	if groups.calls != 1 || groups.senderUUID != "UAI" || groups.groupUUID != "G100" || groups.content != "group reply" {
		t.Fatalf("unexpected group send: %+v", groups)
	}
	if policy.starts != 1 || policy.completions != 1 || policy.failures != 0 {
		t.Fatalf("unexpected policy lifecycle: %+v", policy)
	}
	if len(logs.successArgs) == 0 || logs.successArgs[1] != "MG-REPLY" {
		t.Fatalf("expected group-reply log, got %+v", logs.successArgs)
	}
}

func TestServiceHandleGroupMessageSkipsWithoutMention(t *testing.T) {
	t.Parallel()

	logs := &stubCallLogRepository{beginReturn: true}
	groups := &stubGroupMessenger{}
	agent := &stubAgent{reply: schema.AssistantMessage("must not run", nil)}
	service := newGroupReplyService(logs, groups, &stubExecutionPolicy{}, agent)

	if err := service.HandleGroupMessage(context.Background(), &model.Message{
		UUID: "MG2", SenderUUID: "U100", TargetType: model.MessageTargetGroup, TargetUUID: "G100",
		MessageType: model.MessageTypeText, Content: "今晚谁有空",
	}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if logs.beginLog != nil || groups.calls != 0 || agent.calls != 0 {
		t.Fatalf("non-mention must be a no-op")
	}
}

func TestServiceHandleGroupMessageIsIdempotentOnReplay(t *testing.T) {
	t.Parallel()

	logs := &stubCallLogRepository{beginReturn: false}
	groups := &stubGroupMessenger{}
	agent := &stubAgent{reply: schema.AssistantMessage("must not run", nil)}
	service := newGroupReplyService(logs, groups, &stubExecutionPolicy{}, agent)

	if err := service.HandleGroupMessage(context.Background(), &model.Message{
		UUID: "MG3", SenderUUID: "U100", TargetType: model.MessageTargetGroup, TargetUUID: "G100",
		MessageType: model.MessageTypeText, Content: "@Dipole AI 再问一次",
	}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if groups.calls != 0 || agent.calls != 0 {
		t.Fatalf("replay must not send again")
	}
}

func TestServiceHandleGroupMessageSkipsAssistantSender(t *testing.T) {
	t.Parallel()

	logs := &stubCallLogRepository{beginReturn: true}
	groups := &stubGroupMessenger{}
	service := newGroupReplyService(logs, groups, &stubExecutionPolicy{}, &stubAgent{})

	if err := service.HandleGroupMessage(context.Background(), &model.Message{
		UUID: "MG4", SenderUUID: "UAI", TargetType: model.MessageTargetGroup, TargetUUID: "G100",
		MessageType: model.MessageTypeText, Content: "@Dipole AI 自己说",
	}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if logs.beginLog != nil || groups.calls != 0 {
		t.Fatalf("assistant sender must be skipped")
	}
}
