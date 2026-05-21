package config

import "os"

type Config struct {
	AppEnv        string
	AppPort       string
	AppBaseURL    string
	DBDSN         string
	RedisAddr     string
	RedisPassword string
	LogDir        string
	LogLevel      string
	JWTSecret     string
	JWTAccessTTL  string
	JWTRefreshTTL string
	SMTPKey       string
	ResumeStorage string
}

func Load() Config {
	return Config{
		AppEnv:        getenv("APP_ENV", "development"),
		AppPort:       getenv("APP_PORT", "8080"),
		AppBaseURL:    getenv("APP_BASE_URL", "http://localhost:8080"),
		DBDSN:         getenv("DB_DSN", "postgres://kleos:kleos@localhost:5433/kleos?sslmode=disable"),
		RedisAddr:     getenv("REDIS_ADDR", "localhost:6380"),
		RedisPassword: getenv("REDIS_PASSWORD", ""),
		LogDir:        getenv("LOG_DIR", "./logs"),
		LogLevel:      getenv("LOG_LEVEL", "info"),
		JWTSecret:     getenv("JWT_SECRET", "dev-secret-change-me"),
		JWTAccessTTL:  getenv("JWT_ACCESS_TTL", "15m"),
		JWTRefreshTTL: getenv("JWT_REFRESH_TTL", "720h"),
		SMTPKey:       getenv("SMTP_CRED_ENCRYPTION_KEY", "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"),
		ResumeStorage: getenv("RESUME_STORAGE_DIR", "./data/resumes"),
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
