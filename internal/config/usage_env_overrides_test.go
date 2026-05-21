package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigOptional_UsageQueueEnvOverrides(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("usage-statistics-enabled: false\nredis-usage-queue-retention-seconds: 60\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("CPA_USAGE_STATISTICS_ENABLED", "true")
	t.Setenv("CPA_REDIS_USAGE_QUEUE_RETENTION_SECONDS", "120")

	cfg, err := LoadConfigOptional(configPath, false)
	if err != nil {
		t.Fatalf("LoadConfigOptional() error = %v", err)
	}
	if !cfg.UsageStatisticsEnabled {
		t.Fatalf("expected usage statistics to be enabled by env override")
	}
	if cfg.RedisUsageQueueRetentionSeconds != 120 {
		t.Fatalf("expected redis usage queue retention 120, got %d", cfg.RedisUsageQueueRetentionSeconds)
	}
}

func TestLoadConfigOptional_InvalidUsageQueueEnvOverride(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("usage-statistics-enabled: false\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("CPA_USAGE_STATISTICS_ENABLED", "maybe")

	if _, err := LoadConfigOptional(configPath, false); err == nil {
		t.Fatalf("expected invalid usage statistics env override to fail")
	}
}
