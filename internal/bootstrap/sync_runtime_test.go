package bootstrap

import (
	"context"
	"strings"
	"testing"

	"github.com/JekYUlll/Dipole/internal/config"
)

func TestInitializeSyncServiceRequiresInternalRPCBeforeOpeningDatabase(t *testing.T) {
	_, err := initializeSyncService(context.Background(), config.InternalRPC{}, config.MySQL{}, config.Metrics{}, config.Sync{}, config.Kafka{}, config.Cassandra{})
	if err == nil || !strings.Contains(err.Error(), "internal_rpc.enabled") {
		t.Fatalf("expected internal RPC validation, got %v", err)
	}
}

func TestValidateSyncHydrationConfig(t *testing.T) {
	if err := validateSyncHydrationConfig(config.Sync{}, config.Cassandra{}); err != nil {
		t.Fatalf("disabled shadow: %v", err)
	}
	if err := validateSyncHydrationConfig(config.Sync{CassandraShadowHydration: true}, config.Cassandra{}); err == nil || !strings.Contains(err.Error(), "cassandra.enabled") {
		t.Fatalf("missing Cassandra error=%v", err)
	}
	if err := validateSyncHydrationConfig(config.Sync{CassandraShadowHydration: true}, config.Cassandra{Enabled: true}); err != nil {
		t.Fatalf("enabled shadow: %v", err)
	}
}

func TestValidateSyncProjectorConfig(t *testing.T) {
	if err := validateSyncProjectorConfig(config.Sync{}, config.Kafka{}); err != nil {
		t.Fatalf("disabled projector should not require Kafka: %v", err)
	}
	if err := validateSyncProjectorConfig(config.Sync{ProjectorEnabled: true}, config.Kafka{}); err == nil || !strings.Contains(err.Error(), "kafka.enabled") {
		t.Fatalf("expected enabled projector to require Kafka, got %v", err)
	}
	if err := validateSyncProjectorConfig(config.Sync{ProjectorEnabled: true}, config.Kafka{Enabled: true}); err != nil {
		t.Fatalf("enabled projector with Kafka should pass: %v", err)
	}
}

func TestValidateMessageInboxWriteMode(t *testing.T) {
	for _, test := range []struct {
		name  string
		cfg   config.Message
		kafka config.Kafka
		want  string
	}{
		{name: "atomic owner", cfg: config.Message{RuntimeMode: "owner", InboxWriteMode: "atomic"}},
		{name: "projector owner", cfg: config.Message{RuntimeMode: "owner", InboxWriteMode: "projector"}, kafka: config.Kafka{Enabled: true}},
		{name: "projector shadow", cfg: config.Message{RuntimeMode: "shadow", InboxWriteMode: "projector"}, kafka: config.Kafka{Enabled: true}, want: "requires message.runtime_mode owner"},
		{name: "projector without Kafka", cfg: config.Message{RuntimeMode: "owner", InboxWriteMode: "projector"}, want: "requires kafka.enabled"},
		{name: "unknown", cfg: config.Message{RuntimeMode: "owner", InboxWriteMode: "dual"}, want: "unsupported message.inbox_write_mode"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateMessageInboxWriteMode(test.cfg, test.kafka)
			if test.want == "" && err != nil {
				t.Fatalf("validate mode: %v", err)
			}
			if test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestValidateTimelineNotifyMode(t *testing.T) {
	for _, test := range []struct {
		mode      string
		wantError bool
	}{
		{mode: "off"},
		{mode: "shadow"},
		{mode: "primary", wantError: true},
		{mode: "", wantError: true},
	} {
		err := validateTimelineNotifyMode(config.Message{TimelineNotifyMode: test.mode})
		if (err != nil) != test.wantError {
			t.Fatalf("mode=%q err=%v want_error=%t", test.mode, err, test.wantError)
		}
	}
}
