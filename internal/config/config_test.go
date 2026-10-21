package config

import (
	"os"
	"testing"
)

func TestLoadOrchestratorConfig_FallbackMasterSecretKey(t *testing.T) {
	// Clear MASTER_SECRET_KEY to simulate fresh unconfigured Render deployment
	origKey := os.Getenv("MASTER_SECRET_KEY")
	origEnv := os.Getenv("ENV")
	defer func() {
		_ = os.Setenv("MASTER_SECRET_KEY", origKey)
		_ = os.Setenv("ENV", origEnv)
	}()

	_ = os.Unsetenv("MASTER_SECRET_KEY")
	_ = os.Setenv("ENV", "production")

	cfg, err := LoadOrchestratorConfig()
	if err != nil {
		t.Fatalf("expected nil error with resilient fallback, got: %v", err)
	}

	const wellKnownFallback = "nexacampus_secret_master_key_super_secure_32bytes_long"
	if cfg.MasterSecretKey == wellKnownFallback {
		t.Errorf("production must never fall back to the well-known shared secret")
	}
	if len(cfg.MasterSecretKey) < 32 {
		t.Errorf("expected fallback key length >= 32, got %d", len(cfg.MasterSecretKey))
	}

	// Two consecutive loads must not produce the same ephemeral secret.
	second, err := LoadOrchestratorConfig()
	if err != nil {
		t.Fatalf("expected nil error on second load, got: %v", err)
	}
	if second.MasterSecretKey == cfg.MasterSecretKey {
		t.Errorf("expected a fresh random ephemeral secret on each load")
	}
}

func TestLoadOrchestratorConfig_DevelopmentFallbackIsStable(t *testing.T) {
	origKey := os.Getenv("MASTER_SECRET_KEY")
	origEnv := os.Getenv("ENV")
	defer func() {
		_ = os.Setenv("MASTER_SECRET_KEY", origKey)
		_ = os.Setenv("ENV", origEnv)
	}()

	_ = os.Unsetenv("MASTER_SECRET_KEY")
	_ = os.Setenv("ENV", "development")

	cfg, err := LoadOrchestratorConfig()
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}
	if cfg.MasterSecretKey != "nexacampus_secret_master_key_super_secure_32bytes_long" {
		t.Errorf("expected stable development fallback key, got %q", cfg.MasterSecretKey)
	}
}

func TestLoadOrchestratorConfig_CustomMasterSecretKey(t *testing.T) {
	customKey := "my-custom-production-secret-key-at-least-32-chars!"
	origKey := os.Getenv("MASTER_SECRET_KEY")
	defer func() {
		_ = os.Setenv("MASTER_SECRET_KEY", origKey)
	}()

	_ = os.Setenv("MASTER_SECRET_KEY", customKey)

	cfg, err := LoadOrchestratorConfig()
	if err != nil {
		t.Fatalf("unexpected error loading config with custom key: %v", err)
	}

	if cfg.MasterSecretKey != customKey {
		t.Errorf("expected custom key %q, got %q", customKey, cfg.MasterSecretKey)
	}
}

func TestLoadOrchestratorConfig_TursoDatabaseConfig(t *testing.T) {
	testURL := "libsql://test-cluster.turso.io"
	testToken := "test-auth-token-xyz"

	origURL := os.Getenv("TURSO_DATABASE_URL")
	origToken := os.Getenv("TURSO_AUTH_TOKEN")
	defer func() {
		_ = os.Setenv("TURSO_DATABASE_URL", origURL)
		_ = os.Setenv("TURSO_AUTH_TOKEN", origToken)
	}()

	_ = os.Setenv("TURSO_DATABASE_URL", testURL)
	_ = os.Setenv("TURSO_AUTH_TOKEN", testToken)

	cfg, err := LoadOrchestratorConfig()
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.TursoDatabaseURL != testURL {
		t.Errorf("expected TursoDatabaseURL %q, got %q", testURL, cfg.TursoDatabaseURL)
	}
	if cfg.TursoAuthToken != testToken {
		t.Errorf("expected TursoAuthToken %q, got %q", testToken, cfg.TursoAuthToken)
	}
}
