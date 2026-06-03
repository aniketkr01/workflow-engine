package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Auth     AuthConfig
	Worker   WorkerConfig
	Telemetry TelemetryConfig
}

type ServerConfig struct {
	HTTPPort    string
	GRPCPort    string
	Environment string
	ShutdownTimeout time.Duration
}

type DatabaseConfig struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type AuthConfig struct {
	JWTSecret     string
	TokenDuration time.Duration
}

type WorkerConfig struct {
	Concurrency  int
	QueueName    string
	DLQName      string
	PollInterval time.Duration
}

type TelemetryConfig struct {
	OTELEndpoint   string
	ServiceName    string
	MetricsEnabled bool
	TracingEnabled bool
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			HTTPPort:        getEnv("HTTP_PORT", "8080"),
			GRPCPort:        getEnv("GRPC_PORT", "9090"),
			Environment:     getEnv("ENVIRONMENT", "development"),
			ShutdownTimeout: getDurationEnv("SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		Database: DatabaseConfig{
			DSN:             getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/workflow_engine?sslmode=disable"),
			MaxOpenConns:    getIntEnv("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getIntEnv("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getDurationEnv("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getIntEnv("REDIS_DB", 0),
		},
		Auth: AuthConfig{
			JWTSecret:     getEnv("JWT_SECRET", "change-me-in-production"),
			TokenDuration: getDurationEnv("JWT_TOKEN_DURATION", 24*time.Hour),
		},
		Worker: WorkerConfig{
			Concurrency:  getIntEnv("WORKER_CONCURRENCY", 10),
			QueueName:    getEnv("TASK_QUEUE_NAME", "workflow:tasks"),
			DLQName:      getEnv("TASK_DLQ_NAME", "workflow:dlq"),
			PollInterval: getDurationEnv("WORKER_POLL_INTERVAL", 500*time.Millisecond),
		},
		Telemetry: TelemetryConfig{
			OTELEndpoint:   getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317"),
			ServiceName:    getEnv("SERVICE_NAME", "workflow-engine"),
			MetricsEnabled: getBoolEnv("METRICS_ENABLED", true),
			TracingEnabled: getBoolEnv("TRACING_ENABLED", true),
		},
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func getIntEnv(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getBoolEnv(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
