package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	t.Run("load config with defaults", func(t *testing.T) {
		cfg, err := LoadConfig(".")
		require.NoError(t, err)

		// Check default values
		assert.Equal(t, 8080, cfg.Server.Port)
		assert.Equal(t, 10*time.Second, cfg.Server.ShutdownTimeout)
		assert.Equal(t, "postgres://user:password@localhost:5432/cron?sslmode=disable", cfg.Postgres.URL)
		assert.Equal(t, "amqp://guest:guest@localhost:5672/", cfg.RabbitMQ.URL)
		assert.Equal(t, "cron_jobs", cfg.RabbitMQ.QueueName)
		assert.Equal(t, 10, cfg.Scheduler.Concurrency)
		assert.Equal(t, 5*time.Second, cfg.Scheduler.WebhookTimeout)
		assert.Equal(t, 1*time.Hour, cfg.Scheduler.JobTTL)
		assert.Equal(t, 9090, cfg.Prometheus.Port)
	})

	t.Run("load config with environment variables", func(t *testing.T) {
		// Set environment variables
		os.Setenv("SERVER_PORT", "9000")
		os.Setenv("SERVER_SHUTDOWN_TIMEOUT", "15s")
		os.Setenv("POSTGRES_URL", "postgres://test:test@localhost:5432/testdb")
		os.Setenv("RABBITMQ_URL", "amqp://test:test@localhost:5672/")
		os.Setenv("RABBITMQ_QUEUE_NAME", "test_queue")
		os.Setenv("SCHEDULER_CONCURRENCY", "20")
		os.Setenv("SCHEDULER_WEBHOOK_TIMEOUT", "10s")
		os.Setenv("SCHEDULER_JOB_TTL", "2h")
		os.Setenv("PROMETHEUS_PORT", "9091")

		defer func() {
			// Clean up environment variables
			os.Unsetenv("SERVER_PORT")
			os.Unsetenv("SERVER_SHUTDOWN_TIMEOUT")
			os.Unsetenv("POSTGRES_URL")
			os.Unsetenv("RABBITMQ_URL")
			os.Unsetenv("RABBITMQ_QUEUE_NAME")
			os.Unsetenv("SCHEDULER_CONCURRENCY")
			os.Unsetenv("SCHEDULER_WEBHOOK_TIMEOUT")
			os.Unsetenv("SCHEDULER_JOB_TTL")
			os.Unsetenv("PROMETHEUS_PORT")
		}()

		cfg, err := LoadConfig(".")
		require.NoError(t, err)

		// Check environment variable values
		assert.Equal(t, 9000, cfg.Server.Port)
		assert.Equal(t, 15*time.Second, cfg.Server.ShutdownTimeout)
		assert.Equal(t, "postgres://test:test@localhost:5432/testdb", cfg.Postgres.URL)
		assert.Equal(t, "amqp://test:test@localhost:5672/", cfg.RabbitMQ.URL)
		assert.Equal(t, "test_queue", cfg.RabbitMQ.QueueName)
		assert.Equal(t, 20, cfg.Scheduler.Concurrency)
		assert.Equal(t, 10*time.Second, cfg.Scheduler.WebhookTimeout)
		assert.Equal(t, 2*time.Hour, cfg.Scheduler.JobTTL)
		assert.Equal(t, 9091, cfg.Prometheus.Port)
	})

	t.Run("load config with file", func(t *testing.T) {
		// Create a temporary config file
		configContent := `
server:
  port: 3000
  shutdown_timeout: "20s"
postgres:
  url: "postgres://file:file@localhost:5432/filedb"
rabbitmq:
  url: "amqp://file:file@localhost:5672/"
  queue_name: "file_queue"
scheduler:
  concurrency: 15
  webhook_timeout: "8s"
  job_ttl: "3h"
prometheus:
  port: 9092
`

		// Create temporary directory and file
		tempDir := t.TempDir()
		configFile := tempDir + "/config.yaml"
		err := os.WriteFile(configFile, []byte(configContent), 0644)
		require.NoError(t, err)

		cfg, err := LoadConfig(tempDir)
		require.NoError(t, err)

		// Check file values
		assert.Equal(t, 3000, cfg.Server.Port)
		assert.Equal(t, 20*time.Second, cfg.Server.ShutdownTimeout)
		assert.Equal(t, "postgres://file:file@localhost:5432/filedb", cfg.Postgres.URL)
		assert.Equal(t, "amqp://file:file@localhost:5672/", cfg.RabbitMQ.URL)
		assert.Equal(t, "file_queue", cfg.RabbitMQ.QueueName)
		assert.Equal(t, 15, cfg.Scheduler.Concurrency)
		assert.Equal(t, 8*time.Second, cfg.Scheduler.WebhookTimeout)
		assert.Equal(t, 3*time.Hour, cfg.Scheduler.JobTTL)
		assert.Equal(t, 9092, cfg.Prometheus.Port)
	})
}

func TestConfigStructs(t *testing.T) {
	t.Run("ServerConfig", func(t *testing.T) {
		cfg := ServerConfig{
			Port:            8080,
			ShutdownTimeout: 10 * time.Second,
		}

		assert.Equal(t, 8080, cfg.Port)
		assert.Equal(t, 10*time.Second, cfg.ShutdownTimeout)
	})

	t.Run("PostgresConfig", func(t *testing.T) {
		cfg := PostgresConfig{
			URL: "postgres://user:pass@localhost:5432/db",
		}

		assert.Equal(t, "postgres://user:pass@localhost:5432/db", cfg.URL)
	})

	t.Run("RabbitMQConfig", func(t *testing.T) {
		cfg := RabbitMQConfig{
			URL:       "amqp://user:pass@localhost:5672/",
			QueueName: "test_queue",
		}

		assert.Equal(t, "amqp://user:pass@localhost:5672/", cfg.URL)
		assert.Equal(t, "test_queue", cfg.QueueName)
	})

	t.Run("SchedulerConfig", func(t *testing.T) {
		cfg := SchedulerConfig{
			Concurrency:    5,
			WebhookTimeout: 30 * time.Second,
			JobTTL:         2 * time.Hour,
		}

		assert.Equal(t, 5, cfg.Concurrency)
		assert.Equal(t, 30*time.Second, cfg.WebhookTimeout)
		assert.Equal(t, 2*time.Hour, cfg.JobTTL)
	})

	t.Run("PrometheusConfig", func(t *testing.T) {
		cfg := PrometheusConfig{
			Port: 9090,
		}

		assert.Equal(t, 9090, cfg.Port)
	})

	t.Run("Config", func(t *testing.T) {
		cfg := Config{
			Server: ServerConfig{
				Port:            8080,
				ShutdownTimeout: 10 * time.Second,
			},
			Postgres: PostgresConfig{
				URL: "postgres://user:pass@localhost:5432/db",
			},
			RabbitMQ: RabbitMQConfig{
				URL:       "amqp://user:pass@localhost:5672/",
				QueueName: "test_queue",
			},
			Scheduler: SchedulerConfig{
				Concurrency:    5,
				WebhookTimeout: 30 * time.Second,
				JobTTL:         2 * time.Hour,
			},
			Prometheus: PrometheusConfig{
				Port: 9090,
			},
		}

		assert.Equal(t, 8080, cfg.Server.Port)
		assert.Equal(t, "postgres://user:pass@localhost:5432/db", cfg.Postgres.URL)
		assert.Equal(t, "amqp://user:pass@localhost:5672/", cfg.RabbitMQ.URL)
		assert.Equal(t, "test_queue", cfg.RabbitMQ.QueueName)
		assert.Equal(t, 5, cfg.Scheduler.Concurrency)
		assert.Equal(t, 30*time.Second, cfg.Scheduler.WebhookTimeout)
		assert.Equal(t, 2*time.Hour, cfg.Scheduler.JobTTL)
		assert.Equal(t, 9090, cfg.Prometheus.Port)
	})
}

func TestConfigValidation(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := Config{
			Server: ServerConfig{
				Port:            8080,
				ShutdownTimeout: 10 * time.Second,
			},
			Postgres: PostgresConfig{
				URL: "postgres://user:pass@localhost:5432/db",
			},
			RabbitMQ: RabbitMQConfig{
				URL:       "amqp://user:pass@localhost:5672/",
				QueueName: "test_queue",
			},
			Scheduler: SchedulerConfig{
				Concurrency:    5,
				WebhookTimeout: 30 * time.Second,
				JobTTL:         2 * time.Hour,
			},
			Prometheus: PrometheusConfig{
				Port: 9090,
			},
		}

		// Basic validation checks
		assert.Greater(t, cfg.Server.Port, 0)
		assert.Greater(t, cfg.Server.ShutdownTimeout, 0*time.Second)
		assert.NotEmpty(t, cfg.Postgres.URL)
		assert.NotEmpty(t, cfg.RabbitMQ.URL)
		assert.NotEmpty(t, cfg.RabbitMQ.QueueName)
		assert.Greater(t, cfg.Scheduler.Concurrency, 0)
		assert.Greater(t, cfg.Scheduler.WebhookTimeout, 0*time.Second)
		assert.Greater(t, cfg.Scheduler.JobTTL, 0*time.Second)
		assert.Greater(t, cfg.Prometheus.Port, 0)
	})

	t.Run("edge cases", func(t *testing.T) {
		cfg := Config{
			Server: ServerConfig{
				Port:            1,
				ShutdownTimeout: 1 * time.Nanosecond,
			},
			Postgres: PostgresConfig{
				URL: "postgres://",
			},
			RabbitMQ: RabbitMQConfig{
				URL:       "amqp://",
				QueueName: "",
			},
			Scheduler: SchedulerConfig{
				Concurrency:    1,
				WebhookTimeout: 1 * time.Nanosecond,
				JobTTL:         1 * time.Nanosecond,
			},
			Prometheus: PrometheusConfig{
				Port: 1,
			},
		}

		// Edge case values
		assert.Equal(t, 1, cfg.Server.Port)
		assert.Equal(t, 1*time.Nanosecond, cfg.Server.ShutdownTimeout)
		assert.Equal(t, "postgres://", cfg.Postgres.URL)
		assert.Equal(t, "amqp://", cfg.RabbitMQ.URL)
		assert.Equal(t, "", cfg.RabbitMQ.QueueName)
		assert.Equal(t, 1, cfg.Scheduler.Concurrency)
		assert.Equal(t, 1*time.Nanosecond, cfg.Scheduler.WebhookTimeout)
		assert.Equal(t, 1*time.Nanosecond, cfg.Scheduler.JobTTL)
		assert.Equal(t, 1, cfg.Prometheus.Port)
	})
}