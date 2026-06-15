package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// WithTx runs fn inside a database/sql transaction. The transaction commits
// when fn returns nil and rolls back when fn returns an error or panics.
func WithTx(ctx context.Context, database *sql.DB, options *sql.TxOptions, fn func(context.Context, *sql.Tx) error) (err error) {
	if database == nil {
		return fmt.Errorf("gowdk db: database is required")
	}
	if fn == nil {
		return fmt.Errorf("gowdk db: transaction function is required")
	}
	tx, err := database.BeginTx(ctx, options)
	if err != nil {
		return fmt.Errorf("gowdk db: begin transaction: %w", err)
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = tx.Rollback()
			panic(recovered)
		}
	}()
	if err := fn(ctx, tx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			return errors.Join(err, fmt.Errorf("gowdk db: rollback transaction: %w", rollbackErr))
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("gowdk db: commit transaction: %w", err)
	}
	return nil
}
