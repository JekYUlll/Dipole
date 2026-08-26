package kafka

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultEventVersion        = "v1"
	SupportedEventMajorVersion = 1
)

var ErrUnsupportedEventVersion = errors.New("unsupported kafka event schema version")

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

func validateEnvelope(envelope *Envelope) error {
	if envelope == nil {
		return fmt.Errorf("kafka event envelope is missing")
	}
	if strings.TrimSpace(envelope.EventType) == "" {
		return fmt.Errorf("kafka event envelope event_type is empty")
	}

	major, err := eventMajorVersion(envelope.Version)
	if err != nil {
		return err
	}
	if major != SupportedEventMajorVersion {
		return fmt.Errorf("%w: %q", ErrUnsupportedEventVersion, envelope.Version)
	}
	if strings.TrimSpace(envelope.Version) == "" {
		envelope.Version = DefaultEventVersion
	}
	return nil
}

func eventMajorVersion(version string) (int, error) {
	version = strings.TrimSpace(strings.ToLower(version))
	if version == "" {
		return SupportedEventMajorVersion, nil
	}
	version = strings.TrimPrefix(version, "v")
	parts := strings.Split(version, ".")
	for index, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 || (index == 0 && value == 0) {
			return 0, fmt.Errorf("%w: %q", ErrUnsupportedEventVersion, version)
		}
	}
	return mustParseMajor(parts[0]), nil
}

func mustParseMajor(value string) int {
	major, _ := strconv.Atoi(value)
	return major
}

func generateEventID() string {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("generate kafka event id: %w", err))
	}

	return "E" + strings.ToUpper(hex.EncodeToString(buf))
}
