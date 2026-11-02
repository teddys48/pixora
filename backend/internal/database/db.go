package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps sql.DB with Pixora-specific configuration and methods.
type DB struct {
	*sql.DB
	path string
}

// Open initializes a SQLite database connection at the specified path.
// It creates the parent directory if needed, verifies write permissions, sets WAL journal mode,
// enables foreign keys, configures optimized connection pooling and pragmas, and applies pending schema migrations.
func Open(dbPath string) (*DB, error) {
	cleanPath := filepath.Clean(strings.TrimSpace(dbPath))
	if cleanPath == "" || cleanPath == "." {
		return nil, fmt.Errorf("invalid database path %q", dbPath)
	}

	isMemory := cleanPath == ":memory:"
	if !isMemory {
		parentDir := filepath.Dir(cleanPath)
		if parentDir != "" && parentDir != "." {
			if err := os.MkdirAll(parentDir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create database directory %q: %w", parentDir, err)
			}

			// Pre-flight check: verify parent directory is writable by the current process user
			testFile, err := os.CreateTemp(parentDir, ".pixora_db_perm_check_*")
			if err != nil {
				return nil, fmt.Errorf("database directory %q is not writable (permission denied, UID %d): ensure host directory is owned by UID 1000 ('sudo chown -R 1000:1000 <dir>') or use a Docker named volume: %w", parentDir, os.Getuid(), err)
			}
			tmpName := testFile.Name()
			_ = testFile.Close()
			_ = os.Remove(tmpName)
		}

		// If database file already exists, check if it is writable
		if info, err := os.Stat(cleanPath); err == nil && !info.IsDir() {
			f, err := os.OpenFile(cleanPath, os.O_RDWR, 0644)
			if err != nil {
				return nil, fmt.Errorf("database file %q is not writable (permission denied, UID %d): ensure file is owned by UID 1000 ('sudo chown -R 1000:1000 <dir>'): %w", cleanPath, os.Getuid(), err)
			}
			_ = f.Close()
		}
	}

	var dsn string
	if isMemory {
		dsn = "file::memory:?cache=shared&_pragma=foreign_keys(1)&_pragma=busy_timeout(10000)"
	} else {
		// Production SQLite Pragmas:
		// - WAL journal mode: concurrent readers do not block writers and writer does not block readers
		// - synchronous=NORMAL: full ACID durability in WAL mode with lower sync latency
		// - busy_timeout=10000: wait up to 10s on locked database before throwing busy error
		// - cache_size=-64000: 64MB memory page cache for fast index traversal
		// - temp_store=2 (MEMORY): avoid temporary disk files for sorting and subqueries
		// - mmap_size=268435456 (256MB): memory-mapped file access for fast reads
		dsn = fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-64000)&_pragma=temp_store(2)&_pragma=mmap_size(268435456)", cleanPath)
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database at %q: %w", cleanPath, err)
	}

	// SQLite connection pooling:
	// Limit maximum open connections to prevent connection thrashing and lock contention.
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(15 * time.Minute)

	// Explicitly enforce foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	wrapper := &DB{
		DB:   db,
		path: cleanPath,
	}

	// Apply schema migrations
	if err := wrapper.Migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("database migration failed: %w", err)
	}

	return wrapper, nil
}

// Path returns the database file path.
func (db *DB) Path() string {
	return db.path
}

// Checkpoint performs a WAL checkpoint to transfer write-ahead log pages back to the database file.
func (db *DB) Checkpoint(mode string) error {
	if mode == "" {
		mode = "PASSIVE"
	}
	_, err := db.Exec(fmt.Sprintf("PRAGMA wal_checkpoint(%s);", mode))
	return err
}

// Close cleanly checkpoints WAL and closes the database connection.
func (db *DB) Close() error {
	// Best-effort passive checkpoint before closing to flush pages to disk cleanly
	_ = db.Checkpoint("PASSIVE")
	return db.DB.Close()
}

// PingContext checks database connectivity with a timeout.
func (db *DB) PingContext(ctx context.Context) error {
	var one int
	err := db.QueryRowContext(ctx, "SELECT 1").Scan(&one)
	if err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	return nil
}
