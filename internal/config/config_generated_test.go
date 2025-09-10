package config

import "testing"

func TestZeroValues(t *testing.T) {
    _ = Config{}
    _ = PostgresConfig{}
    _ = PrometheusConfig{}
    _ = RabbitMQConfig{}
    _ = SchedulerConfig{}
    _ = ServerConfig{}
}
