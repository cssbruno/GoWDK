package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// DefaultMigrationTable is the table used to track applied migrations when
// MigrationOptions.Table is empty.
const DefaultMigrationTable = "gowdk_schema_migrations"

const (
	migrationNameMaxLength       = 255
	migrationPendingChecksumFlag = "pending:"
)

var migrationTablePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// PlaceholderFunc returns the SQL placeholder for a 1-based bind position.
type PlaceholderFunc func(position int) string

// MigrationOptions configures ApplyMigrations.
type MigrationOptions struct {
	// Dir selects a directory inside Source. Empty means ".".
	Dir string
	// Table names the migration tracking table. Empty uses
	// DefaultMigrationTable. Only simple SQL identifiers are accepted.
	Table string
	// Placeholder adapts bind placeholders for the target driver. Empty uses
	// QuestionPlaceholder. Use DollarPlaceholder for PostgreSQL-style drivers.
	Placeholder PlaceholderFunc
	// Now overrides the applied_at timestamp source for tests.
	Now func() time.Time
	// TxOptions are passed to database/sql BeginTx.
	TxOptions *sql.TxOptions
}

// MigrationRecord describes one migration file and checksum.
type MigrationRecord struct {
	Name     string
	Checksum string
}

// MigrationResult records which migration files were applied or already
// present with the same checksum.
type MigrationResult struct {
	Applied []MigrationRecord
	Skipped []MigrationRecord
}

// QuestionPlaceholder returns ? placeholders used by SQLite and MySQL-style
// drivers.
func QuestionPlaceholder(position int) string {
	return "?"
}

// DollarPlaceholder returns PostgreSQL-style numbered placeholders.
func DollarPlaceholder(position int) string {
	return fmt.Sprintf("$%d", position)
}

// ApplyMigrations applies .sql files from source in lexical path order. It is
// intentionally only a thin database/sql helper: files are user-owned SQL, and
// this helper tracks applied file names and checksums in one table.
func ApplyMigrations(ctx context.Context, database *sql.DB, source fs.FS, options MigrationOptions) (MigrationResult, error) {
	if database == nil {
		return MigrationResult{}, fmt.Errorf("gowdk db: database is required")
	}
	if source == nil {
		return MigrationResult{}, fmt.Errorf("gowdk db: migration source is required")
	}
	dir := strings.TrimSpace(options.Dir)
	if dir == "" {
		dir = "."
	}
	table := strings.TrimSpace(options.Table)
	if table == "" {
		table = DefaultMigrationTable
	}
	if !migrationTablePattern.MatchString(table) {
		return MigrationResult{}, fmt.Errorf("gowdk db: migration table %q is not a simple SQL identifier", table)
	}
	placeholder := options.Placeholder
	if placeholder == nil {
		placeholder = QuestionPlaceholder
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}

	files, err := migrationFiles(source, dir)
	if err != nil {
		return MigrationResult{}, err
	}

	var txResult MigrationResult
	err = WithTx(ctx, database, options.TxOptions, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, migrationTrackingDDL(table)); err != nil {
			return fmt.Errorf("gowdk db: prepare migration tracking table %q: %w", table, err)
		}
		for _, file := range files {
			record, statement, err := readMigration(source, file)
			if err != nil {
				return err
			}
			applied, err := migrationApplied(ctx, tx, table, placeholder, record)
			if err != nil {
				return err
			}
			if applied {
				txResult.Skipped = append(txResult.Skipped, record)
				continue
			}
			pendingChecksum := migrationPendingChecksum(record)
			if _, err := tx.ExecContext(ctx, migrationReserveSQL(table, placeholder), record.Name, pendingChecksum, now().UTC()); err != nil {
				return fmt.Errorf("gowdk db: reserve migration %q: %w", record.Name, err)
			}
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("gowdk db: apply migration %q: %w", record.Name, err)
			}
			result, err := tx.ExecContext(ctx, migrationFinalizeSQL(table, placeholder), record.Checksum, now().UTC(), record.Name, pendingChecksum)
			if err != nil {
				return fmt.Errorf("gowdk db: record migration %q: %w", record.Name, err)
			}
			if rows, err := result.RowsAffected(); err == nil && rows != 1 {
				return fmt.Errorf("gowdk db: record migration %q: reserved row was not finalized", record.Name)
			}
			txResult.Applied = append(txResult.Applied, record)
		}
		return nil
	})
	if err != nil {
		return MigrationResult{}, err
	}
	return txResult, nil
}

func migrationFiles(source fs.FS, dir string) ([]string, error) {
	var files []string
	if err := fs.WalkDir(source, dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("gowdk db: read migration path %q: %w", path, err)
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".sql") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func readMigration(source fs.FS, path string) (MigrationRecord, string, error) {
	payload, err := fs.ReadFile(source, path)
	if err != nil {
		return MigrationRecord{}, "", fmt.Errorf("gowdk db: read migration %q: %w", path, err)
	}
	sum := sha256.Sum256(payload)
	return MigrationRecord{
		Name:     path,
		Checksum: hex.EncodeToString(sum[:]),
	}, string(payload), nil
}

func migrationApplied(ctx context.Context, tx *sql.Tx, table string, placeholder PlaceholderFunc, record MigrationRecord) (bool, error) {
	if utf8.RuneCountInString(record.Name) > migrationNameMaxLength {
		return false, fmt.Errorf("gowdk db: migration name %q exceeds %d characters", record.Name, migrationNameMaxLength)
	}
	var stored string
	err := tx.QueryRowContext(ctx, migrationSelectSQL(table, placeholder), record.Name).Scan(&stored)
	if err == nil {
		if strings.HasPrefix(stored, migrationPendingChecksumFlag) {
			return false, fmt.Errorf("gowdk db: migration %q is reserved or incomplete", record.Name)
		}
		if stored != record.Checksum {
			return false, fmt.Errorf("gowdk db: migration %q checksum mismatch: applied %s, current %s", record.Name, stored, record.Checksum)
		}
		return true, nil
	}
	if err == sql.ErrNoRows {
		return false, nil
	}
	return false, fmt.Errorf("gowdk db: check migration %q: %w", record.Name, err)
}

func migrationPendingChecksum(record MigrationRecord) string {
	return migrationPendingChecksumFlag + record.Checksum
}

func migrationTrackingDDL(table string) string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (name VARCHAR(%d) PRIMARY KEY, checksum TEXT NOT NULL, applied_at TIMESTAMP NOT NULL)`, table, migrationNameMaxLength)
}

func migrationSelectSQL(table string, placeholder PlaceholderFunc) string {
	return fmt.Sprintf(`SELECT checksum FROM %s WHERE name = %s`, table, placeholder(1))
}

func migrationReserveSQL(table string, placeholder PlaceholderFunc) string {
	return fmt.Sprintf(`INSERT INTO %s (name, checksum, applied_at) VALUES (%s, %s, %s)`, table, placeholder(1), placeholder(2), placeholder(3))
}

func migrationFinalizeSQL(table string, placeholder PlaceholderFunc) string {
	return fmt.Sprintf(`UPDATE %s SET checksum = %s, applied_at = %s WHERE name = %s AND checksum = %s`, table, placeholder(1), placeholder(2), placeholder(3), placeholder(4))
}
