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
	Level       string `mapstructure:"level"`
	Format      string `mapstructure:"format"`
	Development bool   `mapstructure:"development"`
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
	TokenTTLHours int `mapstructure:"token_ttl_hours"`
}

type Kafka struct {
	Enabled             bool     `mapstructure:"enabled"`
	Brokers             []string `mapstructure:"brokers"`
	ClientID            string   `mapstructure:"client_id"`
	TopicPrefix         string   `mapstructure:"topic_prefix"`
	DialTimeoutSeconds  int      `mapstructure:"dial_timeout_seconds"`
	WriteTimeoutSeconds int      `mapstructure:"write_timeout_seconds"`
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
		v.SetDefault("server.host", "0.0.0.0")
		v.SetDefault("server.port", 8080)
		v.SetDefault("tls.enabled", false)
		v.SetDefault("tls.cert_file", "certs/local/dipole-local.pem")
		v.SetDefault("tls.key_file", "certs/local/dipole-local-key.pem")
		v.SetDefault("auth.token_ttl_hours", 168)
		v.SetDefault("kafka.enabled", false)
		v.SetDefault("kafka.brokers", []string{"127.0.0.1:9092"})
		v.SetDefault("kafka.client_id", "dipole")
		v.SetDefault("kafka.topic_prefix", "dipole")
		v.SetDefault("kafka.dial_timeout_seconds", 5)
		v.SetDefault("kafka.write_timeout_seconds", 5)

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

	return auth
}

func KafkaConfig() Kafka {
	MustLoad()

	var kafkaConfig Kafka
	if err := cfg.UnmarshalKey("kafka", &kafkaConfig); err != nil {
		panic(fmt.Errorf("unmarshal kafka config: %w", err))
	}

	return kafkaConfig
}

func Addr() string {
	server := ServerConfig()
	return fmt.Sprintf("%s:%d", server.Host, server.Port)
}

func V() *viper.Viper {
	MustLoad()
	return cfg
}
