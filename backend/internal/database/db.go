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
// enables foreign keys, configures connection pooling, and applies pending schema migrations.
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
		dsn = "file::memory:?cache=shared&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	} else {
		dsn = fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)", cleanPath)
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database at %q: %w", cleanPath, err)
	}

	// SQLite connection pooling
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

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

// PingContext checks database connectivity with a timeout.
func (db *DB) PingContext(ctx context.Context) error {
	var one int
	err := db.QueryRowContext(ctx, "SELECT 1").Scan(&one)
	if err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	return nil
}
