package eventlineage

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

type OriginType string

const (
	OriginAgent   OriginType = "agent"
	OriginService OriginType = "service"
	OriginSystem  OriginType = "system"
)

const maxIdentifierLength = 128

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]*$`)

type Origin struct {
	Type OriginType `json:"type"`
	ID   string     `json:"id"`
}

type Lineage struct {
	Origin           Origin `json:"origin"`
	CausationEventID string `json:"causation_event_id,omitempty"`
	AgentTaskID      string `json:"agent_task_id,omitempty"`
}

type contextKey struct{}

func WithContext(ctx context.Context, lineage Lineage) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, contextKey{}, normalize(lineage))
}

func FromContext(ctx context.Context) Lineage {
	if ctx == nil {
		return Lineage{}
	}
	lineage, _ := ctx.Value(contextKey{}).(Lineage)
	return normalize(lineage)
}

func Advance(lineage Lineage, causationEventID string) Lineage {
	lineage = normalize(lineage)
	if lineage == (Lineage{}) {
		return lineage
	}
	lineage.CausationEventID = strings.TrimSpace(causationEventID)
	return lineage
}

func AgentAction(ctx context.Context, agentID, taskID, causationEventID string) context.Context {
	lineage := FromContext(ctx)
	if lineage.Origin.Type != OriginAgent {
		lineage.Origin = Origin{Type: OriginAgent, ID: strings.TrimSpace(agentID)}
		lineage.AgentTaskID = strings.TrimSpace(taskID)
	}
	lineage.CausationEventID = strings.TrimSpace(causationEventID)
	return WithContext(ctx, lineage)
}

func Validate(lineage Lineage) error {
	lineage = normalize(lineage)
	if lineage == (Lineage{}) {
		return nil
	}
	if lineage.Origin.Type != OriginAgent && lineage.Origin.Type != OriginService && lineage.Origin.Type != OriginSystem {
		return fmt.Errorf("event lineage origin type %q is unsupported", lineage.Origin.Type)
	}
	if err := validateIdentifier("origin id", lineage.Origin.ID); err != nil {
		return err
	}
	if lineage.CausationEventID != "" {
		if err := validateIdentifier("causation event id", lineage.CausationEventID); err != nil {
			return err
		}
	}
	if lineage.AgentTaskID != "" {
		if err := validateIdentifier("Agent task id", lineage.AgentTaskID); err != nil {
			return err
		}
	}
	if lineage.Origin.Type == OriginAgent && lineage.AgentTaskID == "" {
		return fmt.Errorf("event lineage agent_task_id is required for Agent origin")
	}
	return nil
}

func normalize(lineage Lineage) Lineage {
	lineage.Origin.Type = OriginType(strings.TrimSpace(string(lineage.Origin.Type)))
	lineage.Origin.ID = strings.TrimSpace(lineage.Origin.ID)
	lineage.CausationEventID = strings.TrimSpace(lineage.CausationEventID)
	lineage.AgentTaskID = strings.TrimSpace(lineage.AgentTaskID)
	return lineage
}

func validateIdentifier(name, value string) error {
	if value == "" || len(value) > maxIdentifierLength || !identifierPattern.MatchString(value) {
		return fmt.Errorf("event lineage %s is invalid", name)
	}
	return nil
}
