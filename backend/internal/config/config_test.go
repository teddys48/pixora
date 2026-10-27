package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pixora/backend/internal/config"
)

func TestConfigDefaultsProduction(t *testing.T) {
	// Clear relevant env vars
	os.Unsetenv("STORAGE_ROOT")
	os.Unsetenv("DATABASE_PATH")
	os.Setenv("APP_ENV", "production")
	defer os.Unsetenv("APP_ENV")

	cfg := config.Load()

	if cfg.StorageRoot != "/media" {
		t.Errorf("Expected production StorageRoot to be /media, got %s", cfg.StorageRoot)
	}

	if cfg.DatabasePath != "/data/pixora.db" {
		t.Errorf("Expected production DatabasePath to be /data/pixora.db, got %s", cfg.DatabasePath)
	}

	if cfg.OriginalsDir() != "/media/originals" {
		t.Errorf("Expected OriginalsDir to be /media/originals, got %s", cfg.OriginalsDir())
	}

	if cfg.ThumbnailsDir() != "/media/thumbnails" {
		t.Errorf("Expected ThumbnailsDir to be /media/thumbnails, got %s", cfg.ThumbnailsDir())
	}

	if cfg.TmpDir() != "/media/tmp" {
		t.Errorf("Expected TmpDir to be /media/tmp, got %s", cfg.TmpDir())
	}
}

func TestConfigEnvironmentOverrides(t *testing.T) {
	os.Setenv("STORAGE_ROOT", "/custom/photos")
	os.Setenv("DATABASE_PATH", "/custom/db/app.db")
	defer func() {
		os.Unsetenv("STORAGE_ROOT")
		os.Unsetenv("DATABASE_PATH")
	}()

	cfg := config.Load()

	if cfg.StorageRoot != "/custom/photos" {
		t.Errorf("Expected StorageRoot to be /custom/photos, got %s", cfg.StorageRoot)
	}

	if cfg.DatabasePath != "/custom/db/app.db" {
		t.Errorf("Expected DatabasePath to be /custom/db/app.db, got %s", cfg.DatabasePath)
	}
}

func TestResolveStoragePath(t *testing.T) {
	cfg := &config.Config{
		StorageRoot: "/media",
	}

	// Valid relative paths
	path, err := cfg.ResolveStoragePath("originals/2026/10/photo1.jpg")
	if err != nil {
		t.Fatalf("Unexpected error for valid relative path: %v", err)
	}
	if path != "/media/originals/2026/10/photo1.jpg" {
		t.Errorf("Expected /media/originals/2026/10/photo1.jpg, got %s", path)
	}

	// Traversal attempt should be rejected
	_, err = cfg.ResolveStoragePath("../etc/passwd")
	if err == nil {
		t.Errorf("Expected error for traversal path, got nil")
	}

	// Absolute path should be rejected
	_, err = cfg.ResolveStoragePath("/etc/passwd")
	if err == nil {
		t.Errorf("Expected error for absolute path, got nil")
	}
}

func TestCheckWritable(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pixora_writable_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Writable directory should pass
	if err := config.CheckWritable(tempDir); err != nil {
		t.Errorf("Expected writable directory to pass, got error: %v", err)
	}

	// Non-existent directory should fail
	nonExistent := filepath.Join(tempDir, "does_not_exist")
	if err := config.CheckWritable(nonExistent); err == nil {
		t.Errorf("Expected error for non-existent directory, got nil")
	}

	// Read-only directory should fail
	roDir := filepath.Join(tempDir, "read_only")
	if err := os.Mkdir(roDir, 0555); err != nil {
		t.Fatalf("Failed to create read-only dir: %v", err)
	}
	defer os.Chmod(roDir, 0755) // restore for cleanup

	errRO := config.CheckWritable(roDir)
	if errRO == nil {
		t.Errorf("Expected error for read-only directory, got nil")
	} else if !strings.Contains(errRO.Error(), "not writable") && !strings.Contains(errRO.Error(), "permission denied") {
		t.Logf("Got expected permission failure: %v", errRO)
	}
}
