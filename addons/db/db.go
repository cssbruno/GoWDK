// Package db is the GOWDK database plumbing addon. It enables the db feature and
// ships a thin, driver-agnostic helper for opening a *sql.DB on top of the
// standard library's database/sql. It is designed to pair with sqlc: you own
// the schema, the queries, the generated models, and all domain logic. GOWDK
// owns none of that and adds no driver, ORM, or query opinions.
//
// The helper imports no SQL driver. Your application registers a driver with a
// blank import (for example _ "github.com/jackc/pgx/v5/stdlib") and passes its
// name to Open, so GOWDK carries no database dependency of its own.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/cssbruno/gowdk"
)

// ImportPath is the canonical Go import path for the db addon.
const ImportPath = "github.com/cssbruno/gowdk/addons/db"

// Addon enables the database plumbing feature.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("db", gowdk.FeatureDB)
}

// Options tunes the connection pool. The zero value applies sensible defaults.
type Options struct {
	// MaxOpenConns caps total open connections. Zero means unlimited.
	MaxOpenConns int
	// MaxIdleConns caps idle connections. Zero uses the database/sql default.
	MaxIdleConns int
	// ConnMaxLifetime caps connection reuse time. Zero means unlimited.
	ConnMaxLifetime time.Duration
	// PingTimeout bounds the initial connectivity check. Defaults to 5s.
	PingTimeout time.Duration
}

// DefaultPingTimeout bounds the connectivity check performed by Open.
const DefaultPingTimeout = 5 * time.Second

// Open opens a *sql.DB for the given registered driver and DSN, applies pool
// defaults, and verifies the connection with a bounded ping. The driver must be
// registered by the caller (typically via a blank import) before Open is
// called.
func Open(driver, dsn string) (*sql.DB, error) {
	return OpenWithOptions(driver, dsn, Options{})
}

// OpenWithOptions is Open with explicit pool configuration.
func OpenWithOptions(driver, dsn string, options Options) (*sql.DB, error) {
	if strings.TrimSpace(driver) == "" {
		return nil, fmt.Errorf("gowdk db: driver name is required")
	}
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("gowdk db: data source name is required")
	}

	database, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("gowdk db: open %q: %w", driver, err)
	}
	if options.MaxOpenConns > 0 {
		database.SetMaxOpenConns(options.MaxOpenConns)
	}
	if options.MaxIdleConns > 0 {
		database.SetMaxIdleConns(options.MaxIdleConns)
	}
	if options.ConnMaxLifetime > 0 {
		database.SetConnMaxLifetime(options.ConnMaxLifetime)
	}

	pingTimeout := options.PingTimeout
	if pingTimeout <= 0 {
		pingTimeout = DefaultPingTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if err := database.PingContext(ctx); err != nil {
		database.Close()
		return nil, fmt.Errorf("gowdk db: ping %q: %w", driver, err)
	}
	return database, nil
}
