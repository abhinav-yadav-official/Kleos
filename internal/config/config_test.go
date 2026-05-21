package config

import "testing"

func TestLoadReadsConfiguredValues(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("APP_PORT", "9090")
	t.Setenv("DB_DSN", "postgres://kleos:kleos@localhost:5433/kleos?sslmode=disable")
	t.Setenv("REDIS_ADDR", "localhost:6380")
	t.Setenv("REDIS_PASSWORD", "secret")
	t.Setenv("LOG_DIR", "/tmp/kleos-logs")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("JWT_SECRET", "jwt-secret")
	t.Setenv("JWT_ACCESS_TTL", "10m")
	t.Setenv("JWT_REFRESH_TTL", "24h")

	cfg := Load()

	if cfg.AppEnv != "test" {
		t.Fatalf("AppEnv = %q, want test", cfg.AppEnv)
	}
	if cfg.AppPort != "9090" {
		t.Fatalf("AppPort = %q, want 9090", cfg.AppPort)
	}
	if cfg.DBDSN == "" {
		t.Fatal("DBDSN should be populated")
	}
	if cfg.RedisAddr != "localhost:6380" {
		t.Fatalf("RedisAddr = %q, want localhost:6380", cfg.RedisAddr)
	}
	if cfg.RedisPassword != "secret" {
		t.Fatalf("RedisPassword = %q, want secret", cfg.RedisPassword)
	}
	if cfg.LogDir != "/tmp/kleos-logs" {
		t.Fatalf("LogDir = %q, want /tmp/kleos-logs", cfg.LogDir)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want debug", cfg.LogLevel)
	}
	if cfg.JWTSecret != "jwt-secret" {
		t.Fatalf("JWTSecret = %q, want jwt-secret", cfg.JWTSecret)
	}
	if cfg.JWTAccessTTL != "10m" {
		t.Fatalf("JWTAccessTTL = %q, want 10m", cfg.JWTAccessTTL)
	}
	if cfg.JWTRefreshTTL != "24h" {
		t.Fatalf("JWTRefreshTTL = %q, want 24h", cfg.JWTRefreshTTL)
	}
}

func TestLoadDefaultsForLocalDevelopment(t *testing.T) {
	cfg := Load()

	if cfg.AppEnv != "development" {
		t.Fatalf("AppEnv = %q, want development", cfg.AppEnv)
	}
	if cfg.AppPort != "8080" {
		t.Fatalf("AppPort = %q, want 8080", cfg.AppPort)
	}
	if cfg.RedisAddr != "localhost:6380" {
		t.Fatalf("RedisAddr = %q, want localhost:6380", cfg.RedisAddr)
	}
	if cfg.LogDir != "./logs" {
		t.Fatalf("LogDir = %q, want ./logs", cfg.LogDir)
	}
}
