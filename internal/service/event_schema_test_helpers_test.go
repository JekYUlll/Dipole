package service

import (
	"encoding/json"
	"testing"
)

type messageEventSchema struct {
	Required   []string                      `json:"required"`
	Properties map[string]json.RawMessage    `json:"properties"`
	Defs       map[string]messageEventSchema `json:"$defs"`
}

func schemaPattern(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var property struct {
		Pattern string `json:"pattern"`
	}
	if err := json.Unmarshal(raw, &property); err != nil {
		t.Fatalf("decode schema pattern: %v", err)
	}
	return property.Pattern
}

func assertRequiredFields(t *testing.T, scope string, required []string, value map[string]any) {
	t.Helper()
	for _, field := range required {
		if _, ok := value[field]; !ok {
			t.Errorf("%s producer omitted schema-required field %q", scope, field)
		}
	}
}
