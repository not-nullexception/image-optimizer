package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds the complete application configuration.
type Config struct {
	Server        ServerConfig
	Database      DatabaseConfig
	MinIO         MinIOConfig
	RabbitMQ      RabbitMQConfig
	Worker        WorkerConfig
	Log           LogConfig
	Metrics       MetricsConfig
	Tracing       TracingConfig
	Observability ObservabilityConfig
}

type ServerConfig struct {
	Host string
	Port int
	Mode string
}

type DatabaseConfig struct {
	Host           string
	Port           int
	User           string
	Password       string
	DBName         string
	SSLMode        string
	MaxConnections int
	MinConnections int
}

type MinIOConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	SSL       bool
	Location  string
	URLExpiry time.Duration
}

type RabbitMQConfig struct {
	Host        string
	Port        int
	User        string
	Password    string
	Queue       string
	Exchange    string
	RoutingKey  string
	ConsumerTag string
}

type WorkerConfig struct {
	Count       int
	MaxWorkers  int
	MetricsPort int
}

type LogConfig struct {
	Level       string
	Format      string
	ServiceName string
	OutputJSON  bool
}

type MetricsConfig struct {
	Enabled bool
	Port    int
}

type TracingConfig struct {
	Enabled        bool
	OTLPEndpoint   string
	ServiceName    string
	ServiceVersion string
	Environment    string
}

type ObservabilityConfig struct {
	MetricsEndpoint string
	TracingEndpoint string
	ProfilerEnabled bool
}

// ConnectionString generates the connection string for PostgreSQL.
func (c *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.DBName, c.SSLMode)
}

// RabbitMQURL generates the connection string for RabbitMQ.
func (c *RabbitMQConfig) RabbitMQURL() string {
	return fmt.Sprintf("amqp://%s:%s@%s:%d/",
		c.User, c.Password, c.Host, c.Port)
}

// Load reads the application configuration from the .env file (if exists)
// and from OS environment variables, applying default values if variables are not set.
// In production, it is recommended to supply configuration via environment variables.
func Load() (*Config, error) {
	// Load the .env file into OS environment variables.
	// If the file doesn't exist or there's an error, a warning is printed.
	if err := godotenv.Load(".env"); err != nil {
		fmt.Println("Warning: .env file not found or error loading it; relying solely on OS environment variables and defaults")
	}

	cfg := &Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnvAsInt("SERVER_PORT", 8080),
			Mode: getEnv("GIN_MODE", "release"),
		},
		Database: DatabaseConfig{
			Host:           getEnv("DATABASE_HOST", "localhost"),
			Port:           getEnvAsInt("DATABASE_PORT", 5432),
			User:           getEnv("DATABASE_USER", "postgres"),
			Password:       getEnv("DATABASE_PASSWORD", "postgres"),
			DBName:         getEnv("DATABASE_DBNAME", "image_optimizer"),
			SSLMode:        getEnv("DATABASE_SSL_MODE", "disable"),
			MaxConnections: getEnvAsInt("DATABASE_MAX_CONNECTIONS", 10),
			MinConnections: getEnvAsInt("DATABASE_MIN_CONNECTIONS", 2),
		},
		MinIO: MinIOConfig{
			Endpoint:  getEnv("MINIO_ENDPOINT", "localhost:9000"),
			AccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
			SecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin"),
			Bucket:    getEnv("MINIO_BUCKET", "images"),
			SSL:       getEnvAsBool("MINIO_SSL", false),
			Location:  getEnv("MINIO_LOCATION", "us-east-1"),
			URLExpiry: getEnvAsDuration("MINIO_URL_EXPIRY", 24*time.Hour),
		},
		RabbitMQ: RabbitMQConfig{
			Host:        getEnv("RABBITMQ_HOST", "rabbitmq"),
			Port:        getEnvAsInt("RABBITMQ_PORT", 5672),
			User:        getEnv("RABBITMQ_USER", "guest"),
			Password:    getEnv("RABBITMQ_PASSWORD", "guest"),
			Queue:       getEnv("RABBITMQ_QUEUE", "image_processing"),
			Exchange:    getEnv("RABBITMQ_EXCHANGE", "image_optimizer"),
			RoutingKey:  getEnv("RABBITMQ_ROUTING_KEY", "image.resize"),
			ConsumerTag: getEnv("RABBITMQ_CONSUMER_TAG", "image_worker"),
		},
		Worker: WorkerConfig{
			Count:       getEnvAsInt("WORKER_COUNT", 4),
			MaxWorkers:  getEnvAsInt("MAX_WORKERS", 10),
			MetricsPort: getEnvAsInt("WORKER_METRICS_PORT", 9091),
		},
		Log: LogConfig{
			Level:       getEnv("LOG_LEVEL", "info"),
			Format:      getEnv("LOG_FORMAT", "json"),
			ServiceName: getEnv("LOG_SERVICENAME", "image-optimizer"),
			OutputJSON:  getEnvAsBool("LOG_JSON", true),
		},
		Metrics: MetricsConfig{
			Enabled: getEnvAsBool("METRICS_ENABLED", true),
			Port:    getEnvAsInt("METRICS_PORT", 9090),
		},
		Tracing: TracingConfig{
			Enabled:        getEnvAsBool("TRACING_ENABLED", true),
			OTLPEndpoint:   getEnv("TRACING_OTLP_ENDPOINT", "otel-collector:4317"),
			ServiceName:    getEnv("TRACING_SERVICE_NAME", "image-optimizer"),
			ServiceVersion: getEnv("TRACING_SERVICE_VERSION", "1.0.0"),
			Environment:    getEnv("TRACING_ENVIRONMENT", "dev"),
		},
		Observability: ObservabilityConfig{
			MetricsEndpoint: getEnv("OBSERVABILITY_METRICS_ENDPOINT", "/metrics"),
			TracingEndpoint: getEnv("OBSERVABILITY_TRACING_ENDPOINT", "/traces"),
			ProfilerEnabled: getEnvAsBool("OBSERVABILITY_PROFILER_ENABLED", false),
		},
	}

	return cfg, nil
}

// getEnv returns the value of the environment variable key if it exists,
// otherwise returns the defaultValue.
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt returns the value of the environment variable key as an integer,
// or returns the defaultValue if conversion fails or the variable is not set.
func getEnvAsInt(key string, defaultValue int) int {
	valStr := getEnv(key, "")
	if val, err := strconv.Atoi(valStr); err == nil {
		return val
	}
	return defaultValue
}

// getEnvAsBool returns the value of the environment variable key as a boolean,
// or returns the defaultValue if conversion fails or the variable is not set.
func getEnvAsBool(key string, defaultValue bool) bool {
	valStr := getEnv(key, "")
	if val, err := strconv.ParseBool(valStr); err == nil {
		return val
	}
	return defaultValue
}

// getEnvAsDuration returns the value of the environment variable key as a time.Duration,
// or returns the defaultValue if conversion fails or the variable is not set.
func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valStr := getEnv(key, "")
	if val, err := time.ParseDuration(valStr); err == nil {
		return val
	}
	return defaultValue
}
