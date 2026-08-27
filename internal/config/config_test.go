package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestConfigDistDeclaresSafeDependencyReadinessDefaults(t *testing.T) {
	v := viper.New()
	v.SetConfigFile(filepath.Join("..", "..", "configs", "config.dist.yaml"))
	if err := v.ReadInConfig(); err != nil {
		t.Fatal(err)
	}
	if v.GetBool("metrics.dependency_probes_enabled") {
		t.Fatal("dependency probes must remain opt-in outside deployment overlays")
	}
	if v.GetInt("metrics.dependency_probe_interval_seconds") != 5 ||
		v.GetInt("metrics.dependency_probe_timeout_ms") != 1000 ||
		v.GetInt("metrics.dependency_failure_threshold") != 3 ||
		v.GetInt("metrics.dependency_success_threshold") != 2 {
		t.Fatal("dependency readiness defaults drifted")
	}
}

func TestConfigDistKeepsDeliveryObservationShadowDisabled(t *testing.T) {
	v := viper.New()
	v.SetConfigFile(filepath.Join("..", "..", "configs", "config.dist.yaml"))
	if err := v.ReadInConfig(); err != nil {
		t.Fatal(err)
	}
	if v.GetBool("internal_rpc.delivery_observation_enabled") {
		t.Fatal("C2 delivery observation receiver must remain opt-in")
	}
	if got := v.GetString("internal_rpc.delivery_observation_listen_address"); got != "127.0.0.1:9095" {
		t.Fatalf("delivery observation listener = %q", got)
	}
	if v.GetInt("internal_rpc.delivery_observation_capacity") != 1024 ||
		v.GetInt("internal_rpc.delivery_observation_retry_after_ms") != 25 {
		t.Fatal("delivery observation queue defaults drifted")
	}
	if v.GetBool("internal_rpc.delivery_primary_enabled") {
		t.Fatal("C2 primary delivery must remain opt-in")
	}
	if v.GetInt("internal_rpc.delivery_primary_replay_capacity") != 8192 {
		t.Fatal("primary delivery replay capacity drifted")
	}
	if got := v.GetString("realtime.delivery"); got != "go" {
		t.Fatalf("safe realtime delivery default = %q, want go", got)
	}
}

func TestConfigureConfigSourceUsesExplicitEnvironmentFile(t *testing.T) {
	t.Setenv("DIPOLE_CONFIG_FILE", "/tmp/dipole-explicit-config.yaml")
	v := viper.New()
	configureConfigSource(v)
	if err := v.ReadInConfig(); err == nil || !strings.Contains(err.Error(), "/tmp/dipole-explicit-config.yaml") {
		t.Fatalf("explicit config source was not selected: %v", err)
	}
}

func TestConfigureConfigSourceKeepsLegacySearchWhenUnset(t *testing.T) {
	t.Setenv("DIPOLE_CONFIG_FILE", "")
	directory := t.TempDir()
	if err := os.MkdirAll(filepath.Join(directory, "configs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "configs", "config.yaml"), []byte("app:\n  name: fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(directory)
	v := viper.New()
	configureConfigSource(v)
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("read legacy config search path: %v", err)
	}
	if got := v.GetString("app.name"); got != "fixture" {
		t.Fatalf("legacy config value = %q", got)
	}
}

func TestElasticsearchConfigLoadsEnvironmentOverrides(t *testing.T) {
	t.Setenv("DIPOLE_ELASTICSEARCH_ENABLED", "true")
	t.Setenv("DIPOLE_ELASTICSEARCH_ADDRESS", "http://search.example:9200")
	v := viper.New()
	v.SetEnvPrefix("DIPOLE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	for _, key := range []string{"elasticsearch.enabled", "elasticsearch.address"} {
		if err := v.BindEnv(key); err != nil {
			t.Fatalf("bind %s: %v", key, err)
		}
	}
	elasticsearch := elasticsearchConfig(v)
	if !elasticsearch.Enabled || elasticsearch.Address != "http://search.example:9200" {
		t.Fatalf("Elasticsearch environment override = %+v", elasticsearch)
	}
}

func TestConfigDistDeclaresDisabledIsolatedAgentArtifactStorage(t *testing.T) {
	v := viper.New()
	v.SetConfigFile(filepath.Join("..", "..", "configs", "config.dist.yaml"))
	if err := v.ReadInConfig(); err != nil {
		t.Fatal(err)
	}
	if v.GetBool("storage.artifact_enabled") {
		t.Fatal("Agent Artifact storage must remain opt-in outside deployment overlays")
	}
	for _, key := range []string{
		"storage.artifact_endpoint",
		"storage.artifact_access_key",
		"storage.artifact_secret_key",
		"storage.artifact_use_ssl",
		"storage.artifact_bucket",
		"storage.artifact_audit_access_key",
		"storage.artifact_audit_secret_key",
		"storage.artifact_maintenance_access_key",
		"storage.artifact_maintenance_secret_key",
	} {
		if !v.IsSet(key) {
			t.Fatalf("missing isolated Agent Artifact storage key %s", key)
		}
	}
	if v.GetString("storage.artifact_access_key") == v.GetString("storage.access_key") ||
		v.GetString("storage.artifact_secret_key") == v.GetString("storage.secret_key") ||
		v.GetString("storage.artifact_bucket") == v.GetString("storage.bucket") {
		t.Fatal("Agent Artifact storage must not inherit the general file identity or bucket")
	}
}

func TestAgentArtifactMinIOPolicyAllowsOnlyRequiredPrefixOperations(t *testing.T) {
	type statement struct {
		Effect   string   `json:"Effect"`
		Action   []string `json:"Action"`
		Resource []string `json:"Resource"`
	}
	var policy struct {
		Statement []statement `json:"Statement"`
	}
	body, err := os.ReadFile(filepath.Join("..", "..", "configs", "minio", "agent-artifact-policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &policy); err != nil {
		t.Fatal(err)
	}
	var actions, resources []string
	for _, item := range policy.Statement {
		if item.Effect != "Allow" {
			t.Fatalf("unexpected policy effect %q", item.Effect)
		}
		actions = append(actions, item.Action...)
		resources = append(resources, item.Resource...)
	}
	for _, required := range []string{"s3:GetBucketLocation", "s3:ListBucket", "s3:GetObject", "s3:PutObject"} {
		if !slices.Contains(actions, required) {
			t.Fatalf("missing required action %s", required)
		}
	}
	for _, forbidden := range []string{"s3:DeleteObject", "s3:*"} {
		if slices.Contains(actions, forbidden) {
			t.Fatalf("forbidden Artifact runtime action %s", forbidden)
		}
	}
	for _, resource := range resources {
		if resource != "arn:aws:s3:::dipole-agent-artifacts" && resource != "arn:aws:s3:::dipole-agent-artifacts/agent-artifacts/v1/*" {
			t.Fatalf("policy escapes dedicated Artifact storage: %s", resource)
		}
	}
}

func TestPlatformMinIOPolicyCannotReachAgentArtifacts(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "configs", "minio", "platform-storage-policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) == "" {
		t.Fatal("platform storage policy is empty")
	}
	if strings.Contains(string(body), "arn:aws:s3:::dipole-agent-artifacts") {
		t.Fatal("platform storage identity must not receive Agent Artifact resources")
	}
	var policy map[string]any
	if err := json.Unmarshal(body, &policy); err != nil {
		t.Fatal(err)
	}
}

func TestAgentArtifactAuditPolicyIsStrictlyReadOnly(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "configs", "minio", "agent-artifact-audit-policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, required := range []string{"s3:GetBucketLocation", "s3:ListBucket", "arn:aws:s3:::dipole-agent-artifacts"} {
		if !strings.Contains(text, required) {
			t.Fatalf("audit policy is missing %s", required)
		}
	}
	for _, forbidden := range []string{"PutObject", "DeleteObject", "GetObject", "s3:*"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("audit policy contains forbidden action %s", forbidden)
		}
	}
	var policy map[string]any
	if err := json.Unmarshal(body, &policy); err != nil {
		t.Fatal(err)
	}
}

func TestAgentArtifactMaintenancePolicyCanInspectButCannotMutate(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "configs", "minio", "agent-artifact-maintenance-policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, required := range []string{"s3:GetBucketLocation", "s3:GetObject", "arn:aws:s3:::dipole-agent-artifacts/agent-artifacts/v1/*"} {
		if !strings.Contains(text, required) {
			t.Fatalf("maintenance policy is missing %s", required)
		}
	}
	for _, forbidden := range []string{"PutObject", "DeleteObject", "s3:*"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("maintenance policy contains forbidden action %s", forbidden)
		}
	}
	var policy map[string]any
	if err := json.Unmarshal(body, &policy); err != nil {
		t.Fatal(err)
	}
}

func TestSyncConfigLoadsCassandraShadowHydrationFromEnvironment(t *testing.T) {
	t.Chdir(filepath.Join("..", ".."))
	t.Setenv("DIPOLE_CONFIG_FILE", filepath.Join("configs", "config.dist.yaml"))
	t.Setenv("DIPOLE_SYNC_CASSANDRA_SHADOW_HYDRATION", "true")
	if err := Load(); err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !SyncConfig().CassandraShadowHydration {
		t.Fatal("Sync Cassandra shadow hydration environment override was ignored")
	}
}

func TestConfigDistPlacesCassandraShadowHydrationUnderSync(t *testing.T) {
	v := viper.New()
	v.SetConfigFile(filepath.Join("..", "..", "configs", "config.dist.yaml"))
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("read config.dist.yaml: %v", err)
	}
	if !v.IsSet("sync.cassandra_shadow_hydration") {
		t.Fatal("sync.cassandra_shadow_hydration is missing from config.dist.yaml")
	}
	if v.IsSet("message.cassandra_shadow_hydration") {
		t.Fatal("Sync hydration setting must not be declared under message")
	}
}

func TestConfigDistDeclaresCassandraDuplicateHydrationUnderMessage(t *testing.T) {
	v := viper.New()
	v.SetConfigFile(filepath.Join("..", "..", "configs", "config.dist.yaml"))
	if err := v.ReadInConfig(); err != nil {
		t.Fatal(err)
	}
	if !v.IsSet("message.cassandra_duplicate_hydration") {
		t.Fatal("message.cassandra_duplicate_hydration is missing from config.dist.yaml")
	}
}

func TestMergeMySQLConfigKeepsGlobalDefaultsAndAppliesSyncCredentials(t *testing.T) {
	global := MySQL{Host: "mysql", Port: 3306, User: "dipole", Password: "global", DBName: "dipole"}
	got := mergeMySQLConfig(global, MySQL{User: "dipole_sync", Password: "sync"})
	want := MySQL{Host: "mysql", Port: 3306, User: "dipole_sync", Password: "sync", DBName: "dipole"}
	if got != want {
		t.Fatalf("merged MySQL config = %+v, want %+v", got, want)
	}
}

func TestMergeMySQLConfigIgnoresEmptySyncOverride(t *testing.T) {
	global := MySQL{Host: "mysql", Port: 3306, User: "dipole", Password: "global", DBName: "dipole"}
	if got := mergeMySQLConfig(global, MySQL{}); got != global {
		t.Fatalf("empty override changed config: %+v", got)
	}
}

func TestConfigDistDeclaresSearchMaintenanceMySQLOverride(t *testing.T) {
	v := viper.New()
	v.SetConfigFile(filepath.Join("..", "..", "configs", "config.dist.yaml"))
	if err := v.ReadInConfig(); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"search.mysql.host", "search.mysql.port", "search.mysql.user", "search.mysql.password", "search.mysql.dbname"} {
		if !v.IsSet(key) {
			t.Fatalf("missing Search maintenance config key %s", key)
		}
	}
}

func TestConfigDistDeclaresMessageServiceMySQLOverride(t *testing.T) {
	v := viper.New()
	v.SetConfigFile(filepath.Join("..", "..", "configs", "config.dist.yaml"))
	if err := v.ReadInConfig(); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"message.mysql.host", "message.mysql.port", "message.mysql.user", "message.mysql.password", "message.mysql.dbname"} {
		if !v.IsSet(key) {
			t.Fatalf("missing Message Service config key %s", key)
		}
	}
}

func TestConfigDistKeepsAgentTaskControlsDefaultOff(t *testing.T) {
	v := viper.New()
	v.SetConfigFile(filepath.Join("..", "..", "configs", "config.dist.yaml"))
	if err := v.ReadInConfig(); err != nil {
		t.Fatal(err)
	}
	if v.GetBool("gateway.agent_control_enabled") {
		t.Fatal("Gateway Agent Task controls must remain default off")
	}
	if v.GetString("gateway.agent_control_target") == "" {
		t.Fatal("Gateway Agent Task control target is missing")
	}
	if v.GetBool("gateway.agent_mcp_enabled") {
		t.Fatal("Gateway Agent MCP must remain default off")
	}
	if v.GetString("gateway.agent_mcp_target") == "" {
		t.Fatal("Gateway Agent MCP target is missing")
	}
	if v.GetInt("rate_limit.agent_mcp_limit") <= 0 || v.GetInt("rate_limit.agent_mcp_window_seconds") <= 0 {
		t.Fatal("Gateway Agent MCP rate limit must remain bounded when the route is enabled")
	}
	if v.GetString("auth.agent_mcp_resource") != "https://dipole.local/api/v1/agent/mcp" {
		t.Fatal("Agent MCP canonical resource default is missing")
	}
}
