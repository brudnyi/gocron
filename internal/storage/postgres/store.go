package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier defines the interface for database query operations.
type Querier interface {
	CreateJob(ctx context.Context, arg CreateJobParams) (Job, error)
	CreateJobLog(ctx context.Context, arg CreateJobLogParams) (JobLog, error)
	DeleteJob(ctx context.Context, id int64) error
	GetActiveJobs(ctx context.Context) ([]Job, error)
	GetJob(ctx context.Context, id int64) (Job, error)
	GetJobByCustomID(ctx context.Context, customID pgtype.Text) (Job, error)
	GetJobLogs(ctx context.Context, arg GetJobLogsParams) ([]JobLog, error)
	ProcessJob(ctx context.Context, id int64) (Job, error)
	UpdateJobAfterExecution(ctx context.Context, arg UpdateJobAfterExecutionParams) (Job, error)
	UpdateJobStatus(ctx context.Context, arg UpdateJobStatusParams) (Job, error)
}

// Store provides all functions to execute db queries and transactions
type Store struct {
	*Queries
	pool *pgxpool.Pool
}

// NewStore creates a new Store
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{
		pool:    pool,
		Queries: New(pool),
	}
}

// ExecTx executes a function within a database transaction
func (s *Store) ExecTx(ctx context.Context, fn func(*Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) // Rollback is a no-op if the transaction is already committed

	q := New(tx)
	err = fn(q)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}
