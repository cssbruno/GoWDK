package sqlitetest

import (
	"context"
	"database/sql"
	"testing"
	"testing/fstest"

	gowdkdb "github.com/cssbruno/gowdk/addons/db"
	_ "modernc.org/sqlite"
)

func TestDBAddonHelpersWithSQLiteDriver(t *testing.T) {
	ctx := context.Background()
	database, err := gowdkdb.OpenWithOptions("sqlite", "file:gowdk-db-addon-test?mode=memory&cache=shared", gowdkdb.Options{
		MaxOpenConns: 1,
	})
	if err != nil {
		t.Fatalf("OpenWithOptions: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if readiness := gowdkdb.CheckReadiness(ctx, database); !readiness.Ready {
		t.Fatalf("expected database to be ready: %#v", readiness)
	}

	migrations := fstest.MapFS{
		"migrations/001_schema.sql": {Data: []byte(`CREATE TABLE widgets (id INTEGER PRIMARY KEY, name TEXT NOT NULL);`)},
		"migrations/002_seed.sql":   {Data: []byte(`INSERT INTO widgets (name) VALUES ('first');`)},
	}
	result, err := gowdkdb.ApplyMigrations(ctx, database, migrations, gowdkdb.MigrationOptions{Dir: "migrations"})
	if err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}
	if len(result.Applied) != 2 || len(result.Skipped) != 0 {
		t.Fatalf("unexpected migration result: %#v", result)
	}

	second, err := gowdkdb.ApplyMigrations(ctx, database, migrations, gowdkdb.MigrationOptions{Dir: "migrations"})
	if err != nil {
		t.Fatalf("second ApplyMigrations: %v", err)
	}
	if len(second.Applied) != 0 || len(second.Skipped) != 2 {
		t.Fatalf("expected second migration run to skip both files, got %#v", second)
	}

	if err := gowdkdb.WithTx(ctx, database, nil, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO widgets (name) VALUES ('committed')`)
		return err
	}); err != nil {
		t.Fatalf("WithTx commit: %v", err)
	}

	err = gowdkdb.WithTx(ctx, database, nil, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO widgets (name) VALUES ('rolled-back')`)
		if err != nil {
			return err
		}
		return context.Canceled
	})
	if err == nil {
		t.Fatal("expected rollback error")
	}

	var count int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM widgets`).Scan(&count); err != nil {
		t.Fatalf("count widgets: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected only seeded and committed rows, got %d", count)
	}
}
