package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Storer defines the interface for database operations, including transactions.
type Storer interface {
	Querier
	ExecTx(ctx context.Context, fn func(*Queries) error) error
}

// Store provides all functions to execute db queries and transactions
type Store struct {
	*Queries
	pool *pgxpool.Pool
}

// NewStore creates a new Store
func NewStore(pool *pgxpool.Pool) Storer {
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
