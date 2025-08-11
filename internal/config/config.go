package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application.
type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Postgres    PostgresConfig    `mapstructure:"postgres"`
	RabbitMQ    RabbitMQConfig    `mapstructure:"rabbitmq"`
	Scheduler   SchedulerConfig   `mapstructure:"scheduler"`
	Prometheus  PrometheusConfig  `mapstructure:"prometheus"`
}

// ServerConfig holds server specific configuration.
type ServerConfig struct {
	Port            int           `mapstructure:"port"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// PostgresConfig holds postgres specific configuration.
type PostgresConfig struct {
	URL string `mapstructure:"url"`
}

// RabbitMQConfig holds rabbitmq specific configuration.
type RabbitMQConfig struct {
	URL       string `mapstructure:"url"`
	QueueName string `mapstructure:"queue_name"`
}

// SchedulerConfig holds scheduler specific configuration.
type SchedulerConfig struct {
	Concurrency    int           `mapstructure:"concurrency"`
	WebhookTimeout time.Duration `mapstructure:"webhook_timeout"`
	JobTTL         time.Duration `mapstructure:"job_ttl"`
}

// PrometheusConfig holds prometheus specific configuration.
type PrometheusConfig struct {
	Port int `mapstructure:"port"`
}

// LoadConfig reads configuration from file or environment variables.
func LoadConfig(path string) (config Config, err error) {
	// Set default values
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.shutdown_timeout", "10s")
	viper.SetDefault("postgres.url", "postgres://user:password@localhost:5432/cron?sslmode=disable")
	viper.SetDefault("rabbitmq.url", "amqp://guest:guest@localhost:5672/")
	viper.SetDefault("rabbitmq.queue_name", "cron_jobs")
	viper.SetDefault("scheduler.concurrency", 10)
	viper.SetDefault("scheduler.webhook_timeout", "5s")
	viper.SetDefault("scheduler.job_ttl", "1h")
	viper.SetDefault("prometheus.port", 9090)

	viper.AddConfigPath(path)
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	err = viper.ReadInConfig()
	if err != nil {
		// If the config file is not found, we can continue as we have defaults and env vars
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return
		}
	}

	err = viper.Unmarshal(&config)
	return
}
