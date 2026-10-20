package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"gorm.io/gorm"
)

type fakeExecutor struct {
	tx *gorm.DB
}

func (e fakeExecutor) Transaction(fn func(tx *gorm.DB) error, _ ...*sql.TxOptions) error {
	return fn(e.tx)
}

func TestTransactionInjectsExecutor(t *testing.T) {
	ctx := context.Background()
	tx := &gorm.DB{
		Config:    &gorm.Config{},
		Statement: &gorm.Statement{Context: ctx},
	}

	err := Transaction(ctx, fakeExecutor{tx: tx}, func(txCtx context.Context) error {
		got, ok := ExecutorFromContext(txCtx)
		if !ok || got == nil {
			t.Fatal("transaction was not injected into context")
		}
		if got.Statement.Context != ctx {
			t.Fatal("transaction executor did not inherit the original context")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("transaction execution failed: %v", err)
	}
}

func TestTransactionNilArguments(t *testing.T) {
	if err := Transaction(context.Background(), nil, func(context.Context) error { return nil }); err == nil {
		t.Fatal("should return an error when executor is nil")
	}
	if err := Transaction(context.Background(), fakeExecutor{}, nil); err == nil {
		t.Fatal("should return an error when the transaction function is nil")
	}
}

func TestTransactionPropagatesError(t *testing.T) {
	want := errors.New("rollback")
	tx := &gorm.DB{Config: &gorm.Config{}, Statement: &gorm.Statement{Context: context.Background()}}
	err := Transaction(context.Background(), fakeExecutor{tx: tx}, func(context.Context) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}
