package agentchat

import (
	"strings"
	"testing"

	"github.com/JekYUlll/Dipole/internal/config"
)

// NewDirectReplyService must resolve the runtime mode before touching
// collaborators: off/remote compose to (nil, nil) so the microservices Core
// wiring registers no Kafka handler and the legacy chatbot stays fully inert,
// while embedded/shadow require the assistant's collaborators and fail fast when
// they are missing. This is the guard that keeps Route A dormant in the current
// remote experience env until it is explicitly flipped to embedded/shadow.
func TestNewDirectReplyServiceHonorsRuntimeModeBeforeDependencies(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{config.AIRuntimeOff, config.AIRuntimeRemote} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			service, err := NewDirectReplyService(config.AI{Enabled: true, RuntimeMode: mode}, nil, nil, nil, nil)
			if err != nil || service != nil {
				t.Fatalf("mode %s should skip embedded dependencies: service!=nil=%v err=%v", mode, service != nil, err)
			}
		})
	}

	for _, mode := range []string{config.AIRuntimeEmbedded, config.AIRuntimeShadow} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			_, err := NewDirectReplyService(config.AI{Enabled: true, RuntimeMode: mode}, nil, nil, nil, nil)
			if err == nil || !strings.Contains(err.Error(), "Capability") {
				t.Fatalf("mode %s should require embedded dependencies, got %v", mode, err)
			}
		})
	}

	if _, err := NewDirectReplyService(config.AI{Enabled: true, RuntimeMode: "dual"}, nil, nil, nil, nil); err == nil || !strings.Contains(err.Error(), "runtime mode") {
		t.Fatalf("invalid mode should fail fast, got %v", err)
	}
}

// An empty runtime mode with AI disabled resolves to off and stays inert.
func TestNewDirectReplyServiceDisabledResolvesOff(t *testing.T) {
	if svc, err := NewDirectReplyService(config.AI{Enabled: false}, nil, nil, nil, nil); err != nil || svc != nil {
		t.Fatalf("disabled: expected (nil,nil), got service!=nil=%v err=%v", svc != nil, err)
	}
}
