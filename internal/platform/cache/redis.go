package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/JekYUlll/Dipole/internal/config"
)

var RDB *redis.Client

func InitRedis() error {
	return InitRedisWithConfig(config.RedisConfig())
}

func InitRedisWithConfig(cfg config.Redis) error {
	client, err := NewRedisClient(cfg)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return fmt.Errorf("ping redis: %w", err)
	}

	RDB = client
	return nil
}

func NewRedisClient(cfg config.Redis) (*redis.Client, error) {
	const timeout = 3 * time.Second

	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		mode = "single"
	}

	switch mode {
	case "single":
		host := strings.TrimSpace(cfg.Host)
		if host == "" || cfg.Port <= 0 {
			return nil, fmt.Errorf("redis single mode requires host and positive port")
		}
		return redis.NewClient(&redis.Options{
			Addr:         fmt.Sprintf("%s:%d", host, cfg.Port),
			Password:     cfg.Password,
			DB:           cfg.DB,
			DialTimeout:  timeout,
			ReadTimeout:  timeout,
			WriteTimeout: timeout,
		}), nil
	case "sentinel":
		masterName := strings.TrimSpace(cfg.SentinelMasterName)
		addresses := normalizedRedisAddresses(cfg.SentinelAddresses)
		if masterName == "" || len(addresses) == 0 {
			return nil, fmt.Errorf("redis sentinel mode requires sentinel_master_name and sentinel_addresses")
		}
		return redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:       masterName,
			SentinelAddrs:    addresses,
			SentinelPassword: cfg.SentinelPassword,
			Password:         cfg.Password,
			DB:               cfg.DB,
			DialTimeout:      timeout,
			ReadTimeout:      timeout,
			WriteTimeout:     timeout,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported redis mode %q", cfg.Mode)
	}
}

func normalizedRedisAddresses(addresses []string) []string {
	normalized := make([]string, 0, len(addresses))
	for _, address := range addresses {
		address = strings.TrimSpace(address)
		if address != "" {
			normalized = append(normalized, address)
		}
	}
	return normalized
}
