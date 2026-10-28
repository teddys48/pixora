package database

import (
	"fmt"
	"time"
)

// Migration represents a versioned database schema migration.
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// migrations lists all sequential database migrations.
var migrations = []Migration{
	{
		Version: 1,
		Name:    "create_photos_albums_tables",
		SQL: `
CREATE TABLE IF NOT EXISTS photos (
    id TEXT PRIMARY KEY,
    relative_path TEXT NOT NULL UNIQUE,
    original_filename TEXT NOT NULL,
    media_type TEXT NOT NULL,
    file_size INTEGER NOT NULL,
    width INTEGER,
    height INTEGER,
    taken_at DATETIME,
    file_modified_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_photos_taken_at ON photos(taken_at DESC);
CREATE INDEX IF NOT EXISTS idx_photos_created_at ON photos(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_photos_relative_path ON photos(relative_path);

CREATE TABLE IF NOT EXISTS albums (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_albums_name ON albums(name);

CREATE TABLE IF NOT EXISTS photo_albums (
    photo_id TEXT NOT NULL REFERENCES photos(id) ON DELETE CASCADE,
    album_id TEXT NOT NULL REFERENCES albums(id) ON DELETE CASCADE,
    added_at DATETIME NOT NULL,
    PRIMARY KEY (photo_id, album_id)
);

CREATE INDEX IF NOT EXISTS idx_photo_albums_album_id ON photo_albums(album_id);
CREATE INDEX IF NOT EXISTS idx_photo_albums_photo_id ON photo_albums(photo_id);
`,
	},
	{
		Version: 2,
		Name:    "case_insensitive_unique_album_names",
		SQL: `
CREATE UNIQUE INDEX IF NOT EXISTS idx_albums_name_nocase ON albums(name COLLATE NOCASE);
`,
	},
}

// Migrate applies all unapplied database migrations in transactional batches.
func (db *DB) Migrate() error {
	// Ensure migration tracking table exists
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at DATETIME NOT NULL
);`)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	// Fetch already applied migrations
	rows, err := db.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return fmt.Errorf("failed to scan migration version: %w", err)
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error reading applied migrations: %w", err)
	}

	// Apply pending migrations in order
	for _, m := range migrations {
		if applied[m.Version] {
			continue
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %d (%s): %w", m.Version, m.Name, err)
		}

		if _, err := tx.Exec(m.SQL); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %d (%s) failed: %w", m.Version, m.Name, err)
		}

		_, err = tx.Exec(
			"INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)",
			m.Version, m.Name, time.Now().UTC(),
		)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to record migration %d (%s): %w", m.Version, m.Name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %d (%s): %w", m.Version, m.Name, err)
		}
	}

	return nil
}
