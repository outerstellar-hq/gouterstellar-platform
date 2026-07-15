package service

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// TransactionRunner abstracts the ability to run a function inside a database
// transaction. *persistence.TransactionManager satisfies this interface; tests
// can supply a fake runner (e.g. FakeTxRunner) to exercise service logic without
// a live database connection.
type TransactionRunner interface {
	InTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// FakeTxRunner is a test double for TransactionRunner. It invokes fn with the
// supplied Tx (which may be nil when the test does not assert on tx usage) and
// returns whatever fn returns. This lets unit tests exercise transactional
// service methods without a database connection.
type FakeTxRunner struct {
	Tx pgx.Tx
}

func (f *FakeTxRunner) InTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return fn(f.Tx)
}
