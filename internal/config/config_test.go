package config

import (
	"os"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Use a temp dir with no config file to force defaults
	dir := t.TempDir()

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Fatalf("expected default server.port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Scheduler.Concurrency != 10 {
		t.Fatalf("expected default scheduler.concurrency 10, got %d", cfg.Scheduler.Concurrency)
	}
	if cfg.Prometheus.Port != 9090 {
		t.Fatalf("expected default prometheus.port 9090, got %d", cfg.Prometheus.Port)
	}
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	// Ensure clean env
	os.Unsetenv("SERVER_PORT")
	os.Unsetenv("SCHEDULER_CONCURRENCY")

	// Override via env
	if err := os.Setenv("SERVER_PORT", "9999"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("SCHEDULER_CONCURRENCY", "3"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Unsetenv("SERVER_PORT")
		os.Unsetenv("SCHEDULER_CONCURRENCY")
	}()

	// No file, should rely on defaults + env
	dir := t.TempDir()
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.Server.Port != 9999 {
		t.Fatalf("expected server.port 9999 from env, got %d", cfg.Server.Port)
	}
	if cfg.Scheduler.Concurrency != 3 {
		t.Fatalf("expected scheduler.concurrency 3 from env, got %d", cfg.Scheduler.Concurrency)
	}
}

