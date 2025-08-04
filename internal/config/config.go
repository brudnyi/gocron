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
	Port int `mapstructure:"port"`
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
	Concurrency           int           `mapstructure:"concurrency"`
	WebhookTimeoutSeconds time.Duration `mapstructure:"webhook_timeout_seconds"`
	JobTTLSeconds         time.Duration `mapstructure:"job_ttl_seconds"`
}

// PrometheusConfig holds prometheus specific configuration.
type PrometheusConfig struct {
	Port int `mapstructure:"port"`
}

// LoadConfig reads configuration from file or environment variables.
func LoadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	err = viper.ReadInConfig()
	if err != nil {
		return
	}

	err = viper.Unmarshal(&config)

	// Set duration fields manually
	config.Scheduler.WebhookTimeoutSeconds = config.Scheduler.WebhookTimeoutSeconds * time.Second
	config.Scheduler.JobTTLSeconds = config.Scheduler.JobTTLSeconds * time.Second
	
	return
}
