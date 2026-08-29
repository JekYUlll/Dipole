package mapper

import (
	"reflect"
	"testing"

	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/model"
)

func TestAICallLogInsertParams(t *testing.T) {
	t.Parallel()

	log := &model.AICallLog{
		TriggerMessageUUID:  "M-trigger",
		ResponseMessageUUID: "M-response",
		ConversationKey:     "direct:U100:U200",
		UserUUID:            "U100",
		AssistantUUID:       "UAI",
		Provider:            "provider",
		Model:               "model",
		Status:              model.AICallStatusSucceeded,
		ErrorMessage:        "",
		PromptTokens:        10,
		CompletionTokens:    20,
		TotalTokens:         30,
		LatencyMS:           40,
	}

	got := AICallLogInsertParams(log)
	want := generated.InsertAICallLogParams{
		TriggerMessageUuid:  "M-trigger",
		ResponseMessageUuid: "M-response",
		ConversationKey:     "direct:U100:U200",
		UserUuid:            "U100",
		AssistantUuid:       "UAI",
		Provider:            "provider",
		Model:               "model",
		Status:              model.AICallStatusSucceeded,
		PromptTokens:        10,
		CompletionTokens:    20,
		TotalTokens:         30,
		LatencyMs:           40,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected params:\ngot:  %+v\nwant: %+v", got, want)
	}
}

func TestAICallLogInsertParamsHandlesNil(t *testing.T) {
	t.Parallel()

	if got := AICallLogInsertParams(nil); got != (generated.InsertAICallLogParams{}) {
		t.Fatalf("expected zero params, got %+v", got)
	}
}
