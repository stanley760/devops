// Package database defines transaction semantics across database adapters.
//
// The business layer depends only on Transaction / ExecutorFromContext, unaware of MySQL DSN,
// connection pool, or GORM Transaction parameter details.
package database

import (
	"context"
	"database/sql"
	"errors"

	"gorm.io/gorm"
)

// Executor is a database executor that can start transactions.
// *gorm.DB naturally satisfies this interface; tests can also replace it with a custom fake.
type Executor interface {
	Transaction(fc func(tx *gorm.DB) error, opts ...*sql.TxOptions) error
}

// TxFunc is a transaction function. The context received by fn already carries the current transaction,
// which a Repository retrieves via ExecutorFromContext and reuses.
type TxFunc func(ctx context.Context) error

type txContextKey struct{}

// Transaction executes fn on the executor and injects the transaction into the context.
// Transaction boundaries are controlled by the service layer; Repositories no longer Begin/Commit/Rollback on their own.
func Transaction(ctx context.Context, executor Executor, fn TxFunc) error {
	if executor == nil {
		return errors.New("database: executor is nil")
	}
	if fn == nil {
		return errors.New("database: transaction function is nil")
	}

	return executor.Transaction(func(tx *gorm.DB) error {
		if tx == nil {
			return errors.New("database: transaction is nil")
		}
		return fn(context.WithValue(ctx, txContextKey{}, tx.WithContext(ctx)))
	})
}

// ExecutorFromContext retrieves the current transaction; returns false when absent.
func ExecutorFromContext(ctx context.Context) (*gorm.DB, bool) {
	tx, ok := ctx.Value(txContextKey{}).(*gorm.DB)
	return tx, ok && tx != nil
}
