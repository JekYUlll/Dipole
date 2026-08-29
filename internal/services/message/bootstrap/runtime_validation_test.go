package bootstrap

import (
	"strings"
	"testing"

	"github.com/JekYUlll/Dipole/internal/config"
)

func TestValidateMessageInboxWriteMode(t *testing.T) {
	for _, test := range []struct {
		name  string
		cfg   config.Message
		kafka config.Kafka
		sync  config.Sync
		want  string
	}{
		{name: "atomic owner", cfg: config.Message{RuntimeMode: "owner", InboxWriteMode: "atomic"}},
		{name: "projector owner", cfg: config.Message{RuntimeMode: "owner", InboxWriteMode: "projector"}, kafka: config.Kafka{Enabled: true}, sync: config.Sync{ProjectorEnabled: true}},
		{name: "projector shadow", cfg: config.Message{RuntimeMode: "shadow", InboxWriteMode: "projector"}, kafka: config.Kafka{Enabled: true}, want: "requires message.runtime_mode owner"},
		{name: "projector without Kafka", cfg: config.Message{RuntimeMode: "owner", InboxWriteMode: "projector"}, want: "requires kafka.enabled"},
		{name: "projector without Sync projector", cfg: config.Message{RuntimeMode: "owner", InboxWriteMode: "projector"}, kafka: config.Kafka{Enabled: true}, want: "requires sync.projector_enabled"},
		{name: "unknown", cfg: config.Message{RuntimeMode: "owner", InboxWriteMode: "dual"}, want: "unsupported message.inbox_write_mode"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateMessageInboxWriteMode(test.cfg, test.kafka, test.sync)
			if test.want == "" && err != nil {
				t.Fatalf("validate mode: %v", err)
			}
			if test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}
