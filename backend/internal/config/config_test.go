package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pixora/backend/internal/config"
)

func TestConfigDefaultsProduction(t *testing.T) {
	os.Unsetenv("STORAGE_ROOT")
	os.Unsetenv("DATABASE_PATH")
	os.Unsetenv("PORT")
	os.Unsetenv("HOST")
	os.Unsetenv("STATIC_DIR")
	os.Setenv("APP_ENV", "production")
	defer os.Unsetenv("APP_ENV")

	cfg := config.Load()

	if cfg.Port != "8080" {
		t.Errorf("Expected production Port to be 8080, got %s", cfg.Port)
	}

	if cfg.Host != "0.0.0.0" {
		t.Errorf("Expected production Host to be 0.0.0.0, got %s", cfg.Host)
	}

	if cfg.StaticDir != "/app/public" {
		t.Errorf("Expected production StaticDir to be /app/public, got %s", cfg.StaticDir)
	}

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

	if err := cfg.Validate(); err != nil {
		t.Errorf("Expected default production config to be valid, got: %v", err)
	}
}

func TestConfigEnvironmentOverrides(t *testing.T) {
	os.Setenv("PORT", "9000")
	os.Setenv("HOST", "127.0.0.1")
	os.Setenv("STORAGE_ROOT", "/custom/photos")
	os.Setenv("DATABASE_PATH", "/custom/db/app.db")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("HOST")
		os.Unsetenv("STORAGE_ROOT")
		os.Unsetenv("DATABASE_PATH")
	}()

	cfg := config.Load()

	if cfg.Port != "9000" {
		t.Errorf("Expected Port to be 9000, got %s", cfg.Port)
	}

	if cfg.Host != "127.0.0.1" {
		t.Errorf("Expected Host to be 127.0.0.1, got %s", cfg.Host)
	}

	if cfg.StorageRoot != "/custom/photos" {
		t.Errorf("Expected StorageRoot to be /custom/photos, got %s", cfg.StorageRoot)
	}

	if cfg.DatabasePath != "/custom/db/app.db" {
		t.Errorf("Expected DatabasePath to be /custom/db/app.db, got %s", cfg.DatabasePath)
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Expected valid overridden config, got: %v", err)
	}
}

func TestConfigValidationInvalidValues(t *testing.T) {
	baseCfg := func() *config.Config {
		return &config.Config{
			Port:         "8080",
			Host:         "0.0.0.0",
			StorageRoot:  "/media",
			DatabasePath: "/data/pixora.db",
		}
	}

	tests := []struct {
		name      string
		modify    func(c *config.Config)
		wantError string
	}{
		{
			name: "empty port",
			modify: func(c *config.Config) {
				c.Port = ""
			},
			wantError: "PORT cannot be empty",
		},
		{
			name: "non-numeric port",
			modify: func(c *config.Config) {
				c.Port = "abc"
			},
			wantError: "invalid PORT",
		},
		{
			name: "negative port",
			modify: func(c *config.Config) {
				c.Port = "-1"
			},
			wantError: "invalid PORT",
		},
		{
			name: "out of range port",
			modify: func(c *config.Config) {
				c.Port = "70000"
			},
			wantError: "invalid PORT",
		},
		{
			name: "empty host",
			modify: func(c *config.Config) {
				c.Host = ""
			},
			wantError: "HOST cannot be empty",
		},
		{
			name: "empty storage root",
			modify: func(c *config.Config) {
				c.StorageRoot = ""
			},
			wantError: "STORAGE_ROOT cannot be empty",
		},
		{
			name: "empty database path",
			modify: func(c *config.Config) {
				c.DatabasePath = ""
			},
			wantError: "DATABASE_PATH cannot be empty",
		},
		{
			name: "null byte in storage root",
			modify: func(c *config.Config) {
				c.StorageRoot = "/media\x00/evil"
			},
			wantError: "null byte",
		},
		{
			name: "null byte in database path",
			modify: func(c *config.Config) {
				c.DatabasePath = "/data\x00/pixora.db"
			},
			wantError: "null byte",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseCfg()
			tt.modify(cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Expected validation error for %s, got nil", tt.name)
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("Expected error containing %q, got %q", tt.wantError, err.Error())
			}
		})
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
