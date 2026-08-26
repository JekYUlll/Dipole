package kafka

import (
	"errors"
	"testing"
)

func TestNewEnvelopeUsesCurrentSchemaVersion(t *testing.T) {
	t.Parallel()
	envelope, err := NewEnvelope("message.created", map[string]string{"message_id": "M1"})
	if err != nil {
		t.Fatalf("new envelope: %v", err)
	}
	if envelope.Version != DefaultEventVersion || envelope.EventType != "message.created" {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
}

func TestDecodeEnvelopeAcceptsLegacyAndCurrentMajorVersions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		versionJSON string
		wantVersion string
	}{
		{name: "legacy missing version", versionJSON: "", wantVersion: DefaultEventVersion},
		{name: "current major", versionJSON: `,"version":"v1"`, wantVersion: "v1"},
		{name: "compatible minor", versionJSON: `,"version":"v1.7"`, wantVersion: "v1.7"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := []byte(`{"event_type":"message.created"` + test.versionJSON + `,"payload":{}}`)
			envelope, err := DecodeEnvelope(raw)
			if err != nil {
				t.Fatalf("decode envelope: %v", err)
			}
			if envelope.Version != test.wantVersion {
				t.Fatalf("expected version %q, got %q", test.wantVersion, envelope.Version)
			}
		})
	}
}

func TestDecodeEnvelopeRejectsFutureMajorVersion(t *testing.T) {
	t.Parallel()
	_, err := DecodeEnvelope([]byte(`{"event_type":"message.created","version":"v2","payload":{}}`))
	if !errors.Is(err, ErrUnsupportedEventVersion) {
		t.Fatalf("expected unsupported version, got %v", err)
	}
}

func TestDecodeEnvelopeRejectsMalformedVersion(t *testing.T) {
	t.Parallel()
	for _, version := range []string{"banana", "v1.beta", "v1."} {
		_, err := DecodeEnvelope([]byte(`{"event_type":"message.created","version":"` + version + `","payload":{}}`))
		if !errors.Is(err, ErrUnsupportedEventVersion) {
			t.Fatalf("version %q: expected unsupported version, got %v", version, err)
		}
	}
}
