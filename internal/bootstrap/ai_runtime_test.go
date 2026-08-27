package bootstrap

import (
	"strings"
	"testing"

	"github.com/JekYUlll/Dipole/internal/config"
)

func TestNewAIServiceHonorsRuntimeModeBeforeDependencies(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{config.AIRuntimeOff, config.AIRuntimeRemote} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			service, err := newAIService(config.AI{RuntimeMode: mode}, nil, nil, nil)
			if err != nil || service != nil {
				t.Fatalf("mode %s should skip Embedded dependencies: service=%v err=%v", mode, service, err)
			}
		})
	}

	for _, mode := range []string{config.AIRuntimeEmbedded, config.AIRuntimeShadow} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			_, err := newAIService(config.AI{RuntimeMode: mode}, nil, nil, nil)
			if err == nil || !strings.Contains(err.Error(), "Capability") {
				t.Fatalf("mode %s should require Embedded dependencies, got %v", mode, err)
			}
		})
	}

	if _, err := newAIService(config.AI{RuntimeMode: "dual"}, nil, nil, nil); err == nil || !strings.Contains(err.Error(), "runtime mode") {
		t.Fatalf("invalid mode should fail fast, got %v", err)
	}
}
