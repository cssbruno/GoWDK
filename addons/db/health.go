package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Readiness reports the outcome of a database readiness check.
type Readiness struct {
	Ready     bool          `json:"ready"`
	CheckedAt time.Time     `json:"checkedAt"`
	Duration  time.Duration `json:"duration"`
	Error     string        `json:"error,omitempty"`
}

// Ping verifies database connectivity through database/sql.
func Ping(ctx context.Context, database *sql.DB) error {
	if database == nil {
		return fmt.Errorf("gowdk db: database is required")
	}
	if err := database.PingContext(ctx); err != nil {
		return fmt.Errorf("gowdk db: ping: %w", err)
	}
	return nil
}

// CheckReadiness returns a small framework-neutral readiness snapshot for a
// generated app endpoint or startup check.
func CheckReadiness(ctx context.Context, database *sql.DB) Readiness {
	start := time.Now()
	result := Readiness{
		CheckedAt: start.UTC(),
	}
	if err := Ping(ctx, database); err != nil {
		result.Duration = time.Since(start)
		result.Error = err.Error()
		return result
	}
	result.Ready = true
	result.Duration = time.Since(start)
	return result
}
