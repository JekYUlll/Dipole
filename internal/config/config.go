package config

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type App struct {
	Name string `mapstructure:"name"`
	Env  string `mapstructure:"env"`
}

type Log struct {
	Level           string `mapstructure:"level"`
	Format          string `mapstructure:"format"`
	Development     bool   `mapstructure:"development"`
	FileEnabled     bool   `mapstructure:"file_enabled"`
	FilePath        string `mapstructure:"file_path"`
	FileRotateDaily bool   `mapstructure:"file_rotate_daily"`
}

type Server struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type Gateway struct {
	Mode                      string `mapstructure:"mode"`
	CoreHTTPTarget            string `mapstructure:"core_http_target"`
	AgentControlEnabled       bool   `mapstructure:"agent_control_enabled"`
	AgentControlTarget        string `mapstructure:"agent_control_target"`
	AgentSubscriptionEnabled  bool   `mapstructure:"agent_subscription_enabled"`
	AgentSubscriptionTenantID string `mapstructure:"agent_subscription_tenant_id"`
	AgentMCPEnabled           bool   `mapstructure:"agent_mcp_enabled"`
	AgentMCPTarget            string `mapstructure:"agent_mcp_target"`
}

type Realtime struct {
	Delivery       string `mapstructure:"delivery"`
	FencingEnabled bool   `mapstructure:"fencing_enabled"`
	FencingKey     string `mapstructure:"fencing_key"`
	FencingEpoch   uint64 `mapstructure:"fencing_epoch"`
}

type TLS struct {
	Enabled  bool   `mapstructure:"enabled"`
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
}

type Metrics struct {
	Enabled                        bool   `mapstructure:"enabled"`
	Address                        string `mapstructure:"address"`
	DependencyProbesEnabled        bool   `mapstructure:"dependency_probes_enabled"`
	DependencyProbeIntervalSeconds int    `mapstructure:"dependency_probe_interval_seconds"`
	DependencyProbeTimeoutMS       int    `mapstructure:"dependency_probe_timeout_ms"`
	DependencyFailureThreshold     int    `mapstructure:"dependency_failure_threshold"`
	DependencySuccessThreshold     int    `mapstructure:"dependency_success_threshold"`
}

type MySQL struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
}

type Redis struct {
	Mode               string   `mapstructure:"mode"`
	Host               string   `mapstructure:"host"`
	Port               int      `mapstructure:"port"`
	Password           string   `mapstructure:"password"`
	DB                 int      `mapstructure:"db"`
	SentinelMasterName string   `mapstructure:"sentinel_master_name"`
	SentinelAddresses  []string `mapstructure:"sentinel_addresses"`
	SentinelPassword   string   `mapstructure:"sentinel_password"`
}

type Cassandra struct {
	Enabled               bool     `mapstructure:"enabled"`
	Hosts                 []string `mapstructure:"hosts"`
	Keyspace              string   `mapstructure:"keyspace"`
	LocalDatacenter       string   `mapstructure:"local_datacenter"`
	TimelineBucketSize    uint64   `mapstructure:"timeline_bucket_size"`
	ConnectTimeoutSeconds int      `mapstructure:"connect_timeout_seconds"`
}

type Elasticsearch struct {
	Enabled               bool   `mapstructure:"enabled"`
	Address               string `mapstructure:"address"`
	IndexPrefix           string `mapstructure:"index_prefix"`
	Shards                int    `mapstructure:"shards"`
	Replicas              int    `mapstructure:"replicas"`
	RequestTimeoutSeconds int    `mapstructure:"request_timeout_seconds"`
	Username              string `mapstructure:"username"`
	Password              string `mapstructure:"password"`
	APIKey                string `mapstructure:"api_key"`
}

type Auth struct {
	TokenTTLHours    int    `mapstructure:"token_ttl_hours"`
	JWTSecret        string `mapstructure:"jwt_secret"`
	JWTIssuer        string `mapstructure:"jwt_issuer"`
	AgentMCPResource string `mapstructure:"agent_mcp_resource"`
}

type Kafka struct {
	Enabled                         bool     `mapstructure:"enabled"`
	Brokers                         []string `mapstructure:"brokers"`
	ClientID                        string   `mapstructure:"client_id"`
	TopicPrefix                     string   `mapstructure:"topic_prefix"`
	TopicPartitions                 int      `mapstructure:"topic_partitions"`
	TopicReplicationFactor          int      `mapstructure:"topic_replication_factor"`
	TopicMinInSyncReplicas          int      `mapstructure:"topic_min_insync_replicas"`
	TopicRetentionHours             int      `mapstructure:"topic_retention_hours"`
	RequiredAcks                    string   `mapstructure:"required_acks"`
	DialTimeoutSeconds              int      `mapstructure:"dial_timeout_seconds"`
	WriteTimeoutSeconds             int      `mapstructure:"write_timeout_seconds"`
	ConsumeRetryMaxAttempts         int      `mapstructure:"consume_retry_max_attempts"`
	ConsumeRetryBackoffMS           int      `mapstructure:"consume_retry_backoff_ms"`
	ConsumerGroupBalancer           string   `mapstructure:"consumer_group_balancer"`
	ConsumerHeartbeatSeconds        int      `mapstructure:"consumer_heartbeat_seconds"`
	ConsumerSessionTimeoutSeconds   int      `mapstructure:"consumer_session_timeout_seconds"`
	ConsumerRebalanceTimeoutSeconds int      `mapstructure:"consumer_rebalance_timeout_seconds"`
}

type Message struct {
	Transport                   string `mapstructure:"transport"`
	RuntimeMode                 string `mapstructure:"runtime_mode"`
	ShadowQueries               bool   `mapstructure:"shadow_queries"`
	CassandraShadowReads        bool   `mapstructure:"cassandra_shadow_reads"`
	CassandraReadPercent        int    `mapstructure:"cassandra_read_percentage"`
	CassandraReadVerifyPercent  int    `mapstructure:"cassandra_read_verify_percentage"`
	CassandraDuplicateHydration bool   `mapstructure:"cassandra_duplicate_hydration"`
	EnforceDBPermissions        bool   `mapstructure:"enforce_db_permissions"`
	InboxWriteMode              string `mapstructure:"inbox_write_mode"`
	TimelineNotifyMode          string `mapstructure:"timeline_notify_mode"`
}

type Search struct {
	Enabled bool `mapstructure:"enabled"`
}

type Sync struct {
	Transport                string `mapstructure:"transport"`
	ShadowQueries            bool   `mapstructure:"shadow_queries"`
	ProjectorEnabled         bool   `mapstructure:"projector_enabled"`
	EnforceDBPermissions     bool   `mapstructure:"enforce_db_permissions"`
	CassandraShadowHydration bool   `mapstructure:"cassandra_shadow_hydration"`
}

type InternalRPC struct {
	Enabled                          bool   `mapstructure:"enabled"`
	SharedSecret                     string `mapstructure:"shared_secret"`
	CoreListenAddress                string `mapstructure:"core_listen_address"`
	CoreTarget                       string `mapstructure:"core_target"`
	MessageListenAddress             string `mapstructure:"message_listen_address"`
	MessageTarget                    string `mapstructure:"message_target"`
	SearchListenAddress              string `mapstructure:"search_listen_address"`
	SearchTarget                     string `mapstructure:"search_target"`
	SyncListenAddress                string `mapstructure:"sync_listen_address"`
	SyncTarget                       string `mapstructure:"sync_target"`
	DeliveryObservationEnabled       bool   `mapstructure:"delivery_observation_enabled"`
	DeliveryObservationListenAddress string `mapstructure:"delivery_observation_listen_address"`
	DeliveryObservationCapacity      int    `mapstructure:"delivery_observation_capacity"`
	DeliveryObservationRetryAfterMS  int    `mapstructure:"delivery_observation_retry_after_ms"`
	DeliveryPrimaryEnabled           bool   `mapstructure:"delivery_primary_enabled"`
	DeliveryPrimaryReplayCapacity    int    `mapstructure:"delivery_primary_replay_capacity"`
	DialTimeoutSeconds               int    `mapstructure:"dial_timeout_seconds"`
	ShutdownTimeoutSeconds           int    `mapstructure:"shutdown_timeout_seconds"`
	TLSEnabled                       bool   `mapstructure:"tls_enabled"`
	TLSCertFile                      string `mapstructure:"tls_cert_file"`
	TLSKeyFile                       string `mapstructure:"tls_key_file"`
	TLSCAFile                        string `mapstructure:"tls_ca_file"`
	TLSServerName                    string `mapstructure:"tls_server_name"`
}

type Storage struct {
	Enabled                      bool   `mapstructure:"enabled"`
	Provider                     string `mapstructure:"provider"`
	Endpoint                     string `mapstructure:"endpoint"`
	PresignEndpoint              string `mapstructure:"presign_endpoint"`
	AccessKey                    string `mapstructure:"access_key"`
	SecretKey                    string `mapstructure:"secret_key"`
	UseSSL                       bool   `mapstructure:"use_ssl"`
	Bucket                       string `mapstructure:"bucket"`
	SearchArchiveBucket          string `mapstructure:"search_archive_bucket"`
	SearchArchiveRetentionDays   int    `mapstructure:"search_archive_retention_days"`
	MessageArchiveBucket         string `mapstructure:"message_archive_bucket"`
	MessageArchiveRetentionDays  int    `mapstructure:"message_archive_retention_days"`
	ArtifactEnabled              bool   `mapstructure:"artifact_enabled"`
	ArtifactEndpoint             string `mapstructure:"artifact_endpoint"`
	ArtifactAccessKey            string `mapstructure:"artifact_access_key"`
	ArtifactSecretKey            string `mapstructure:"artifact_secret_key"`
	ArtifactUseSSL               bool   `mapstructure:"artifact_use_ssl"`
	ArtifactBucket               string `mapstructure:"artifact_bucket"`
	ArtifactAuditAccessKey       string `mapstructure:"artifact_audit_access_key"`
	ArtifactAuditSecretKey       string `mapstructure:"artifact_audit_secret_key"`
	ArtifactMaintenanceAccessKey string `mapstructure:"artifact_maintenance_access_key"`
	ArtifactMaintenanceSecretKey string `mapstructure:"artifact_maintenance_secret_key"`
	PublicBaseURL                string `mapstructure:"public_base_url"`
	FileMaxSizeMB                int64  `mapstructure:"file_max_size_mb"`
	MultipartChunkSizeMB         int64  `mapstructure:"multipart_chunk_size_mb"`
	MultipartSessionTTLMin       int    `mapstructure:"multipart_session_ttl_minutes"`
	DownloadURLTTLMinutes        int    `mapstructure:"download_url_ttl_minutes"`
}

type RateLimit struct {
	Enabled                 bool `mapstructure:"enabled"`
	RegisterLimit           int  `mapstructure:"register_limit"`
	RegisterWindowSeconds   int  `mapstructure:"register_window_seconds"`
	LoginLimit              int  `mapstructure:"login_limit"`
	LoginWindowSeconds      int  `mapstructure:"login_window_seconds"`
	MessageLimit            int  `mapstructure:"message_limit"`
	MessageWindowSeconds    int  `mapstructure:"message_window_seconds"`
	FileUploadLimit         int  `mapstructure:"file_upload_limit"`
	FileUploadWindowSeconds int  `mapstructure:"file_upload_window_seconds"`
	AgentMCPLimit           int  `mapstructure:"agent_mcp_limit"`
	AgentMCPWindowSeconds   int  `mapstructure:"agent_mcp_window_seconds"`
}

type Presence struct {
	Enabled    bool   `mapstructure:"enabled"`
	NodeID     string `mapstructure:"node_id"`
	TTLSeconds int    `mapstructure:"ttl_seconds"`
}

type HotGroup struct {
	Enabled              bool `mapstructure:"enabled"`
	MemberCountThreshold int  `mapstructure:"member_count_threshold"`
	MessageThreshold     int  `mapstructure:"message_threshold"`
	WindowSeconds        int  `mapstructure:"window_seconds"`
	CoolingSeconds       int  `mapstructure:"cooling_seconds"`
}

type AI struct {
	Enabled            bool   `mapstructure:"enabled"`
	RuntimeMode        string `mapstructure:"runtime_mode"`
	PolicyMode         string `mapstructure:"policy_mode"`
	Provider           string `mapstructure:"provider"`
	Model              string `mapstructure:"model"`
	APIKey             string `mapstructure:"api_key"`
	BaseURL            string `mapstructure:"base_url"`
	TimeoutSeconds     int    `mapstructure:"timeout_seconds"`
	MaxContextMessages int    `mapstructure:"max_context_messages"`
	AssistantUUID      string `mapstructure:"assistant_uuid"`
	AssistantNickname  string `mapstructure:"assistant_nickname"`
	AssistantTelephone string `mapstructure:"assistant_telephone"`
	AssistantEmail     string `mapstructure:"assistant_email"`
	AssistantAvatar    string `mapstructure:"assistant_avatar"`
	SystemPrompt       string `mapstructure:"system_prompt"`
}

const (
	AIRuntimeOff       = "off"
	AIRuntimeEmbedded  = "embedded"
	AIRuntimeShadow    = "shadow"
	AIRuntimeRemote    = "remote"
	AIPolicyStatic     = "static"
	AIPolicyPersistent = "persistent"
)

func (a AI) ResolvedRuntimeMode() (string, error) {
	mode := strings.ToLower(strings.TrimSpace(a.RuntimeMode))
	if mode == "" {
		if a.Enabled {
			return AIRuntimeEmbedded, nil
		}
		return AIRuntimeOff, nil
	}
	switch mode {
	case AIRuntimeOff, AIRuntimeEmbedded, AIRuntimeShadow, AIRuntimeRemote:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid AI runtime mode %q: expected off, embedded, shadow, or remote", a.RuntimeMode)
	}
}

func (a AI) ResolvedPolicyMode() (string, error) {
	mode := strings.ToLower(strings.TrimSpace(a.PolicyMode))
	if mode == "" {
		return AIPolicyPersistent, nil
	}
	switch mode {
	case AIPolicyStatic, AIPolicyPersistent:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid AI policy mode %q: expected static or persistent", a.PolicyMode)
	}
}

func (a AI) RunsEmbeddedAgent() (bool, error) {
	mode, err := a.ResolvedRuntimeMode()
	if err != nil {
		return false, err
	}
	return mode == AIRuntimeEmbedded || mode == AIRuntimeShadow, nil
}

type AIProvider struct {
	Name           string
	Model          string
	APIKey         string
	BaseURL        string
	TimeoutSeconds int
}

var (
	cfg     *viper.Viper
	loadErr error
	once    sync.Once
)

func configureConfigSource(v *viper.Viper) {
	if configFile := strings.TrimSpace(os.Getenv("DIPOLE_CONFIG_FILE")); configFile != "" {
		v.SetConfigFile(configFile)
		return
	}
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("configs")
	v.AddConfigPath(".")
}

func Load() error {
	once.Do(func() {
		v := viper.New()
		configureConfigSource(v)

		v.SetEnvPrefix("DIPOLE")
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		// Load .env file if present; ignore error when file doesn't exist.
		_ = godotenv.Load()
		v.AutomaticEnv()

		v.SetDefault("app.name", "dipole")
		v.SetDefault("app.env", "local")
		v.SetDefault("log.level", "info")
		v.SetDefault("log.format", "console")
		v.SetDefault("log.development", true)
		v.SetDefault("log.file_enabled", false)
		v.SetDefault("log.file_path", "logs/dipole.log")
		v.SetDefault("log.file_rotate_daily", true)
		v.SetDefault("server.host", "0.0.0.0")
		v.SetDefault("server.port", 8080)
		v.SetDefault("gateway.mode", "embedded")
		v.SetDefault("gateway.core_http_target", "http://127.0.0.1:8081")
		v.SetDefault("gateway.agent_control_enabled", false)
		v.SetDefault("gateway.agent_control_target", "http://127.0.0.1:8091")
		v.SetDefault("gateway.agent_subscription_enabled", false)
		v.SetDefault("gateway.agent_subscription_tenant_id", "dipole")
		v.SetDefault("gateway.agent_mcp_enabled", false)
		v.SetDefault("gateway.agent_mcp_target", "http://127.0.0.1:8091")
		v.SetDefault("realtime.delivery", "go")
		v.SetDefault("realtime.fencing_enabled", false)
		v.SetDefault("realtime.fencing_key", "dipole:realtime:delivery:authority:v1")
		v.SetDefault("realtime.fencing_epoch", 0)
		v.SetDefault("tls.enabled", false)
		v.SetDefault("tls.cert_file", "certs/local/dipole-local.pem")
		v.SetDefault("tls.key_file", "certs/local/dipole-local-key.pem")
		v.SetDefault("metrics.enabled", false)
		v.SetDefault("metrics.address", "127.0.0.1:9100")
		v.SetDefault("metrics.dependency_probes_enabled", false)
		v.SetDefault("metrics.dependency_probe_interval_seconds", 5)
		v.SetDefault("metrics.dependency_probe_timeout_ms", 1000)
		v.SetDefault("metrics.dependency_failure_threshold", 3)
		v.SetDefault("metrics.dependency_success_threshold", 2)
		v.SetDefault("auth.token_ttl_hours", 168)
		v.SetDefault("auth.jwt_secret", "dipole-dev-jwt-secret-change-me")
		v.SetDefault("auth.jwt_issuer", "dipole")
		v.SetDefault("auth.agent_mcp_resource", "https://dipole.local/api/v1/agent/mcp")
		v.SetDefault("redis.mode", "single")
		v.SetDefault("cassandra.enabled", false)
		v.SetDefault("cassandra.hosts", []string{"127.0.0.1:19042"})
		v.SetDefault("cassandra.keyspace", "dipole_message_shadow")
		v.SetDefault("cassandra.local_datacenter", "datacenter1")
		v.SetDefault("cassandra.timeline_bucket_size", 10000)
		v.SetDefault("cassandra.connect_timeout_seconds", 5)
		v.SetDefault("elasticsearch.enabled", false)
		v.SetDefault("elasticsearch.address", "http://127.0.0.1:19200")
		v.SetDefault("elasticsearch.index_prefix", "dipole")
		v.SetDefault("elasticsearch.shards", 1)
		v.SetDefault("elasticsearch.replicas", 0)
		v.SetDefault("elasticsearch.request_timeout_seconds", 10)
		v.SetDefault("elasticsearch.username", "")
		v.SetDefault("elasticsearch.password", "")
		v.SetDefault("elasticsearch.api_key", "")
		v.SetDefault("kafka.enabled", false)
		v.SetDefault("kafka.brokers", []string{"127.0.0.1:9092"})
		v.SetDefault("kafka.client_id", "dipole")
		v.SetDefault("kafka.topic_prefix", "dipole")
		v.SetDefault("kafka.topic_partitions", 6)
		v.SetDefault("kafka.topic_replication_factor", 1)
		v.SetDefault("kafka.topic_min_insync_replicas", 1)
		v.SetDefault("kafka.topic_retention_hours", 168)
		v.SetDefault("kafka.required_acks", "one")
		v.SetDefault("kafka.dial_timeout_seconds", 5)
		v.SetDefault("kafka.write_timeout_seconds", 5)
		v.SetDefault("kafka.consume_retry_max_attempts", 3)
		v.SetDefault("kafka.consume_retry_backoff_ms", 500)
		v.SetDefault("kafka.consumer_group_balancer", "roundrobin")
		v.SetDefault("kafka.consumer_heartbeat_seconds", 3)
		v.SetDefault("kafka.consumer_session_timeout_seconds", 30)
		v.SetDefault("kafka.consumer_rebalance_timeout_seconds", 30)
		v.SetDefault("message.transport", "local")
		v.SetDefault("message.runtime_mode", "owner")
		v.SetDefault("message.shadow_queries", false)
		v.SetDefault("message.cassandra_shadow_reads", false)
		v.SetDefault("message.cassandra_read_percentage", 0)
		v.SetDefault("message.cassandra_read_verify_percentage", 0)
		v.SetDefault("message.cassandra_duplicate_hydration", false)
		v.SetDefault("message.enforce_db_permissions", false)
		v.SetDefault("message.inbox_write_mode", "atomic")
		v.SetDefault("message.timeline_notify_mode", "off")
		v.SetDefault("message.mysql.host", "")
		v.SetDefault("message.mysql.port", 0)
		v.SetDefault("message.mysql.user", "")
		v.SetDefault("message.mysql.password", "")
		v.SetDefault("message.mysql.dbname", "")
		v.SetDefault("search.enabled", false)
		v.SetDefault("sync.transport", "local")
		v.SetDefault("sync.shadow_queries", false)
		v.SetDefault("sync.projector_enabled", false)
		v.SetDefault("sync.enforce_db_permissions", false)
		v.SetDefault("sync.cassandra_shadow_hydration", false)
		v.SetDefault("internal_rpc.enabled", false)
		v.SetDefault("internal_rpc.shared_secret", "")
		v.SetDefault("internal_rpc.core_listen_address", "127.0.0.1:9091")
		v.SetDefault("internal_rpc.core_target", "127.0.0.1:9091")
		v.SetDefault("internal_rpc.message_listen_address", "127.0.0.1:9092")
		v.SetDefault("internal_rpc.message_target", "127.0.0.1:9092")
		v.SetDefault("internal_rpc.search_listen_address", "127.0.0.1:9093")
		v.SetDefault("internal_rpc.search_target", "127.0.0.1:9093")
		v.SetDefault("internal_rpc.sync_listen_address", "127.0.0.1:9094")
		v.SetDefault("internal_rpc.sync_target", "127.0.0.1:9094")
		v.SetDefault("internal_rpc.delivery_observation_enabled", false)
		v.SetDefault("internal_rpc.delivery_observation_listen_address", "127.0.0.1:9095")
		v.SetDefault("internal_rpc.delivery_observation_capacity", 1024)
		v.SetDefault("internal_rpc.delivery_observation_retry_after_ms", 25)
		v.SetDefault("internal_rpc.delivery_primary_enabled", false)
		v.SetDefault("internal_rpc.delivery_primary_replay_capacity", 8192)
		v.SetDefault("internal_rpc.dial_timeout_seconds", 5)
		v.SetDefault("internal_rpc.shutdown_timeout_seconds", 15)
		v.SetDefault("internal_rpc.tls_enabled", false)
		v.SetDefault("internal_rpc.tls_cert_file", "")
		v.SetDefault("internal_rpc.tls_key_file", "")
		v.SetDefault("internal_rpc.tls_ca_file", "")
		v.SetDefault("internal_rpc.tls_server_name", "")
		v.SetDefault("storage.enabled", false)
		v.SetDefault("storage.provider", "minio")
		v.SetDefault("storage.endpoint", "127.0.0.1:9000")
		v.SetDefault("storage.presign_endpoint", "")
		v.SetDefault("storage.access_key", "dipoleplatform")
		v.SetDefault("storage.secret_key", "dipoleplatformpass")
		v.SetDefault("storage.use_ssl", false)
		v.SetDefault("storage.bucket", "dipole-files")
		v.SetDefault("storage.search_archive_bucket", "dipole-search-archives")
		v.SetDefault("storage.search_archive_retention_days", 30)
		v.SetDefault("storage.message_archive_bucket", "dipole-message-archives")
		v.SetDefault("storage.message_archive_retention_days", 30)
		v.SetDefault("storage.artifact_enabled", false)
		v.SetDefault("storage.artifact_endpoint", "127.0.0.1:9000")
		v.SetDefault("storage.artifact_access_key", "")
		v.SetDefault("storage.artifact_secret_key", "")
		v.SetDefault("storage.artifact_use_ssl", false)
		v.SetDefault("storage.artifact_bucket", "dipole-agent-artifacts")
		v.SetDefault("storage.artifact_audit_access_key", "")
		v.SetDefault("storage.artifact_audit_secret_key", "")
		v.SetDefault("storage.artifact_maintenance_access_key", "")
		v.SetDefault("storage.artifact_maintenance_secret_key", "")
		v.SetDefault("storage.public_base_url", "http://127.0.0.1:9000/dipole-files")
		v.SetDefault("storage.file_max_size_mb", 50)
		v.SetDefault("storage.multipart_chunk_size_mb", 5)
		v.SetDefault("storage.multipart_session_ttl_minutes", 60)
		v.SetDefault("storage.download_url_ttl_minutes", 10)
		v.SetDefault("rate_limit.enabled", true)
		v.SetDefault("rate_limit.register_limit", 5)
		v.SetDefault("rate_limit.register_window_seconds", 3600)
		v.SetDefault("rate_limit.login_limit", 10)
		v.SetDefault("rate_limit.login_window_seconds", 300)
		v.SetDefault("rate_limit.message_limit", 120)
		v.SetDefault("rate_limit.message_window_seconds", 60)
		v.SetDefault("rate_limit.file_upload_limit", 10)
		v.SetDefault("rate_limit.file_upload_window_seconds", 300)
		v.SetDefault("rate_limit.agent_mcp_limit", 60)
		v.SetDefault("rate_limit.agent_mcp_window_seconds", 60)
		v.SetDefault("presence.enabled", true)
		v.SetDefault("presence.node_id", "")
		v.SetDefault("presence.ttl_seconds", 120)
		v.SetDefault("hot_group.enabled", true)
		v.SetDefault("hot_group.member_count_threshold", 200)
		v.SetDefault("hot_group.message_threshold", 50)
		v.SetDefault("hot_group.window_seconds", 60)
		v.SetDefault("hot_group.cooling_seconds", 180)
		v.SetDefault("ai.enabled", false)
		v.SetDefault("ai.runtime_mode", "")
		v.SetDefault("ai.policy_mode", AIPolicyPersistent)
		v.SetDefault("ai.provider", "openai")
		v.SetDefault("ai.model", "gpt-4o-mini")
		v.SetDefault("ai.api_key", "")
		v.SetDefault("ai.base_url", "")
		v.SetDefault("ai.timeout_seconds", 30)
		v.SetDefault("ai.max_context_messages", 12)
		v.SetDefault("ai.assistant_uuid", "UAI000000000000000001")
		v.SetDefault("ai.assistant_nickname", "Dipole AI")
		v.SetDefault("ai.assistant_telephone", "19900000000")
		v.SetDefault("ai.assistant_email", "ai@dipole.local")
		v.SetDefault("ai.assistant_avatar", "https://cube.elemecdn.com/0/88/03b0d39583f48206768a7534e55bcpng.png")
		v.SetDefault("ai.system_prompt", "You are Dipole AI, a concise and helpful instant messaging assistant. Use the conversation context to answer naturally. If the user sends a file, acknowledge the file metadata you can see. Keep answers short unless the user asks for depth.")
		for _, key := range []string{
			"app.name",
			"app.env",
			"log.level",
			"log.format",
			"log.development",
			"log.file_enabled",
			"log.file_path",
			"log.file_rotate_daily",
			"server.host",
			"server.port",
			"gateway.core_http_target",
			"gateway.agent_control_enabled",
			"gateway.agent_control_target",
			"gateway.agent_subscription_enabled",
			"gateway.agent_subscription_tenant_id",
			"gateway.agent_mcp_enabled",
			"gateway.agent_mcp_target",
			"realtime.delivery",
			"tls.enabled",
			"tls.cert_file",
			"tls.key_file",
			"metrics.enabled",
			"metrics.address",
			"auth.token_ttl_hours",
			"auth.jwt_secret",
			"auth.jwt_issuer",
			"mysql.host",
			"mysql.port",
			"mysql.user",
			"mysql.password",
			"mysql.dbname",
			"redis.host",
			"redis.port",
			"redis.password",
			"redis.db",
			"redis.mode",
			"redis.sentinel_master_name",
			"redis.sentinel_addresses",
			"redis.sentinel_password",
			"cassandra.enabled",
			"cassandra.hosts",
			"cassandra.keyspace",
			"cassandra.local_datacenter",
			"cassandra.timeline_bucket_size",
			"cassandra.connect_timeout_seconds",
			"elasticsearch.enabled",
			"elasticsearch.address",
			"elasticsearch.index_prefix",
			"elasticsearch.shards",
			"elasticsearch.replicas",
			"elasticsearch.request_timeout_seconds",
			"elasticsearch.username",
			"elasticsearch.password",
			"elasticsearch.api_key",
			"kafka.enabled",
			"kafka.brokers",
			"kafka.client_id",
			"kafka.topic_prefix",
			"kafka.topic_partitions",
			"kafka.topic_replication_factor",
			"kafka.dial_timeout_seconds",
			"kafka.write_timeout_seconds",
			"kafka.consume_retry_max_attempts",
			"kafka.consume_retry_backoff_ms",
			"message.transport",
			"message.runtime_mode",
			"message.shadow_queries",
			"message.cassandra_shadow_reads",
			"message.cassandra_read_percentage",
			"message.cassandra_read_verify_percentage",
			"message.cassandra_duplicate_hydration",
			"message.enforce_db_permissions",
			"message.inbox_write_mode",
			"message.timeline_notify_mode",
			"message.mysql.host",
			"message.mysql.port",
			"message.mysql.user",
			"message.mysql.password",
			"message.mysql.dbname",
			"search.enabled",
			"sync.transport",
			"sync.shadow_queries",
			"sync.projector_enabled",
			"sync.enforce_db_permissions",
			"sync.cassandra_shadow_hydration",
			"sync.mysql.host",
			"sync.mysql.port",
			"sync.mysql.user",
			"sync.mysql.password",
			"sync.mysql.dbname",
			"search.mysql.host",
			"search.mysql.port",
			"search.mysql.user",
			"search.mysql.password",
			"search.mysql.dbname",
			"internal_rpc.enabled",
			"internal_rpc.shared_secret",
			"internal_rpc.core_listen_address",
			"internal_rpc.core_target",
			"internal_rpc.message_listen_address",
			"internal_rpc.message_target",
			"internal_rpc.search_listen_address",
			"internal_rpc.search_target",
			"internal_rpc.sync_listen_address",
			"internal_rpc.sync_target",
			"internal_rpc.delivery_observation_enabled",
			"internal_rpc.delivery_observation_listen_address",
			"internal_rpc.delivery_observation_capacity",
			"internal_rpc.delivery_observation_retry_after_ms",
			"internal_rpc.delivery_primary_enabled",
			"internal_rpc.delivery_primary_replay_capacity",
			"internal_rpc.dial_timeout_seconds",
			"internal_rpc.shutdown_timeout_seconds",
			"internal_rpc.tls_enabled",
			"internal_rpc.tls_cert_file",
			"internal_rpc.tls_key_file",
			"internal_rpc.tls_ca_file",
			"internal_rpc.tls_server_name",
			"storage.enabled",
			"storage.provider",
			"storage.endpoint",
			"storage.presign_endpoint",
			"storage.access_key",
			"storage.secret_key",
			"storage.use_ssl",
			"storage.bucket",
			"storage.search_archive_bucket",
			"storage.search_archive_retention_days",
			"storage.message_archive_bucket",
			"storage.message_archive_retention_days",
			"storage.artifact_enabled",
			"storage.artifact_endpoint",
			"storage.artifact_access_key",
			"storage.artifact_secret_key",
			"storage.artifact_use_ssl",
			"storage.artifact_bucket",
			"storage.artifact_audit_access_key",
			"storage.artifact_audit_secret_key",
			"storage.artifact_maintenance_access_key",
			"storage.artifact_maintenance_secret_key",
			"storage.public_base_url",
			"storage.file_max_size_mb",
			"storage.download_url_ttl_minutes",
			"rate_limit.enabled",
			"rate_limit.register_limit",
			"rate_limit.register_window_seconds",
			"rate_limit.login_limit",
			"rate_limit.login_window_seconds",
			"rate_limit.message_limit",
			"rate_limit.message_window_seconds",
			"rate_limit.file_upload_limit",
			"rate_limit.file_upload_window_seconds",
			"rate_limit.agent_mcp_limit",
			"rate_limit.agent_mcp_window_seconds",
			"presence.enabled",
			"presence.node_id",
			"presence.ttl_seconds",
			"hot_group.enabled",
			"hot_group.member_count_threshold",
			"hot_group.message_threshold",
			"hot_group.window_seconds",
			"hot_group.cooling_seconds",
			"ai.enabled",
			"ai.runtime_mode",
			"ai.policy_mode",
			"ai.provider",
			"ai.model",
			"ai.api_key",
			"ai.base_url",
			"ai.timeout_seconds",
			"ai.max_context_messages",
			"ai.assistant_uuid",
			"ai.assistant_nickname",
			"ai.assistant_telephone",
			"ai.assistant_email",
			"ai.assistant_avatar",
			"ai.system_prompt",
		} {
			if err := v.BindEnv(key); err != nil {
				loadErr = fmt.Errorf("bind env for %s: %w", key, err)
				return
			}
		}

		if err := v.ReadInConfig(); err != nil {
			loadErr = fmt.Errorf("read config: %w", err)
			return
		}

		cfg = v
	})

	return loadErr
}

func MustLoad() {
	if err := Load(); err != nil {
		panic(err)
	}
}

func AppConfig() App {
	MustLoad()

	var app App
	if err := cfg.UnmarshalKey("app", &app); err != nil {
		panic(fmt.Errorf("unmarshal app config: %w", err))
	}

	return app
}

func LogConfig() Log {
	MustLoad()

	var logConfig Log
	if err := cfg.UnmarshalKey("log", &logConfig); err != nil {
		panic(fmt.Errorf("unmarshal log config: %w", err))
	}

	return logConfig
}

func ServerConfig() Server {
	MustLoad()

	var server Server
	if err := cfg.UnmarshalKey("server", &server); err != nil {
		panic(fmt.Errorf("unmarshal server config: %w", err))
	}
	server.Host = cfg.GetString("server.host")
	server.Port = cfg.GetInt("server.port")

	return server
}

func GatewayConfig() Gateway {
	MustLoad()
	return Gateway{
		Mode:                      strings.ToLower(strings.TrimSpace(cfg.GetString("gateway.mode"))),
		CoreHTTPTarget:            strings.TrimSpace(cfg.GetString("gateway.core_http_target")),
		AgentControlEnabled:       cfg.GetBool("gateway.agent_control_enabled"),
		AgentControlTarget:        strings.TrimSpace(cfg.GetString("gateway.agent_control_target")),
		AgentSubscriptionEnabled:  cfg.GetBool("gateway.agent_subscription_enabled"),
		AgentSubscriptionTenantID: strings.TrimSpace(cfg.GetString("gateway.agent_subscription_tenant_id")),
		AgentMCPEnabled:           cfg.GetBool("gateway.agent_mcp_enabled"),
		AgentMCPTarget:            strings.TrimSpace(cfg.GetString("gateway.agent_mcp_target")),
	}
}

func RealtimeConfig() Realtime {
	MustLoad()
	return Realtime{
		Delivery:       strings.ToLower(strings.TrimSpace(cfg.GetString("realtime.delivery"))),
		FencingEnabled: cfg.GetBool("realtime.fencing_enabled"),
		FencingKey:     strings.TrimSpace(cfg.GetString("realtime.fencing_key")),
		FencingEpoch:   cfg.GetUint64("realtime.fencing_epoch"),
	}
}

func MySQLConfig() MySQL {
	MustLoad()

	var mysql MySQL
	if err := cfg.UnmarshalKey("mysql", &mysql); err != nil {
		panic(fmt.Errorf("unmarshal mysql config: %w", err))
	}

	return mysql
}

func TLSConfig() TLS {
	MustLoad()

	var tlsConfig TLS
	if err := cfg.UnmarshalKey("tls", &tlsConfig); err != nil {
		panic(fmt.Errorf("unmarshal tls config: %w", err))
	}

	return tlsConfig
}

func MetricsConfig() Metrics {
	MustLoad()
	return Metrics{
		Enabled:                        cfg.GetBool("metrics.enabled"),
		Address:                        strings.TrimSpace(cfg.GetString("metrics.address")),
		DependencyProbesEnabled:        cfg.GetBool("metrics.dependency_probes_enabled"),
		DependencyProbeIntervalSeconds: cfg.GetInt("metrics.dependency_probe_interval_seconds"),
		DependencyProbeTimeoutMS:       cfg.GetInt("metrics.dependency_probe_timeout_ms"),
		DependencyFailureThreshold:     cfg.GetInt("metrics.dependency_failure_threshold"),
		DependencySuccessThreshold:     cfg.GetInt("metrics.dependency_success_threshold"),
	}
}

func RedisConfig() Redis {
	MustLoad()

	var redis Redis
	if err := cfg.UnmarshalKey("redis", &redis); err != nil {
		panic(fmt.Errorf("unmarshal redis config: %w", err))
	}

	return redis
}

func CassandraConfig() Cassandra {
	MustLoad()

	var cassandra Cassandra
	if err := cfg.UnmarshalKey("cassandra", &cassandra); err != nil {
		panic(fmt.Errorf("unmarshal Cassandra config: %w", err))
	}

	return cassandra
}

func ElasticsearchConfig() Elasticsearch {
	MustLoad()
	return elasticsearchConfig(cfg)
}

func elasticsearchConfig(source *viper.Viper) Elasticsearch {
	var elasticsearch Elasticsearch
	if err := source.UnmarshalKey("elasticsearch", &elasticsearch); err != nil {
		panic(fmt.Errorf("unmarshal Elasticsearch config: %w", err))
	}
	elasticsearch.Enabled = source.GetBool("elasticsearch.enabled")
	elasticsearch.Address = source.GetString("elasticsearch.address")
	elasticsearch.IndexPrefix = source.GetString("elasticsearch.index_prefix")
	elasticsearch.Shards = source.GetInt("elasticsearch.shards")
	elasticsearch.Replicas = source.GetInt("elasticsearch.replicas")
	elasticsearch.RequestTimeoutSeconds = source.GetInt("elasticsearch.request_timeout_seconds")
	elasticsearch.Username = source.GetString("elasticsearch.username")
	elasticsearch.Password = source.GetString("elasticsearch.password")
	elasticsearch.APIKey = source.GetString("elasticsearch.api_key")
	return elasticsearch
}

func AuthConfig() Auth {
	MustLoad()

	var auth Auth
	if err := cfg.UnmarshalKey("auth", &auth); err != nil {
		panic(fmt.Errorf("unmarshal auth config: %w", err))
	}
	auth.TokenTTLHours = cfg.GetInt("auth.token_ttl_hours")
	auth.JWTSecret = cfg.GetString("auth.jwt_secret")
	auth.JWTIssuer = cfg.GetString("auth.jwt_issuer")
	auth.AgentMCPResource = cfg.GetString("auth.agent_mcp_resource")

	return auth
}

func KafkaConfig() Kafka {
	MustLoad()

	var kafkaConfig Kafka
	if err := cfg.UnmarshalKey("kafka", &kafkaConfig); err != nil {
		panic(fmt.Errorf("unmarshal kafka config: %w", err))
	}
	kafkaConfig.Enabled = cfg.GetBool("kafka.enabled")
	kafkaConfig.Brokers = cfg.GetStringSlice("kafka.brokers")
	kafkaConfig.ClientID = cfg.GetString("kafka.client_id")
	kafkaConfig.TopicPrefix = cfg.GetString("kafka.topic_prefix")
	kafkaConfig.TopicPartitions = cfg.GetInt("kafka.topic_partitions")
	kafkaConfig.TopicReplicationFactor = cfg.GetInt("kafka.topic_replication_factor")
	kafkaConfig.TopicMinInSyncReplicas = cfg.GetInt("kafka.topic_min_insync_replicas")
	kafkaConfig.TopicRetentionHours = cfg.GetInt("kafka.topic_retention_hours")
	kafkaConfig.RequiredAcks = strings.ToLower(strings.TrimSpace(cfg.GetString("kafka.required_acks")))
	kafkaConfig.DialTimeoutSeconds = cfg.GetInt("kafka.dial_timeout_seconds")
	kafkaConfig.WriteTimeoutSeconds = cfg.GetInt("kafka.write_timeout_seconds")
	kafkaConfig.ConsumeRetryMaxAttempts = cfg.GetInt("kafka.consume_retry_max_attempts")
	kafkaConfig.ConsumeRetryBackoffMS = cfg.GetInt("kafka.consume_retry_backoff_ms")
	kafkaConfig.ConsumerGroupBalancer = strings.ToLower(strings.TrimSpace(cfg.GetString("kafka.consumer_group_balancer")))
	kafkaConfig.ConsumerHeartbeatSeconds = cfg.GetInt("kafka.consumer_heartbeat_seconds")
	kafkaConfig.ConsumerSessionTimeoutSeconds = cfg.GetInt("kafka.consumer_session_timeout_seconds")
	kafkaConfig.ConsumerRebalanceTimeoutSeconds = cfg.GetInt("kafka.consumer_rebalance_timeout_seconds")

	return kafkaConfig
}

func MessageConfig() Message {
	MustLoad()

	return Message{
		Transport:                   strings.ToLower(strings.TrimSpace(cfg.GetString("message.transport"))),
		RuntimeMode:                 strings.ToLower(strings.TrimSpace(cfg.GetString("message.runtime_mode"))),
		ShadowQueries:               cfg.GetBool("message.shadow_queries"),
		CassandraShadowReads:        cfg.GetBool("message.cassandra_shadow_reads"),
		CassandraReadPercent:        cfg.GetInt("message.cassandra_read_percentage"),
		CassandraReadVerifyPercent:  cfg.GetInt("message.cassandra_read_verify_percentage"),
		CassandraDuplicateHydration: cfg.GetBool("message.cassandra_duplicate_hydration"),
		EnforceDBPermissions:        cfg.GetBool("message.enforce_db_permissions"),
		InboxWriteMode:              strings.ToLower(strings.TrimSpace(cfg.GetString("message.inbox_write_mode"))),
		TimelineNotifyMode:          strings.ToLower(strings.TrimSpace(cfg.GetString("message.timeline_notify_mode"))),
	}
}

func MessageMySQLConfig() MySQL {
	MustLoad()
	return mergeMySQLConfig(MySQLConfig(), MySQL{
		Host: cfg.GetString("message.mysql.host"), Port: cfg.GetInt("message.mysql.port"),
		User: cfg.GetString("message.mysql.user"), Password: cfg.GetString("message.mysql.password"),
		DBName: cfg.GetString("message.mysql.dbname"),
	})
}

func SearchConfig() Search {
	MustLoad()
	return Search{Enabled: cfg.GetBool("search.enabled")}
}

func SearchMySQLConfig() MySQL {
	MustLoad()
	return mergeMySQLConfig(MySQLConfig(), MySQL{
		Host: cfg.GetString("search.mysql.host"), Port: cfg.GetInt("search.mysql.port"),
		User: cfg.GetString("search.mysql.user"), Password: cfg.GetString("search.mysql.password"),
		DBName: cfg.GetString("search.mysql.dbname"),
	})
}

func SyncConfig() Sync {
	MustLoad()
	return Sync{
		Transport:                strings.ToLower(strings.TrimSpace(cfg.GetString("sync.transport"))),
		ShadowQueries:            cfg.GetBool("sync.shadow_queries"),
		ProjectorEnabled:         cfg.GetBool("sync.projector_enabled"),
		EnforceDBPermissions:     cfg.GetBool("sync.enforce_db_permissions"),
		CassandraShadowHydration: cfg.GetBool("sync.cassandra_shadow_hydration"),
	}
}

func SyncMySQLConfig() MySQL {
	MustLoad()
	return mergeMySQLConfig(MySQLConfig(), MySQL{
		Host: cfg.GetString("sync.mysql.host"), Port: cfg.GetInt("sync.mysql.port"),
		User: cfg.GetString("sync.mysql.user"), Password: cfg.GetString("sync.mysql.password"),
		DBName: cfg.GetString("sync.mysql.dbname"),
	})
}

func mergeMySQLConfig(result, override MySQL) MySQL {
	if strings.TrimSpace(override.Host) != "" {
		result.Host = override.Host
	}
	if override.Port != 0 {
		result.Port = override.Port
	}
	if strings.TrimSpace(override.User) != "" {
		result.User = override.User
	}
	if override.Password != "" {
		result.Password = override.Password
	}
	if strings.TrimSpace(override.DBName) != "" {
		result.DBName = override.DBName
	}
	return result
}

func InternalRPCConfig() InternalRPC {
	MustLoad()

	var internalRPC InternalRPC
	if err := cfg.UnmarshalKey("internal_rpc", &internalRPC); err != nil {
		panic(fmt.Errorf("unmarshal internal rpc config: %w", err))
	}
	internalRPC.Enabled = cfg.GetBool("internal_rpc.enabled")
	internalRPC.SharedSecret = strings.TrimSpace(cfg.GetString("internal_rpc.shared_secret"))
	internalRPC.CoreListenAddress = strings.TrimSpace(cfg.GetString("internal_rpc.core_listen_address"))
	internalRPC.CoreTarget = strings.TrimSpace(cfg.GetString("internal_rpc.core_target"))
	internalRPC.MessageListenAddress = strings.TrimSpace(cfg.GetString("internal_rpc.message_listen_address"))
	internalRPC.MessageTarget = strings.TrimSpace(cfg.GetString("internal_rpc.message_target"))
	internalRPC.SearchListenAddress = strings.TrimSpace(cfg.GetString("internal_rpc.search_listen_address"))
	internalRPC.SearchTarget = strings.TrimSpace(cfg.GetString("internal_rpc.search_target"))
	internalRPC.SyncListenAddress = strings.TrimSpace(cfg.GetString("internal_rpc.sync_listen_address"))
	internalRPC.SyncTarget = strings.TrimSpace(cfg.GetString("internal_rpc.sync_target"))
	internalRPC.DeliveryObservationEnabled = cfg.GetBool("internal_rpc.delivery_observation_enabled")
	internalRPC.DeliveryObservationListenAddress = strings.TrimSpace(cfg.GetString("internal_rpc.delivery_observation_listen_address"))
	internalRPC.DeliveryObservationCapacity = cfg.GetInt("internal_rpc.delivery_observation_capacity")
	internalRPC.DeliveryObservationRetryAfterMS = cfg.GetInt("internal_rpc.delivery_observation_retry_after_ms")
	internalRPC.DeliveryPrimaryEnabled = cfg.GetBool("internal_rpc.delivery_primary_enabled")
	internalRPC.DeliveryPrimaryReplayCapacity = cfg.GetInt("internal_rpc.delivery_primary_replay_capacity")
	internalRPC.DialTimeoutSeconds = cfg.GetInt("internal_rpc.dial_timeout_seconds")
	internalRPC.ShutdownTimeoutSeconds = cfg.GetInt("internal_rpc.shutdown_timeout_seconds")
	internalRPC.TLSEnabled = cfg.GetBool("internal_rpc.tls_enabled")
	internalRPC.TLSCertFile = strings.TrimSpace(cfg.GetString("internal_rpc.tls_cert_file"))
	internalRPC.TLSKeyFile = strings.TrimSpace(cfg.GetString("internal_rpc.tls_key_file"))
	internalRPC.TLSCAFile = strings.TrimSpace(cfg.GetString("internal_rpc.tls_ca_file"))
	internalRPC.TLSServerName = strings.TrimSpace(cfg.GetString("internal_rpc.tls_server_name"))

	return internalRPC
}

func StorageConfig() Storage {
	MustLoad()

	var storageConfig Storage
	if err := cfg.UnmarshalKey("storage", &storageConfig); err != nil {
		panic(fmt.Errorf("unmarshal storage config: %w", err))
	}
	storageConfig.Enabled = cfg.GetBool("storage.enabled")
	storageConfig.Provider = cfg.GetString("storage.provider")
	storageConfig.Endpoint = cfg.GetString("storage.endpoint")
	storageConfig.PresignEndpoint = cfg.GetString("storage.presign_endpoint")
	storageConfig.AccessKey = cfg.GetString("storage.access_key")
	storageConfig.SecretKey = cfg.GetString("storage.secret_key")
	storageConfig.UseSSL = cfg.GetBool("storage.use_ssl")
	storageConfig.Bucket = cfg.GetString("storage.bucket")
	storageConfig.SearchArchiveBucket = cfg.GetString("storage.search_archive_bucket")
	storageConfig.SearchArchiveRetentionDays = cfg.GetInt("storage.search_archive_retention_days")
	storageConfig.MessageArchiveBucket = cfg.GetString("storage.message_archive_bucket")
	storageConfig.MessageArchiveRetentionDays = cfg.GetInt("storage.message_archive_retention_days")
	storageConfig.ArtifactEnabled = cfg.GetBool("storage.artifact_enabled")
	storageConfig.ArtifactEndpoint = cfg.GetString("storage.artifact_endpoint")
	storageConfig.ArtifactAccessKey = cfg.GetString("storage.artifact_access_key")
	storageConfig.ArtifactSecretKey = cfg.GetString("storage.artifact_secret_key")
	storageConfig.ArtifactUseSSL = cfg.GetBool("storage.artifact_use_ssl")
	storageConfig.ArtifactBucket = cfg.GetString("storage.artifact_bucket")
	storageConfig.ArtifactAuditAccessKey = cfg.GetString("storage.artifact_audit_access_key")
	storageConfig.ArtifactAuditSecretKey = cfg.GetString("storage.artifact_audit_secret_key")
	storageConfig.ArtifactMaintenanceAccessKey = cfg.GetString("storage.artifact_maintenance_access_key")
	storageConfig.ArtifactMaintenanceSecretKey = cfg.GetString("storage.artifact_maintenance_secret_key")
	storageConfig.PublicBaseURL = cfg.GetString("storage.public_base_url")
	storageConfig.FileMaxSizeMB = cfg.GetInt64("storage.file_max_size_mb")
	storageConfig.DownloadURLTTLMinutes = cfg.GetInt("storage.download_url_ttl_minutes")

	return storageConfig
}

func RateLimitConfig() RateLimit {
	MustLoad()

	var rateLimitConfig RateLimit
	if err := cfg.UnmarshalKey("rate_limit", &rateLimitConfig); err != nil {
		panic(fmt.Errorf("unmarshal rate limit config: %w", err))
	}
	rateLimitConfig.Enabled = cfg.GetBool("rate_limit.enabled")
	rateLimitConfig.RegisterLimit = cfg.GetInt("rate_limit.register_limit")
	rateLimitConfig.RegisterWindowSeconds = cfg.GetInt("rate_limit.register_window_seconds")
	rateLimitConfig.LoginLimit = cfg.GetInt("rate_limit.login_limit")
	rateLimitConfig.LoginWindowSeconds = cfg.GetInt("rate_limit.login_window_seconds")
	rateLimitConfig.MessageLimit = cfg.GetInt("rate_limit.message_limit")
	rateLimitConfig.MessageWindowSeconds = cfg.GetInt("rate_limit.message_window_seconds")
	rateLimitConfig.FileUploadLimit = cfg.GetInt("rate_limit.file_upload_limit")
	rateLimitConfig.FileUploadWindowSeconds = cfg.GetInt("rate_limit.file_upload_window_seconds")
	rateLimitConfig.AgentMCPLimit = cfg.GetInt("rate_limit.agent_mcp_limit")
	rateLimitConfig.AgentMCPWindowSeconds = cfg.GetInt("rate_limit.agent_mcp_window_seconds")

	return rateLimitConfig
}

func PresenceConfig() Presence {
	MustLoad()

	var presenceConfig Presence
	if err := cfg.UnmarshalKey("presence", &presenceConfig); err != nil {
		panic(fmt.Errorf("unmarshal presence config: %w", err))
	}
	presenceConfig.Enabled = cfg.GetBool("presence.enabled")
	presenceConfig.NodeID = cfg.GetString("presence.node_id")
	presenceConfig.TTLSeconds = cfg.GetInt("presence.ttl_seconds")

	return presenceConfig
}

func HotGroupConfig() HotGroup {
	MustLoad()

	var hotGroupConfig HotGroup
	if err := cfg.UnmarshalKey("hot_group", &hotGroupConfig); err != nil {
		panic(fmt.Errorf("unmarshal hot group config: %w", err))
	}
	hotGroupConfig.Enabled = cfg.GetBool("hot_group.enabled")
	hotGroupConfig.MemberCountThreshold = cfg.GetInt("hot_group.member_count_threshold")
	hotGroupConfig.MessageThreshold = cfg.GetInt("hot_group.message_threshold")
	hotGroupConfig.WindowSeconds = cfg.GetInt("hot_group.window_seconds")
	hotGroupConfig.CoolingSeconds = cfg.GetInt("hot_group.cooling_seconds")

	return hotGroupConfig
}

func AIConfig() AI {
	MustLoad()

	var aiConfig AI
	if err := cfg.UnmarshalKey("ai", &aiConfig); err != nil {
		panic(fmt.Errorf("unmarshal ai config: %w", err))
	}
	aiConfig.Enabled = cfg.GetBool("ai.enabled")
	aiConfig.RuntimeMode = cfg.GetString("ai.runtime_mode")
	aiConfig.PolicyMode = cfg.GetString("ai.policy_mode")
	aiConfig.Provider = cfg.GetString("ai.provider")
	aiConfig.Model = cfg.GetString("ai.model")
	aiConfig.APIKey = cfg.GetString("ai.api_key")
	aiConfig.BaseURL = cfg.GetString("ai.base_url")
	aiConfig.TimeoutSeconds = cfg.GetInt("ai.timeout_seconds")
	aiConfig.MaxContextMessages = cfg.GetInt("ai.max_context_messages")
	aiConfig.AssistantUUID = cfg.GetString("ai.assistant_uuid")
	aiConfig.AssistantNickname = cfg.GetString("ai.assistant_nickname")
	aiConfig.AssistantTelephone = cfg.GetString("ai.assistant_telephone")
	aiConfig.AssistantEmail = cfg.GetString("ai.assistant_email")
	aiConfig.AssistantAvatar = cfg.GetString("ai.assistant_avatar")
	aiConfig.SystemPrompt = cfg.GetString("ai.system_prompt")
	if mode, err := aiConfig.ResolvedRuntimeMode(); err == nil {
		aiConfig.RuntimeMode = mode
		aiConfig.Enabled = mode != AIRuntimeOff
	}
	if mode, err := aiConfig.ResolvedPolicyMode(); err == nil {
		aiConfig.PolicyMode = mode
	}

	return aiConfig
}

func (a AI) DefaultProvider() AIProvider {
	return AIProvider{
		Name:           strings.TrimSpace(a.Provider),
		Model:          strings.TrimSpace(a.Model),
		APIKey:         strings.TrimSpace(a.APIKey),
		BaseURL:        strings.TrimSpace(a.BaseURL),
		TimeoutSeconds: a.TimeoutSeconds,
	}
}

func Addr() string {
	server := ServerConfig()
	return fmt.Sprintf("%s:%d", server.Host, server.Port)
}

func V() *viper.Viper {
	MustLoad()
	return cfg
}
