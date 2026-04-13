package config

import (
	"fmt"
	"strings"
	"sync"

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

type TLS struct {
	Enabled  bool   `mapstructure:"enabled"`
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
}

type MySQL struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
}

type Redis struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type Auth struct {
	TokenTTLHours int    `mapstructure:"token_ttl_hours"`
	JWTSecret     string `mapstructure:"jwt_secret"`
	JWTIssuer     string `mapstructure:"jwt_issuer"`
}

type Kafka struct {
	Enabled                 bool     `mapstructure:"enabled"`
	Brokers                 []string `mapstructure:"brokers"`
	ClientID                string   `mapstructure:"client_id"`
	TopicPrefix             string   `mapstructure:"topic_prefix"`
	DialTimeoutSeconds      int      `mapstructure:"dial_timeout_seconds"`
	WriteTimeoutSeconds     int      `mapstructure:"write_timeout_seconds"`
	ConsumeRetryMaxAttempts int      `mapstructure:"consume_retry_max_attempts"`
	ConsumeRetryBackoffMS   int      `mapstructure:"consume_retry_backoff_ms"`
}

type Storage struct {
	Enabled       bool   `mapstructure:"enabled"`
	Provider      string `mapstructure:"provider"`
	Endpoint      string `mapstructure:"endpoint"`
	AccessKey     string `mapstructure:"access_key"`
	SecretKey     string `mapstructure:"secret_key"`
	UseSSL        bool   `mapstructure:"use_ssl"`
	Bucket        string `mapstructure:"bucket"`
	PublicBaseURL string `mapstructure:"public_base_url"`
	FileMaxSizeMB int64  `mapstructure:"file_max_size_mb"`
}

var (
	cfg     *viper.Viper
	loadErr error
	once    sync.Once
)

func Load() error {
	once.Do(func() {
		v := viper.New()
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("configs")
		v.AddConfigPath(".")

		v.SetEnvPrefix("DIPOLE")
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
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
		v.SetDefault("tls.enabled", false)
		v.SetDefault("tls.cert_file", "certs/local/dipole-local.pem")
		v.SetDefault("tls.key_file", "certs/local/dipole-local-key.pem")
		v.SetDefault("auth.token_ttl_hours", 168)
		v.SetDefault("auth.jwt_secret", "dipole-dev-jwt-secret-change-me")
		v.SetDefault("auth.jwt_issuer", "dipole")
		v.SetDefault("kafka.enabled", false)
		v.SetDefault("kafka.brokers", []string{"127.0.0.1:9092"})
		v.SetDefault("kafka.client_id", "dipole")
		v.SetDefault("kafka.topic_prefix", "dipole")
		v.SetDefault("kafka.dial_timeout_seconds", 5)
		v.SetDefault("kafka.write_timeout_seconds", 5)
		v.SetDefault("kafka.consume_retry_max_attempts", 3)
		v.SetDefault("kafka.consume_retry_backoff_ms", 500)
		v.SetDefault("storage.enabled", false)
		v.SetDefault("storage.provider", "minio")
		v.SetDefault("storage.endpoint", "127.0.0.1:9000")
		v.SetDefault("storage.access_key", "dipoleminio")
		v.SetDefault("storage.secret_key", "dipoleminiopass")
		v.SetDefault("storage.use_ssl", false)
		v.SetDefault("storage.bucket", "dipole-files")
		v.SetDefault("storage.public_base_url", "http://127.0.0.1:9000/dipole-files")
		v.SetDefault("storage.file_max_size_mb", 50)
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
			"tls.enabled",
			"tls.cert_file",
			"tls.key_file",
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
			"kafka.enabled",
			"kafka.brokers",
			"kafka.client_id",
			"kafka.topic_prefix",
			"kafka.dial_timeout_seconds",
			"kafka.write_timeout_seconds",
			"kafka.consume_retry_max_attempts",
			"kafka.consume_retry_backoff_ms",
			"storage.enabled",
			"storage.provider",
			"storage.endpoint",
			"storage.access_key",
			"storage.secret_key",
			"storage.use_ssl",
			"storage.bucket",
			"storage.public_base_url",
			"storage.file_max_size_mb",
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

func RedisConfig() Redis {
	MustLoad()

	var redis Redis
	if err := cfg.UnmarshalKey("redis", &redis); err != nil {
		panic(fmt.Errorf("unmarshal redis config: %w", err))
	}

	return redis
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
	kafkaConfig.DialTimeoutSeconds = cfg.GetInt("kafka.dial_timeout_seconds")
	kafkaConfig.WriteTimeoutSeconds = cfg.GetInt("kafka.write_timeout_seconds")
	kafkaConfig.ConsumeRetryMaxAttempts = cfg.GetInt("kafka.consume_retry_max_attempts")
	kafkaConfig.ConsumeRetryBackoffMS = cfg.GetInt("kafka.consume_retry_backoff_ms")

	return kafkaConfig
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
	storageConfig.AccessKey = cfg.GetString("storage.access_key")
	storageConfig.SecretKey = cfg.GetString("storage.secret_key")
	storageConfig.UseSSL = cfg.GetBool("storage.use_ssl")
	storageConfig.Bucket = cfg.GetString("storage.bucket")
	storageConfig.PublicBaseURL = cfg.GetString("storage.public_base_url")
	storageConfig.FileMaxSizeMB = cfg.GetInt64("storage.file_max_size_mb")

	return storageConfig
}

func Addr() string {
	server := ServerConfig()
	return fmt.Sprintf("%s:%d", server.Host, server.Port)
}

func V() *viper.Viper {
	MustLoad()
	return cfg
}
