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

func TestSyncConfigLoadsCassandraShadowHydrationFromEnvironment(t *testing.T) {
	t.Chdir(filepath.Join("..", ".."))
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
}
