package config

import (
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestAIResolvedRuntimeMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   AI
		wantMode string
		wantRun  bool
		wantErr  bool
	}{
		{name: "legacy disabled", config: AI{Enabled: false}, wantMode: AIRuntimeOff},
		{name: "legacy enabled", config: AI{Enabled: true}, wantMode: AIRuntimeEmbedded, wantRun: true},
		{name: "explicit off", config: AI{Enabled: true, RuntimeMode: " off "}, wantMode: AIRuntimeOff},
		{name: "explicit embedded", config: AI{RuntimeMode: "EMBEDDED"}, wantMode: AIRuntimeEmbedded, wantRun: true},
		{name: "shadow keeps embedded authority", config: AI{RuntimeMode: "shadow"}, wantMode: AIRuntimeShadow, wantRun: true},
		{name: "remote disables embedded", config: AI{Enabled: true, RuntimeMode: "remote"}, wantMode: AIRuntimeRemote},
		{name: "invalid rejected", config: AI{RuntimeMode: "dual"}, wantErr: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			mode, err := test.config.ResolvedRuntimeMode()
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected invalid mode rejection, got %q", mode)
				}
				return
			}
			if err != nil || mode != test.wantMode {
				t.Fatalf("resolved mode = %q, want %q, err=%v", mode, test.wantMode, err)
			}
			run, err := test.config.RunsEmbeddedAgent()
			if err != nil || run != test.wantRun {
				t.Fatalf("runs embedded = %v, want %v, err=%v", run, test.wantRun, err)
			}
		})
	}
}

func TestAIResolvedPolicyMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  AI
		want    string
		wantErr bool
	}{
		{name: "default persistent", config: AI{}, want: AIPolicyPersistent},
		{name: "explicit persistent", config: AI{PolicyMode: " PERSISTENT "}, want: AIPolicyPersistent},
		{name: "explicit static rollback", config: AI{PolicyMode: "static"}, want: AIPolicyStatic},
		{name: "invalid", config: AI{PolicyMode: "dual"}, wantErr: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := test.config.ResolvedPolicyMode()
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected invalid policy mode rejection, got %q", got)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("resolved policy mode = %q, want %q, err=%v", got, test.want, err)
			}
		})
	}
}

func TestConfigDistDeclaresAIRuntimeMode(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.SetConfigFile(filepath.Join("..", "..", "configs", "config.dist.yaml"))
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("read config.dist.yaml: %v", err)
	}
	if got := v.GetString("ai.runtime_mode"); got != AIRuntimeEmbedded {
		t.Fatalf("ai.runtime_mode = %q, want %q", got, AIRuntimeEmbedded)
	}
	if got := v.GetString("ai.policy_mode"); got != AIPolicyPersistent {
		t.Fatalf("ai.policy_mode = %q, want %q", got, AIPolicyPersistent)
	}
}
