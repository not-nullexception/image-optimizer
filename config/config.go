package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	MinIO    MinIOConfig
	RabbitMQ RabbitMQConfig
	Worker   WorkerConfig
	Log      LogConfig
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
	Count      int
	MaxWorkers int
}

type LogConfig struct {
	Level string
}

// ConnectionString generates the connection string for the PostgreSQL database
func (c *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s", c.User, c.Password, c.Host, c.Port, c.DBName, c.SSLMode)
}

// RabbitMQURL generates the connection string for RabbitMQ
func (c *RabbitMQConfig) RabbitMQURL() string {
	return fmt.Sprintf("amqp://%s:%s@%s:%d/",
		c.User, c.Password, c.Host, c.Port)
}

// Load returns the application configuration from environment variables
func Load() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	var config Config
	if err := unmarshalConfig(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}

func setDefaults() {
	// Server defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "release")

	// Database defaults
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.user", "postgres")
	viper.SetDefault("database.password", "postgres")
	viper.SetDefault("database.dbname", "image_optimizer")
	viper.SetDefault("database.sslmode", "disable")
	viper.SetDefault("database.max.connections", 10)
	viper.SetDefault("database.min.connections", 2)

	// MinIO defaults
	viper.SetDefault("minio.endpoint", "localhost:9000")
	viper.SetDefault("minio.access.key", "minioadmin")
	viper.SetDefault("minio.secret.key", "minioadmin")
	viper.SetDefault("minio.bucket", "images")
	viper.SetDefault("minio.ssl", false)
	viper.SetDefault("minio.location", "us-east-1")
	viper.SetDefault("minio.url.expiry", 24*time.Hour)

	// RabbitMQ defaults
	viper.SetDefault("rabbitmq.host", "rabbitmq")
	viper.SetDefault("rabbitmq.port", 5672)
	viper.SetDefault("rabbitmq.user", "guest")
	viper.SetDefault("rabbitmq.password", "guest")
	viper.SetDefault("rabbitmq.queue", "image_processing")
	viper.SetDefault("rabbitmq.exchange", "image_optimizer")
	viper.SetDefault("rabbitmq.routing.key", "image.resize")
	viper.SetDefault("rabbitmq.consumer.tag", "image_worker")

	// Worker defaults
	viper.SetDefault("worker.count", 4)
	viper.SetDefault("max.workers", 10)

	// Log defaults
	viper.SetDefault("log.level", "info")
}

func unmarshalConfig(config *Config) error {
	// Server config
	config.Server.Host = viper.GetString("server.host")
	config.Server.Port = viper.GetInt("server.port")
	config.Server.Mode = viper.GetString("server.mode")

	// Database config
	config.Database.Host = viper.GetString("database.host")
	config.Database.Port = viper.GetInt("database.port")
	config.Database.User = viper.GetString("database.user")
	config.Database.Password = viper.GetString("database.password")
	config.Database.DBName = viper.GetString("database.dbname")
	config.Database.SSLMode = viper.GetString("database.sslmode")
	config.Database.MaxConnections = viper.GetInt("database.max.connections")
	config.Database.MinConnections = viper.GetInt("database.min.connections")

	// MinIO config
	config.MinIO.Endpoint = viper.GetString("minio.endpoint")
	config.MinIO.AccessKey = viper.GetString("minio.access.key")
	config.MinIO.SecretKey = viper.GetString("minio.secret.key")
	config.MinIO.Bucket = viper.GetString("minio.bucket")
	config.MinIO.SSL = viper.GetBool("minio.ssl")
	config.MinIO.Location = viper.GetString("minio.location")
	config.MinIO.URLExpiry = viper.GetDuration("minio.url.expiry")

	// RabbitMQ config
	config.RabbitMQ.Host = viper.GetString("rabbitmq.host")
	config.RabbitMQ.Port = viper.GetInt("rabbitmq.port")
	config.RabbitMQ.User = viper.GetString("rabbitmq.user")
	config.RabbitMQ.Password = viper.GetString("rabbitmq.password")
	config.RabbitMQ.Queue = viper.GetString("rabbitmq.queue")
	config.RabbitMQ.Exchange = viper.GetString("rabbitmq.exchange")
	config.RabbitMQ.RoutingKey = viper.GetString("rabbitmq.routing.key")
	config.RabbitMQ.ConsumerTag = viper.GetString("rabbitmq.consumer.tag")

	// Worker config
	config.Worker.Count = viper.GetInt("worker.count")
	config.Worker.MaxWorkers = viper.GetInt("max.workers")

	// Log config
	config.Log.Level = viper.GetString("log.level")

	return nil
}
