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

type Server struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
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
		v.SetDefault("server.host", "0.0.0.0")
		v.SetDefault("server.port", 8080)
		v.SetDefault("auth.token_ttl_hours", 168)

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

func Addr() string {
	server := ServerConfig()
	return fmt.Sprintf("%s:%d", server.Host, server.Port)
}

func V() *viper.Viper {
	MustLoad()
	return cfg
}
