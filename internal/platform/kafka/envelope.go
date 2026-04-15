package kafka

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const DefaultEventVersion = "v1"

type Envelope struct {
	EventID    string          `json:"event_id"`
	EventType  string          `json:"event_type"`
	Version    string          `json:"version"`
	Source     string          `json:"source"`
	OccurredAt time.Time       `json:"occurred_at"`
	Payload    json.RawMessage `json:"payload"`
}

func NewEnvelope(eventType string, payload any) (*Envelope, error) {
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal event payload: %w", err)
	}

	return &Envelope{
		EventID:    generateEventID(),
		EventType:  strings.TrimSpace(eventType),
		Version:    DefaultEventVersion,
		Source:     "dipole",
		OccurredAt: time.Now().UTC(),
		Payload:    rawPayload,
	}, nil
}

func generateEventID() string {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("generate kafka event id: %w", err))
	}

	return "E" + strings.ToUpper(hex.EncodeToString(buf))
}
