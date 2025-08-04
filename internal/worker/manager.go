package worker

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"gitlab.uis.dev/service/gocron/internal/config"
)

// JobFunc is a function that processes a job.
type JobFunc func(ctx context.Context, jobID int64) error

// Manager handles the RabbitMQ connection and worker pool.
type Manager struct {
	log         *slog.Logger
	cfg         config.RabbitMQConfig
	conn        *amqp.Connection
	channel     *amqp.Channel
	jobQueue    <-chan amqp.Delivery
	jobFunc     JobFunc
	wg          sync.WaitGroup
	concurrency int
}

// NewManager creates a new worker manager.
func NewManager(log *slog.Logger, cfg config.RabbitMQConfig, schedulerCfg config.SchedulerConfig, jobFunc JobFunc) (*Manager, error) {
	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open a channel: %w", err)
	}

	// Declare a queue for delayed messages
	_, err = ch.QueueDeclare(
		cfg.QueueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-dead-letter-exchange":    "",
			"x-dead-letter-routing-key": cfg.QueueName,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare a queue: %w", err)
	}

	msgs, err := ch.Consume(
		cfg.QueueName,
		"",    // consumer
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register a consumer: %w", err)
	}

	return &Manager{
		log:         log,
		cfg:         cfg,
		conn:        conn,
		channel:     ch,
		jobQueue:    msgs,
		jobFunc:     jobFunc,
		concurrency: schedulerCfg.Concurrency,
	}, nil
}

// Start starts the worker pool.
func (m *Manager) Start(ctx context.Context) {
	for i := 0; i < m.concurrency; i++ {
		m.wg.Add(1)
		go func(workerID int) {
			defer m.wg.Done()
			m.log.Info("worker started", "id", workerID)
			for {
				select {
				case <-ctx.Done():
					m.log.Info("worker stopping", "id", workerID)
					return
				case msg, ok := <-m.jobQueue:
					if !ok {
						m.log.Info("job queue channel closed, worker stopping", "id", workerID)
						return
					}
					jobID, err := strconv.ParseInt(string(msg.Body), 10, 64)
					if err != nil {
						m.log.Error("failed to parse job ID", "error", err, "body", string(msg.Body))
						msg.Nack(false, false) // Nack and drop
						continue
					}

					m.log.Info("received job", "id", jobID)
					if err := m.jobFunc(ctx, jobID); err != nil {
						m.log.Error("failed to process job", "error", err, "job_id", jobID)
						// Depending on the error, we might want to requeue
						msg.Nack(false, false) // Nack and drop for now
					} else {
						msg.Ack(false)
					}
				}
			}
		}(i)
	}
}

// Stop gracefully stops the worker manager.
func (m *Manager) Stop() {
	m.log.Info("stopping worker manager")
	if m.channel != nil {
		m.channel.Close()
	}
	if m.conn != nil {
		m.conn.Close()
	}
	m.wg.Wait()
	m.log.Info("worker manager stopped")
}

// Publish sends a job to the queue with a delay.
func (m *Manager) Publish(ctx context.Context, jobID int64, delay time.Duration) error {
	// RabbitMQ's delayed messaging is often implemented using a dead-letter exchange
	// and a per-message TTL. We'll create a temporary queue for the delay.
	delayQueueName := fmt.Sprintf("%s_delay_%d", m.cfg.QueueName, delay.Milliseconds())

	_, err := m.channel.QueueDeclare(
		delayQueueName,
		true,  // durable
		true,  // auto-delete
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-dead-letter-exchange":    "",
			"x-dead-letter-routing-key": m.cfg.QueueName,
			"x-message-ttl":             delay.Milliseconds(),
			"x-expires":                 delay.Milliseconds() + 10000, // queue expires after delay + 10s
		},
	)
	if err != nil {
		return fmt.Errorf("failed to declare delay queue: %w", err)
	}

	body := []byte(strconv.FormatInt(jobID, 10))

	return m.channel.PublishWithContext(
		ctx,
		"",             // exchange
		delayQueueName, // routing key (the temporary queue)
		false,          // mandatory
		false,          // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        body,
		},
	)
}
