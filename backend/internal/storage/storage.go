package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrPathTraversal is returned when a path attempts to escape the storage root.
	ErrPathTraversal = errors.New("path traversal detected")

	// ErrInvalidPath is returned when a path is empty or contains invalid characters.
	ErrInvalidPath = errors.New("invalid storage path")

	// ErrStorageRootNotFound is returned when the configured storage root does not exist.
	ErrStorageRootNotFound = errors.New("storage root directory does not exist")

	// ErrReadOnly is returned when an operation requiring write access is attempted on read-only storage.
	ErrReadOnly = errors.New("storage is read-only")

	// ErrPermissionDenied is returned when filesystem permissions prevent accessing the storage.
	ErrPermissionDenied = errors.New("storage permission denied")
)

// Storage provides safe file access and path resolution within a configured root directory.
type Storage struct {
	root string
}

// New creates a new Storage instance pointing to the given storage root.
// It does not create or modify files during initialization.
func New(root string) (*Storage, error) {
	clean := filepath.Clean(strings.TrimSpace(root))
	if clean == "" || clean == "." {
		return nil, fmt.Errorf("invalid storage root %q", root)
	}
	return &Storage{root: clean}, nil
}

// Root returns the configured storage root path.
func (s *Storage) Root() string {
	return s.root
}

// ThumbnailsDir returns the directory path intended for generated thumbnails.
func (s *Storage) ThumbnailsDir() string {
	return filepath.Join(s.root, "thumbnails")
}

// TmpDir returns the directory path intended for temporary processing.
func (s *Storage) TmpDir() string {
	return filepath.Join(s.root, "tmp")
}

// ValidateRelativePath validates that a relative path does not escape the storage root or contain illegal characters.
func (s *Storage) ValidateRelativePath(relPath string) error {
	trimmed := strings.TrimSpace(relPath)
	if trimmed == "" {
		return fmt.Errorf("%w: path is empty", ErrInvalidPath)
	}
	if strings.ContainsRune(trimmed, 0) {
		return fmt.Errorf("%w: path contains null byte", ErrInvalidPath)
	}
	if filepath.IsAbs(trimmed) || strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "\\") {
		return fmt.Errorf("%w: absolute paths are not permitted: %s", ErrPathTraversal, relPath)
	}

	clean := filepath.Clean(trimmed)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%w: path traversal detected: %s", ErrPathTraversal, relPath)
	}

	return nil
}

// ResolvePath securely validates and resolves a relative path under the StorageRoot.
// It prevents lexical traversal (..) and symlink escapes pointing outside StorageRoot.
func (s *Storage) ResolvePath(relPath string) (string, error) {
	if err := s.ValidateRelativePath(relPath); err != nil {
		return "", err
	}

	cleanRel := filepath.Clean(relPath)
	target := filepath.Join(s.root, cleanRel)

	relToRoot, err := filepath.Rel(s.root, target)
	if err != nil || strings.HasPrefix(relToRoot, "..") || (relToRoot == "." && cleanRel != ".") {
		return "", fmt.Errorf("%w: path escapes storage root: %s", ErrPathTraversal, relPath)
	}

	// Symlink escape check:
	// If root exists, check realpath of root and target
	if realRoot, err := filepath.EvalSymlinks(s.root); err == nil {
		if realTarget, err := filepath.EvalSymlinks(target); err == nil {
			relReal, err := filepath.Rel(realRoot, realTarget)
			if err != nil || strings.HasPrefix(relReal, "..") {
				return "", fmt.Errorf("%w: symlink target escapes storage root: %s", ErrPathTraversal, relPath)
			}
		} else {
			// If target doesn't exist yet, evaluate symlinks on the nearest existing parent directory
			curr := filepath.Dir(target)
			for curr != "" && curr != "." && curr != "/" {
				if realCurr, err := filepath.EvalSymlinks(curr); err == nil {
					relReal, err := filepath.Rel(realRoot, realCurr)
					if err != nil || strings.HasPrefix(relReal, "..") {
						return "", fmt.Errorf("%w: symlink parent directory escapes storage root: %s", ErrPathTraversal, relPath)
					}
					break
				}
				parent := filepath.Dir(curr)
				if parent == curr {
					break
				}
				curr = parent
			}
		}
	}

	return target, nil
}

// CheckReadAccess verifies if the storage root exists and is readable.
func (s *Storage) CheckReadAccess() error {
	info, err := os.Stat(s.root)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrStorageRootNotFound, s.root)
		}
		if os.IsPermission(err) {
			return fmt.Errorf("%w: %s: %w", ErrPermissionDenied, s.root, err)
		}
		return fmt.Errorf("failed to access storage root %s: %w", s.root, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("storage root is not a directory: %s", s.root)
	}

	f, err := os.Open(s.root)
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("%w: cannot read storage root %s", ErrPermissionDenied, s.root)
		}
		return fmt.Errorf("storage root read error: %w", err)
	}
	_ = f.Close()

	return nil
}

// CheckWriteAccess verifies whether storage root currently has write permissions.
// NOTE: This must NOT be called at application startup. Call only when a write operation is initiated.
func (s *Storage) CheckWriteAccess() error {
	if err := s.CheckReadAccess(); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(s.root, ".pixora_perm_check_*")
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("%w: %s", ErrReadOnly, s.root)
		}
		return fmt.Errorf("%w: %s (%v)", ErrReadOnly, s.root, err)
	}
	tmpName := tmpFile.Name()
	_ = tmpFile.Close()
	_ = os.Remove(tmpName)

	return nil
}

// OpenFile opens an existing file under storage root for reading.
func (s *Storage) OpenFile(relPath string) (io.ReadCloser, error) {
	resolved, err := s.ResolvePath(relPath)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("%w: %s", ErrPermissionDenied, relPath)
		}
		return nil, err
	}
	return f, nil
}

// Stat returns FileInfo for a file under storage root.
func (s *Storage) Stat(relPath string) (os.FileInfo, error) {
	resolved, err := s.ResolvePath(relPath)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("%w: %s", ErrPermissionDenied, relPath)
		}
		return nil, err
	}
	return info, nil
}

// CreateFile securely creates or truncates a file under storage root.
// Fails with ErrReadOnly if the storage destination cannot be written to.
func (s *Storage) CreateFile(relPath string) (io.WriteCloser, error) {
	resolved, err := s.ResolvePath(relPath)
	if err != nil {
		return nil, err
	}

	parent := filepath.Dir(resolved)
	if err := os.MkdirAll(parent, 0755); err != nil {
		if os.IsPermission(err) {
			return nil, fmt.Errorf("%w: cannot create directory %s", ErrReadOnly, parent)
		}
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	f, err := os.OpenFile(resolved, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		if os.IsPermission(err) {
			return nil, fmt.Errorf("%w: %s", ErrReadOnly, relPath)
		}
		return nil, err
	}
	return f, nil
}
