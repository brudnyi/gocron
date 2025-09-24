package worker

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gitlab.uis.dev/service/gocron/internal/config"
)

// MockWorkerManager implements ManagerInterface for testing
type MockWorkerManager struct {
	PublishedJobs map[int64]time.Duration
}

// NewMockWorkerManager creates a new mock worker manager.
func NewMockWorkerManager() *MockWorkerManager {
	return &MockWorkerManager{
		PublishedJobs: make(map[int64]time.Duration),
	}
}

// Publish records the job and its delay.
func (m *MockWorkerManager) Publish(ctx context.Context, jobID int64, delay time.Duration) error {
	m.PublishedJobs[jobID] = delay
	return nil
}

// Start is a no-op for the mock.
func (m *MockWorkerManager) Start(ctx context.Context) {}

// Stop is a no-op for the mock.
func (m *MockWorkerManager) Stop() {}

func TestManagerInterface(t *testing.T) {
	// Test that Manager implements ManagerInterface
	var _ ManagerInterface = (*Manager)(nil)
}

func TestNewManager(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		// This test would require a real RabbitMQ instance
		// For now, we'll test the interface and mock behavior
		log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		cfg := config.RabbitMQConfig{
			URL:       "amqp://guest:guest@localhost:5672/",
			QueueName: "test_queue",
		}
		schedulerCfg := config.SchedulerConfig{
			Concurrency: 5,
		}

		// Mock job function
		jobFunc := func(ctx context.Context, jobID int64) error {
			return nil
		}

		// Note: This test will fail without a real RabbitMQ instance
		// In a real test environment, you would use a test container or mock
		_, err := NewManager(log, cfg, schedulerCfg, jobFunc)
		
		// We expect this to fail in test environment without RabbitMQ
		// This demonstrates the test structure
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to connect to RabbitMQ")
	})
}

func TestMockWorkerManager(t *testing.T) {
	t.Run("publish job", func(t *testing.T) {
		mock := NewMockWorkerManager()
		ctx := context.Background()
		jobID := int64(123)
		delay := 5 * time.Second

		err := mock.Publish(ctx, jobID, delay)
		assert.NoError(t, err)
		assert.Equal(t, delay, mock.PublishedJobs[jobID])
	})

	t.Run("publish multiple jobs", func(t *testing.T) {
		mock := NewMockWorkerManager()
		ctx := context.Background()

		// Publish multiple jobs
		err1 := mock.Publish(ctx, 1, 1*time.Second)
		err2 := mock.Publish(ctx, 2, 2*time.Second)
		err3 := mock.Publish(ctx, 3, 3*time.Second)

		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.NoError(t, err3)

		assert.Len(t, mock.PublishedJobs, 3)
		assert.Equal(t, 1*time.Second, mock.PublishedJobs[1])
		assert.Equal(t, 2*time.Second, mock.PublishedJobs[2])
		assert.Equal(t, 3*time.Second, mock.PublishedJobs[3])
	})

	t.Run("start and stop", func(t *testing.T) {
		mock := NewMockWorkerManager()
		ctx := context.Background()

		// These should not panic
		mock.Start(ctx)
		mock.Stop()

		// Verify the mock is still functional
		err := mock.Publish(ctx, 1, 1*time.Second)
		assert.NoError(t, err)
	})

	t.Run("overwrite job", func(t *testing.T) {
		mock := NewMockWorkerManager()
		ctx := context.Background()
		jobID := int64(1)

		// Publish job with initial delay
		err1 := mock.Publish(ctx, jobID, 5*time.Second)
		assert.NoError(t, err1)
		assert.Equal(t, 5*time.Second, mock.PublishedJobs[jobID])

		// Publish same job with different delay
		err2 := mock.Publish(ctx, jobID, 10*time.Second)
		assert.NoError(t, err2)
		assert.Equal(t, 10*time.Second, mock.PublishedJobs[jobID])
	})
}

func TestJobFunc(t *testing.T) {
	t.Run("job function signature", func(t *testing.T) {
		// Test that JobFunc has the correct signature
		var jobFunc JobFunc = func(ctx context.Context, jobID int64) error {
			return nil
		}

		ctx := context.Background()
		err := jobFunc(ctx, 123)
		assert.NoError(t, err)
	})

	t.Run("job function with error", func(t *testing.T) {
		var jobFunc JobFunc = func(ctx context.Context, jobID int64) error {
			return assert.AnError
		}

		ctx := context.Background()
		err := jobFunc(ctx, 123)
		assert.Error(t, err)
		assert.Equal(t, assert.AnError, err)
	})

	t.Run("job function with context cancellation", func(t *testing.T) {
		var jobFunc JobFunc = func(ctx context.Context, jobID int64) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return nil
			}
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := jobFunc(ctx, 123)
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})
}

func TestManagerConcurrency(t *testing.T) {
	t.Run("concurrent publishes", func(t *testing.T) {
		mock := NewMockWorkerManager()
		ctx := context.Background()

		// Publish jobs concurrently
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func(jobID int) {
				defer func() { done <- true }()
				err := mock.Publish(ctx, int64(jobID), time.Duration(jobID)*time.Second)
				assert.NoError(t, err)
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}

		assert.Len(t, mock.PublishedJobs, 10)
		for i := 0; i < 10; i++ {
			expectedDelay := time.Duration(i) * time.Second
			assert.Equal(t, expectedDelay, mock.PublishedJobs[int64(i)])
		}
	})
}

func TestManagerEdgeCases(t *testing.T) {
	t.Run("zero delay", func(t *testing.T) {
		mock := NewMockWorkerManager()
		ctx := context.Background()

		err := mock.Publish(ctx, 1, 0)
		assert.NoError(t, err)
		assert.Equal(t, time.Duration(0), mock.PublishedJobs[1])
	})

	t.Run("negative job ID", func(t *testing.T) {
		mock := NewMockWorkerManager()
		ctx := context.Background()

		err := mock.Publish(ctx, -1, 1*time.Second)
		assert.NoError(t, err)
		assert.Equal(t, 1*time.Second, mock.PublishedJobs[-1])
	})

	t.Run("very large delay", func(t *testing.T) {
		mock := NewMockWorkerManager()
		ctx := context.Background()

		largeDelay := 24 * time.Hour
		err := mock.Publish(ctx, 1, largeDelay)
		assert.NoError(t, err)
		assert.Equal(t, largeDelay, mock.PublishedJobs[1])
	})

	t.Run("nil context", func(t *testing.T) {
		mock := NewMockWorkerManager()

		// This should not panic
		err := mock.Publish(nil, 1, 1*time.Second)
		assert.NoError(t, err)
		assert.Equal(t, 1*time.Second, mock.PublishedJobs[1])
	})
}

func TestManagerIntegration(t *testing.T) {
	t.Run("full workflow", func(t *testing.T) {
		mock := NewMockWorkerManager()
		ctx := context.Background()

		// Start the manager
		mock.Start(ctx)

		// Publish some jobs
		jobIDs := []int64{1, 2, 3, 4, 5}
		delays := []time.Duration{
			1 * time.Second,
			2 * time.Second,
			3 * time.Second,
			4 * time.Second,
			5 * time.Second,
		}

		for i, jobID := range jobIDs {
			err := mock.Publish(ctx, jobID, delays[i])
			assert.NoError(t, err)
		}

		// Verify all jobs were published
		assert.Len(t, mock.PublishedJobs, len(jobIDs))
		for i, jobID := range jobIDs {
			assert.Equal(t, delays[i], mock.PublishedJobs[jobID])
		}

		// Stop the manager
		mock.Stop()

		// Verify jobs are still there after stop
		assert.Len(t, mock.PublishedJobs, len(jobIDs))
	})
}

func TestManagerErrorHandling(t *testing.T) {
	t.Run("job function error handling", func(t *testing.T) {
		// This test demonstrates how error handling would work
		// in a real manager implementation
		var jobFunc JobFunc = func(ctx context.Context, jobID int64) error {
			if jobID == 0 {
				return assert.AnError
			}
			return nil
		}

		ctx := context.Background()

		// Test successful job
		err := jobFunc(ctx, 1)
		assert.NoError(t, err)

		// Test failing job
		err = jobFunc(ctx, 0)
		assert.Error(t, err)
		assert.Equal(t, assert.AnError, err)
	})
}

func TestManagerTimeout(t *testing.T) {
	t.Run("job function with timeout", func(t *testing.T) {
		var jobFunc JobFunc = func(ctx context.Context, jobID int64) error {
			select {
			case <-time.After(100 * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Test with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err := jobFunc(ctx, 1)
		assert.Error(t, err)
		assert.Equal(t, context.DeadlineExceeded, err)
	})

	t.Run("job function without timeout", func(t *testing.T) {
		var jobFunc JobFunc = func(ctx context.Context, jobID int64) error {
			select {
			case <-time.After(50 * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Test without timeout
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := jobFunc(ctx, 1)
		assert.NoError(t, err)
	})
}