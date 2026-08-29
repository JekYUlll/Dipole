package store

import (
	"github.com/redis/go-redis/v9"

	"github.com/JekYUlll/Dipole/internal/config"
	platformcache "github.com/JekYUlll/Dipole/internal/platform/cache"
)

// RDB remains available for legacy integrations. New services use
// internal/platform/cache directly so Redis ownership stays in the platform layer.
var RDB *redis.Client

func InitRedis() error {
	if err := platformcache.InitRedisWithConfig(config.RedisConfig()); err != nil {
		return err
	}
	RDB = platformcache.RDB
	return nil
}

func NewRedisClient(cfg config.Redis) (*redis.Client, error) {
	return platformcache.NewRedisClient(cfg)
}
