package database_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pixora/backend/internal/database"
)

func TestOpenInNonWritableDir(t *testing.T) {
	tmp, err := os.MkdirTemp("", "ro_db_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	roDir := filepath.Join(tmp, "ro")
	if err := os.Mkdir(roDir, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(roDir, 0755)

	dbPath := filepath.Join(roDir, "pixora.db")
	_, err = database.Open(dbPath)
	if err == nil {
		t.Errorf("Expected error opening db in non-writable directory, got nil")
	}
	if !strings.Contains(err.Error(), "not writable") && !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("Expected permission denied error message, got: %v", err)
	}
}

func TestDatabaseInitializationAndMigrations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pixora_db_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "nested", "dir", "pixora_test.db")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("Expected database.Open to succeed and create nested parent directory, got: %v", err)
	}
	defer db.Close()

	// Verify database file was created
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("Expected db file to exist at %s, got: %v", dbPath, err)
	}

	// Verify schema_migrations has version 1, 2, and 3 recorded
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version IN (1, 2, 3)").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query schema_migrations: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected migrations 1, 2, and 3 to be recorded, got count %d", count)
	}

	// Verify tables exist
	tables := []string{"photos", "albums", "photo_albums", "schema_migrations"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("Expected table %q to exist in database, got error: %v", table, err)
		}
	}

	// Verify indexes exist
	indexes := []string{
		"idx_photos_taken_at",
		"idx_photos_created_at",
		"idx_photos_relative_path",
		"idx_photos_original_filename_nocase",
		"idx_photos_media_type",
		"idx_photos_file_modified_at",
		"idx_photos_file_size",
		"idx_albums_name",
		"idx_albums_name_nocase",
		"idx_photo_albums_album_id",
		"idx_photo_albums_photo_id",
	}
	for _, idx := range indexes {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx).Scan(&name)
		if err != nil {
			t.Errorf("Expected index %q to exist, got error: %v", idx, err)
		}
	}
}

func TestDatabaseForeignKeyEnforcement(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pixora_fk_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "fk_test.db")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Verify foreign_keys PRAGMA is 1 (ON)
	var fkStatus int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fkStatus); err != nil {
		t.Fatalf("Failed to query PRAGMA foreign_keys: %v", err)
	}
	if fkStatus != 1 {
		t.Fatalf("Expected PRAGMA foreign_keys to be 1, got %d", fkStatus)
	}

	// Attempt to insert photo_album with non-existent photo and album IDs
	_, err = db.Exec(
		"INSERT INTO photo_albums (photo_id, album_id, added_at) VALUES (?, ?, ?)",
		"non-existent-photo", "non-existent-album", time.Now().UTC(),
	)
	if err == nil {
		t.Fatalf("Expected foreign key violation error, got nil")
	}
	if !strings.Contains(err.Error(), "FOREIGN KEY") && !strings.Contains(err.Error(), "foreign key") {
		t.Errorf("Expected foreign key violation error message, got: %v", err)
	}

	// Insert valid photo and album
	photoID := "p-1"
	albumID := "a-1"
	now := time.Now().UTC()

	_, err = db.Exec(`
		INSERT INTO photos (id, relative_path, original_filename, media_type, file_size, file_modified_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		photoID, "originals/p1.jpg", "p1.jpg", "image/jpeg", 1024, now, now, now,
	)
	if err != nil {
		t.Fatalf("Failed to insert photo: %v", err)
	}

	_, err = db.Exec(`INSERT INTO albums (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		albumID, "Vacation", now, now,
	)
	if err != nil {
		t.Fatalf("Failed to insert album: %v", err)
	}

	// Insert into join table should now succeed
	_, err = db.Exec(
		"INSERT INTO photo_albums (photo_id, album_id, added_at) VALUES (?, ?, ?)",
		photoID, albumID, now,
	)
	if err != nil {
		t.Fatalf("Failed to insert photo_album: %v", err)
	}

	// Verify cascade deletion: deleting photo should delete join record
	_, err = db.Exec("DELETE FROM photos WHERE id = ?", photoID)
	if err != nil {
		t.Fatalf("Failed to delete photo: %v", err)
	}

	var joinCount int
	err = db.QueryRow("SELECT COUNT(*) FROM photo_albums WHERE photo_id = ?", photoID).Scan(&joinCount)
	if err != nil {
		t.Fatalf("Failed to query photo_albums: %v", err)
	}
	if joinCount != 0 {
		t.Errorf("Expected cascade deletion in photo_albums, got count %d", joinCount)
	}
}

func TestDatabaseUniqueConstraints(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pixora_unique_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	db, err := database.Open(filepath.Join(tempDir, "unique_test.db"))
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	now := time.Now().UTC()
	// Insert first photo
	_, err = db.Exec(`
		INSERT INTO photos (id, relative_path, original_filename, media_type, file_size, file_modified_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"photo-1", "originals/same_path.jpg", "same_path.jpg", "image/jpeg", 500, now, now, now,
	)
	if err != nil {
		t.Fatalf("Failed to insert photo: %v", err)
	}

	// Insert second photo with duplicate relative_path must fail
	_, err = db.Exec(`
		INSERT INTO photos (id, relative_path, original_filename, media_type, file_size, file_modified_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"photo-2", "originals/same_path.jpg", "another_name.jpg", "image/jpeg", 600, now, now, now,
	)
	if err == nil {
		t.Fatalf("Expected unique constraint violation on relative_path, got nil")
	}
	if !strings.Contains(err.Error(), "UNIQUE") && !strings.Contains(err.Error(), "unique") {
		t.Errorf("Expected UNIQUE constraint error, got: %v", err)
	}

	// Test album case-insensitive unique name constraint
	_, err = db.Exec(`INSERT INTO albums (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		"alb-1", "Holidays", now, now,
	)
	if err != nil {
		t.Fatalf("Failed to insert first album: %v", err)
	}

	// Insert duplicate album name with different casing must fail
	_, err = db.Exec(`INSERT INTO albums (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		"alb-2", "holidays", now, now,
	)
	if err == nil {
		t.Fatalf("Expected unique constraint violation on case-insensitive album name, got nil")
	}
	if !strings.Contains(err.Error(), "UNIQUE") && !strings.Contains(err.Error(), "unique") {
		t.Errorf("Expected UNIQUE constraint error for duplicate album name, got: %v", err)
	}
}

func TestDatabasePingAndClose(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pixora_ping_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	db, err := database.Open(filepath.Join(tempDir, "ping_test.db"))
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Errorf("PingContext failed on open database: %v", err)
	}

	// Close database
	if err := db.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Ping after close should fail
	if err := db.PingContext(ctx); err == nil {
		t.Errorf("Expected ping to fail on closed database, got nil")
	}
}
