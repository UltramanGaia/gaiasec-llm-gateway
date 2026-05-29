package config

import "testing"

func TestLoadConfigDefaultsUpstreamConnectionPool(t *testing.T) {
	t.Setenv("UPSTREAM_MAX_IDLE_CONNS", "")
	t.Setenv("UPSTREAM_MAX_IDLE_CONNS_PER_HOST", "")
	t.Setenv("UPSTREAM_MAX_CONNS_PER_HOST", "")

	cfg := LoadConfig()

	if cfg.UpstreamMaxIdleConns != 8000 {
		t.Fatalf("expected default UpstreamMaxIdleConns to be 8000, got %d", cfg.UpstreamMaxIdleConns)
	}
	if cfg.UpstreamMaxIdleConnsPerHost != 5000 {
		t.Fatalf("expected default UpstreamMaxIdleConnsPerHost to be 5000, got %d", cfg.UpstreamMaxIdleConnsPerHost)
	}
	if cfg.UpstreamMaxConnsPerHost != 5000 {
		t.Fatalf("expected default UpstreamMaxConnsPerHost to be 5000, got %d", cfg.UpstreamMaxConnsPerHost)
	}
}

func TestLoadConfigReadsUpstreamConnectionPoolFromEnv(t *testing.T) {
	t.Setenv("UPSTREAM_MAX_IDLE_CONNS", "9000")
	t.Setenv("UPSTREAM_MAX_IDLE_CONNS_PER_HOST", "6000")
	t.Setenv("UPSTREAM_MAX_CONNS_PER_HOST", "7000")

	cfg := LoadConfig()

	if cfg.UpstreamMaxIdleConns != 9000 {
		t.Fatalf("expected UpstreamMaxIdleConns from env, got %d", cfg.UpstreamMaxIdleConns)
	}
	if cfg.UpstreamMaxIdleConnsPerHost != 6000 {
		t.Fatalf("expected UpstreamMaxIdleConnsPerHost from env, got %d", cfg.UpstreamMaxIdleConnsPerHost)
	}
	if cfg.UpstreamMaxConnsPerHost != 7000 {
		t.Fatalf("expected UpstreamMaxConnsPerHost from env, got %d", cfg.UpstreamMaxConnsPerHost)
	}
}
