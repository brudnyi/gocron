package worker

import (
	"context"
	"time"
)

// ManagerInterface defines the interface for a worker manager.
// It allows for mocking in tests.
type ManagerInterface interface {
	Publish(ctx context.Context, jobID int64, delay time.Duration) error
	Start(ctx context.Context)
	Stop()
}
