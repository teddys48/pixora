package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds the centralized application configuration.
type Config struct {
	Port         string
	Host         string
	StaticDir    string
	Env          string
	StorageRoot  string // Root directory for photo storage (default: /media in container)
	DatabasePath string // Path to SQLite database file (default: /data/pixora.db in container)
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	host := os.Getenv("HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		if env == "production" {
			staticDir = "/app/public"
		} else if _, err := os.Stat("./public"); err == nil {
			staticDir = "./public"
		} else if _, err := os.Stat("../frontend/dist"); err == nil {
			staticDir = "../frontend/dist"
		} else {
			staticDir = "./public"
		}
	}

	// Storage Root: photo storage directory (host bind-mount target)
	storageRoot := os.Getenv("STORAGE_ROOT")
	if storageRoot == "" {
		if env == "production" {
			storageRoot = "/media"
		} else if _, err := os.Stat("/media"); err == nil {
			storageRoot = "/media"
		} else {
			storageRoot = "./storage/media"
		}
	}

	// Database Path: application database (separate persistent volume target)
	databasePath := os.Getenv("DATABASE_PATH")
	if databasePath == "" {
		if env == "production" {
			databasePath = "/data/pixora.db"
		} else if _, err := os.Stat("/data"); err == nil {
			databasePath = "/data/pixora.db"
		} else {
			databasePath = "./storage/data/pixora.db"
		}
	}

	return &Config{
		Port:         strings.TrimSpace(port),
		Host:         strings.TrimSpace(host),
		StaticDir:    filepath.Clean(strings.TrimSpace(staticDir)),
		Env:          strings.TrimSpace(env),
		StorageRoot:  filepath.Clean(strings.TrimSpace(storageRoot)),
		DatabasePath: filepath.Clean(strings.TrimSpace(databasePath)),
	}
}

// Validate verifies that required configuration values are present, valid, and safe.
func (c *Config) Validate() error {
	if c.Port == "" {
		return fmt.Errorf("PORT cannot be empty")
	}
	portNum, err := strconv.Atoi(c.Port)
	if err != nil || portNum < 1 || portNum > 65535 {
		return fmt.Errorf("invalid PORT %q: must be an integer between 1 and 65535", c.Port)
	}

	if c.Host == "" {
		return fmt.Errorf("HOST cannot be empty")
	}

	if c.StorageRoot == "" || c.StorageRoot == "." {
		return fmt.Errorf("STORAGE_ROOT cannot be empty or relative current directory")
	}
	if strings.ContainsRune(c.StorageRoot, 0) {
		return fmt.Errorf("STORAGE_ROOT contains illegal null byte")
	}

	if c.DatabasePath == "" || c.DatabasePath == "." {
		return fmt.Errorf("DATABASE_PATH cannot be empty or relative current directory")
	}
	if strings.ContainsRune(c.DatabasePath, 0) {
		return fmt.Errorf("DATABASE_PATH contains illegal null byte")
	}

	return nil
}

// OriginalsDir returns the path for original photo storage.
func (c *Config) OriginalsDir() string {
	return filepath.Join(c.StorageRoot, "originals")
}

// ThumbnailsDir returns the path for generated thumbnail storage.
func (c *Config) ThumbnailsDir() string {
	return filepath.Join(c.StorageRoot, "thumbnails")
}

// TmpDir returns the path for temporary upload processing.
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
	relToRoot, err := filepath.Rel(c.StorageRoot, target)
	if err != nil || strings.HasPrefix(relToRoot, "..") || (relToRoot == "." && relativePath != ".") {
		return "", fmt.Errorf("path traversal detected: %s", relativePath)
	}

	return target, nil
}

// CheckWritable verifies whether a directory is actually writable by testing
// file creation. A directory existing does not imply write permissions.
// NOTE: This must not be called during regular application startup against StorageRoot.
func CheckWritable(dirPath string) error {
	info, err := os.Stat(dirPath)
	if err != nil {
		return fmt.Errorf("directory check failed: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", dirPath)
	}

	tmpFile, err := os.CreateTemp(dirPath, ".pixora_perm_check_*")
	if err != nil {
		return fmt.Errorf("directory is not writable: %w", err)
	}
	tmpName := tmpFile.Name()
	_ = tmpFile.Close()
	_ = os.Remove(tmpName)

	return nil
}
