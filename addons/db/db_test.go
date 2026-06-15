package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/cssbruno/gowdk"
)

func TestAddonEnablesDBFeature(t *testing.T) {
	addon := Addon()
	if addon.Name() != "db" {
		t.Fatalf("addon.Name() = %q, want db", addon.Name())
	}
	config := gowdk.Config{Addons: []gowdk.Addon{addon}}
	if !config.HasFeature(gowdk.FeatureDB) {
		t.Fatal("expected db feature to be enabled")
	}
}

func TestOpenRequiresDriver(t *testing.T) {
	if _, err := Open("", "some-dsn"); err == nil {
		t.Fatal("expected an error for an empty driver name")
	}
}

func TestOpenRequiresDSN(t *testing.T) {
	if _, err := Open("postgres", "   "); err == nil {
		t.Fatal("expected an error for an empty DSN")
	}
}

func TestOpenUnknownDriver(t *testing.T) {
	// No driver is registered in this test binary, so sql.Open reports an
	// unknown driver. We assert the helper surfaces that as a wrapped error
	// rather than panicking or returning a usable handle.
	_, err := Open("definitely-not-registered", "dsn")
	if err == nil {
		t.Fatal("expected an error for an unregistered driver")
	}
	if !strings.Contains(err.Error(), "gowdk db:") {
		t.Fatalf("error %q is not wrapped by the helper", err.Error())
	}
}

func TestPingAndReadiness(t *testing.T) {
	healthy := newFakeDB(t, &fakeDBState{})
	if err := Ping(context.Background(), healthy); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if readiness := CheckReadiness(context.Background(), healthy); !readiness.Ready || readiness.Error != "" || readiness.Duration < 0 {
		t.Fatalf("unexpected healthy readiness: %#v", readiness)
	}

	failing := newFakeDB(t, &fakeDBState{pingErr: errors.New("offline")})
	if err := Ping(context.Background(), failing); err == nil || !strings.Contains(err.Error(), "offline") {
		t.Fatalf("expected ping failure, got %v", err)
	}
	if readiness := CheckReadiness(context.Background(), failing); readiness.Ready || !strings.Contains(readiness.Error, "offline") {
		t.Fatalf("unexpected failing readiness: %#v", readiness)
	}
}

func TestWithTxCommits(t *testing.T) {
	state := &fakeDBState{}
	database := newFakeDB(t, state)

	if err := WithTx(context.Background(), database, nil, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO widgets VALUES (?)", "one")
		return err
	}); err != nil {
		t.Fatalf("WithTx: %v", err)
	}
	if state.commits != 1 || state.rollbacks != 0 {
		t.Fatalf("expected one commit and no rollback, got commits=%d rollbacks=%d", state.commits, state.rollbacks)
	}
}

func TestWithTxRollsBackOnError(t *testing.T) {
	state := &fakeDBState{}
	database := newFakeDB(t, state)
	expected := errors.New("handler failed")

	err := WithTx(context.Background(), database, nil, func(context.Context, *sql.Tx) error {
		return expected
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected handler error, got %v", err)
	}
	if state.commits != 0 || state.rollbacks != 1 {
		t.Fatalf("expected one rollback and no commit, got commits=%d rollbacks=%d", state.commits, state.rollbacks)
	}
}

func TestWithTxHonorsCanceledContext(t *testing.T) {
	state := &fakeDBState{}
	database := newFakeDB(t, state)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	err := WithTx(ctx, database, nil, func(context.Context, *sql.Tx) error {
		called = true
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if called {
		t.Fatal("transaction function should not run when BeginTx fails")
	}
	if state.commits != 0 || state.rollbacks != 0 {
		t.Fatalf("did not expect commit or rollback, got commits=%d rollbacks=%d", state.commits, state.rollbacks)
	}
}

func TestWithTxRejectsMissingInputs(t *testing.T) {
	if err := WithTx(context.Background(), nil, nil, func(context.Context, *sql.Tx) error { return nil }); err == nil {
		t.Fatal("expected nil database error")
	}
	database := newFakeDB(t, &fakeDBState{})
	if err := WithTx(context.Background(), database, nil, nil); err == nil {
		t.Fatal("expected nil transaction function error")
	}
}

func TestApplyMigrationsAppliesAndSkipsInOrder(t *testing.T) {
	state := &fakeDBState{migrations: map[string]string{}}
	database := newFakeDB(t, state)
	source := fstest.MapFS{
		"migrations/002_seed.sql":   {Data: []byte("INSERT INTO widgets VALUES ('seed');")},
		"migrations/001_schema.sql": {Data: []byte("CREATE TABLE widgets (name TEXT);")},
		"migrations/notes.txt":      {Data: []byte("ignore me")},
	}

	result, err := ApplyMigrations(context.Background(), database, source, MigrationOptions{
		Dir: "migrations",
		Now: func() time.Time { return time.Unix(1_700_000_000, 0) },
	})
	if err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}
	if got, want := migrationNames(result.Applied), []string{"migrations/001_schema.sql", "migrations/002_seed.sql"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("applied migrations = %#v, want %#v", got, want)
	}
	if len(result.Skipped) != 0 {
		t.Fatalf("did not expect skipped migrations, got %#v", result.Skipped)
	}
	if state.commits != 1 || state.rollbacks != 0 {
		t.Fatalf("expected commit, got commits=%d rollbacks=%d", state.commits, state.rollbacks)
	}

	second, err := ApplyMigrations(context.Background(), database, source, MigrationOptions{Dir: "migrations"})
	if err != nil {
		t.Fatalf("second ApplyMigrations: %v", err)
	}
	if len(second.Applied) != 0 || len(second.Skipped) != 2 {
		t.Fatalf("expected two skipped migrations on second run, got applied=%#v skipped=%#v", second.Applied, second.Skipped)
	}
}

func TestApplyMigrationsDetectsChecksumDrift(t *testing.T) {
	state := &fakeDBState{migrations: map[string]string{}}
	database := newFakeDB(t, state)
	first := fstest.MapFS{"001.sql": {Data: []byte("CREATE TABLE widgets (name TEXT);")}}
	if _, err := ApplyMigrations(context.Background(), database, first, MigrationOptions{}); err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}

	second := fstest.MapFS{"001.sql": {Data: []byte("CREATE TABLE widgets (name TEXT, id INTEGER);")}}
	_, err := ApplyMigrations(context.Background(), database, second, MigrationOptions{})
	if err == nil || !strings.Contains(err.Error(), "001.sql") || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch with file name, got %v", err)
	}
}

func TestApplyMigrationsRejectsUnsafeTableName(t *testing.T) {
	database := newFakeDB(t, &fakeDBState{})
	_, err := ApplyMigrations(context.Background(), database, fstest.MapFS{}, MigrationOptions{Table: "schema;drop"})
	if err == nil || !strings.Contains(err.Error(), "schema;drop") {
		t.Fatalf("expected unsafe table name error, got %v", err)
	}
}

func migrationNames(records []MigrationRecord) []string {
	names := make([]string, 0, len(records))
	for _, record := range records {
		names = append(names, record.Name)
	}
	return names
}

var fakeDriverCounter atomic.Int64

type fakeDBState struct {
	mu         sync.Mutex
	pingErr    error
	execErr    error
	commits    int
	rollbacks  int
	executed   []string
	migrations map[string]string
}

func newFakeDB(t *testing.T, state *fakeDBState) *sql.DB {
	t.Helper()
	if state.migrations == nil {
		state.migrations = map[string]string{}
	}
	name := fmt.Sprintf("gowdk_db_fake_%d", fakeDriverCounter.Add(1))
	sql.Register(name, fakeDriver{state: state})
	database, err := sql.Open(name, "test")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

type fakeDriver struct {
	state *fakeDBState
}

func (drv fakeDriver) Open(string) (driver.Conn, error) {
	return &fakeConn{state: drv.state}, nil
}

type fakeConn struct {
	state *fakeDBState
}

func (conn *fakeConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not implemented")
}

func (conn *fakeConn) Close() error {
	return nil
}

func (conn *fakeConn) Begin() (driver.Tx, error) {
	return &fakeTx{state: conn.state}, nil
}

func (conn *fakeConn) BeginTx(ctx context.Context, _ driver.TxOptions) (driver.Tx, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &fakeTx{state: conn.state}, nil
}

func (conn *fakeConn) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return conn.state.pingErr
}

func (conn *fakeConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	conn.state.mu.Lock()
	defer conn.state.mu.Unlock()
	if conn.state.execErr != nil {
		return nil, conn.state.execErr
	}
	conn.state.executed = append(conn.state.executed, strings.TrimSpace(query))
	if strings.Contains(query, "checksum") && strings.HasPrefix(strings.ToUpper(strings.TrimSpace(query)), "INSERT INTO") && len(args) >= 2 {
		name, _ := args[0].Value.(string)
		checksum, _ := args[1].Value.(string)
		conn.state.migrations[name] = checksum
	}
	return fakeResult(1), nil
}

func (conn *fakeConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	conn.state.mu.Lock()
	defer conn.state.mu.Unlock()
	if strings.Contains(query, "checksum") && len(args) >= 1 {
		name, _ := args[0].Value.(string)
		if checksum, ok := conn.state.migrations[name]; ok {
			return &fakeRows{values: []driver.Value{checksum}}, nil
		}
	}
	return &fakeRows{}, nil
}

type fakeTx struct {
	state *fakeDBState
}

func (tx *fakeTx) Commit() error {
	tx.state.mu.Lock()
	defer tx.state.mu.Unlock()
	tx.state.commits++
	return nil
}

func (tx *fakeTx) Rollback() error {
	tx.state.mu.Lock()
	defer tx.state.mu.Unlock()
	tx.state.rollbacks++
	return nil
}

type fakeResult int64

func (result fakeResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (result fakeResult) RowsAffected() (int64, error) {
	return int64(result), nil
}

type fakeRows struct {
	values []driver.Value
	sent   bool
}

func (rows *fakeRows) Columns() []string {
	return []string{"checksum"}
}

func (rows *fakeRows) Close() error {
	return nil
}

func (rows *fakeRows) Next(dest []driver.Value) error {
	if rows.sent || len(rows.values) == 0 {
		return io.EOF
	}
	rows.sent = true
	copy(dest, rows.values)
	return nil
}
