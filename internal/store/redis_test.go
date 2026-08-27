package store

import (
	"testing"

	"github.com/JekYUlll/Dipole/internal/config"
)

func TestNewRedisClientKeepsSingleNodeCompatibility(t *testing.T) {
	client, err := NewRedisClient(config.Redis{
		Host: "127.0.0.1",
		Port: 6379,
		DB:   2,
	})
	if err != nil {
		t.Fatalf("create single-node client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if got := client.Options().Addr; got != "127.0.0.1:6379" {
		t.Fatalf("expected single-node address, got %q", got)
	}
	if got := client.Options().DB; got != 2 {
		t.Fatalf("expected database 2, got %d", got)
	}
}

func TestNewRedisClientBuildsSentinelFailoverClient(t *testing.T) {
	client, err := NewRedisClient(config.Redis{
		Mode:               "sentinel",
		Password:           "data-password",
		DB:                 3,
		SentinelMasterName: "dipole-master",
		SentinelAddresses:  []string{" sentinel-1:26379 ", "sentinel-2:26379"},
		SentinelPassword:   "sentinel-password",
	})
	if err != nil {
		t.Fatalf("create Sentinel client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if got := client.Options().Addr; got != "FailoverClient" {
		t.Fatalf("expected go-redis failover client, got address %q", got)
	}
	if got := client.Options().Password; got != "data-password" {
		t.Fatalf("expected Redis data password to be preserved")
	}
	if got := client.Options().DB; got != 3 {
		t.Fatalf("expected database 3, got %d", got)
	}
}

func TestNewRedisClientRejectsInvalidTopology(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Redis
	}{
		{name: "unsupported mode", cfg: config.Redis{Mode: "cluster"}},
		{name: "single missing host", cfg: config.Redis{Mode: "single", Port: 6379}},
		{name: "single missing port", cfg: config.Redis{Mode: "single", Host: "redis"}},
		{name: "sentinel missing master", cfg: config.Redis{Mode: "sentinel", SentinelAddresses: []string{"sentinel:26379"}}},
		{name: "sentinel missing addresses", cfg: config.Redis{Mode: "sentinel", SentinelMasterName: "dipole-master"}},
		{name: "sentinel blank address", cfg: config.Redis{Mode: "sentinel", SentinelMasterName: "dipole-master", SentinelAddresses: []string{" "}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewRedisClient(tt.cfg)
			if client != nil {
				_ = client.Close()
			}
			if err == nil {
				t.Fatal("expected invalid Redis topology to fail")
			}
		})
	}
}
