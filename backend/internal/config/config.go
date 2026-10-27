package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the application configuration
type Config struct {
	Port         string
	Host         string
	StaticDir    string
	Env          string
	StorageRoot  string // Root directory for photo storage (default: /media in container)
	DatabasePath string // Path to SQLite database file (default: /data/pixora.db in container)
}

// Load reads configuration from environment variables with sensible defaults
func Load() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	host := os.Getenv("HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		// In production container, static assets are in ./public
		// If running locally from repo root or backend dir, fallback gracefully
		if _, err := os.Stat("./public"); err == nil {
			staticDir = "./public"
		} else if _, err := os.Stat("../frontend/dist"); err == nil {
			staticDir = "../frontend/dist"
		} else {
			staticDir = "./public"
		}
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	// Storage Root: photo storage directory (host bind-mount target)
	storageRoot := os.Getenv("STORAGE_ROOT")
	if storageRoot == "" {
		if env == "production" {
			storageRoot = "/media"
		} else {
			// In local dev, use local path if /media is absent
			if _, err := os.Stat("/media"); err == nil {
				storageRoot = "/media"
			} else {
				storageRoot = "./storage/media"
			}
		}
	}

	// Database Path: application database (separate persistent volume target)
	databasePath := os.Getenv("DATABASE_PATH")
	if databasePath == "" {
		if env == "production" {
			databasePath = "/data/pixora.db"
		} else {
			// In local dev, use local path if /data is absent
			if _, err := os.Stat("/data"); err == nil {
				databasePath = "/data/pixora.db"
			} else {
				databasePath = "./storage/data/pixora.db"
			}
		}
	}

	return &Config{
		Port:         port,
		Host:         host,
		StaticDir:    staticDir,
		Env:          env,
		StorageRoot:  filepath.Clean(storageRoot),
		DatabasePath: filepath.Clean(databasePath),
	}
}

// OriginalsDir returns the absolute path for original photo storage
func (c *Config) OriginalsDir() string {
	return filepath.Join(c.StorageRoot, "originals")
}

// ThumbnailsDir returns the absolute path for generated thumbnail storage
func (c *Config) ThumbnailsDir() string {
	return filepath.Join(c.StorageRoot, "thumbnails")
}

// TmpDir returns the absolute path for temporary upload processing
func (c *Config) TmpDir() string {
	return filepath.Join(c.StorageRoot, "tmp")
}

// ResolveStoragePath resolves a relative file path under StorageRoot safely,
// rejecting path traversal attempts and returning an error if escaping StorageRoot.
func (c *Config) ResolveStoragePath(relativePath string) (string, error) {
	cleanRel := filepath.Clean(relativePath)
	if filepath.IsAbs(cleanRel) {
		return "", fmt.Errorf("storage paths must be relative, got absolute: %s", relativePath)
	}

	target := filepath.Join(c.StorageRoot, cleanRel)
	// Ensure target is strictly within StorageRoot
	relToRoot, err := filepath.Rel(c.StorageRoot, target)
	if err != nil || strings.HasPrefix(relToRoot, "..") || relToRoot == "." && relativePath != "." {
		return "", fmt.Errorf("path traversal detected: %s", relativePath)
	}

	return target, nil
}

// CheckWritable verifies whether a directory is actually writable by testing
// file creation. A directory existing does not imply write permissions.
func CheckWritable(dirPath string) error {
	info, err := os.Stat(dirPath)
	if err != nil {
		return fmt.Errorf("directory check failed: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", dirPath)
	}

	// Test write access by creating a temporary sentinel file
	tmpFile, err := os.CreateTemp(dirPath, ".pixora_perm_check_*")
	if err != nil {
		return fmt.Errorf("directory is not writable: %w", err)
	}
	tmpName := tmpFile.Name()
	_ = tmpFile.Close()
	_ = os.Remove(tmpName)

	return nil
}
