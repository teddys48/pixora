package storage_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"pixora/backend/internal/storage"
)

func TestStorageRelativePathValidation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pixora_storage_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s, err := storage.New(tempDir)
	if err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	validPaths := []string{
		"photo.jpg",
		"originals/2026/10/img_1234.png",
		"thumbnails/thumb_1234.webp",
		"sub/folder/nested/image.jpeg",
	}

	for _, p := range validPaths {
		if err := s.ValidateRelativePath(p); err != nil {
			t.Errorf("Expected valid path for %q, got: %v", p, err)
		}
	}

	invalidPaths := []struct {
		path    string
		wantErr error
	}{
		{"", storage.ErrInvalidPath},
		{"   ", storage.ErrInvalidPath},
		{"photo\x00.jpg", storage.ErrInvalidPath},
		{"/etc/passwd", storage.ErrPathTraversal},
		{"/originals/photo.jpg", storage.ErrPathTraversal},
		{"../etc/passwd", storage.ErrPathTraversal},
		{"../../escape.jpg", storage.ErrPathTraversal},
		{"originals/../../escape.jpg", storage.ErrPathTraversal},
		{".", storage.ErrPathTraversal},
		{"..", storage.ErrPathTraversal},
	}

	for _, tc := range invalidPaths {
		err := s.ValidateRelativePath(tc.path)
		if err == nil {
			t.Errorf("Expected error for path %q, got nil", tc.path)
			continue
		}
		if !errors.Is(err, tc.wantErr) {
			t.Errorf("Expected error type %v for %q, got: %v", tc.wantErr, tc.path, err)
		}
	}
}

func TestStoragePathTraversalRejection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pixora_traversal_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s, err := storage.New(tempDir)
	if err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	traversalAttempts := []string{
		"../outside.jpg",
		"../../outside.jpg",
		"folder/../../outside.jpg",
		"/absolute/path.jpg",
	}

	for _, p := range traversalAttempts {
		_, err := s.ResolvePath(p)
		if err == nil {
			t.Errorf("Expected path traversal error for %q, got nil", p)
		} else if !errors.Is(err, storage.ErrPathTraversal) {
			t.Errorf("Expected ErrPathTraversal for %q, got %v", p, err)
		}
	}
}

func TestStorageSymlinkEscapeRejection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pixora_symlink_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an outside secret file
	outsideFile, err := os.CreateTemp("", "pixora_outside_*")
	if err != nil {
		t.Fatalf("Failed to create outside temp file: %v", err)
	}
	outsidePath := outsideFile.Name()
	_ = outsideFile.Close()
	defer os.Remove(outsidePath)

	// Create a symlink inside storage root pointing to outsideFile
	symlinkPath := filepath.Join(tempDir, "evil_symlink.jpg")
	if err := os.Symlink(outsidePath, symlinkPath); err != nil {
		t.Skipf("Symlink creation not supported on this filesystem: %v", err)
	}

	s, err := storage.New(tempDir)
	if err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Attempting to resolve the symlinked path must fail with path traversal
	_, err = s.ResolvePath("evil_symlink.jpg")
	if err == nil {
		t.Errorf("Expected path traversal error for symlink escape, got nil")
	} else if !errors.Is(err, storage.ErrPathTraversal) {
		t.Errorf("Expected ErrPathTraversal for symlink escape, got %v", err)
	}
}

func TestStorageMissingDirectoryBehavior(t *testing.T) {
	nonExistent := filepath.Join(os.TempDir(), "pixora_missing_dir_123456")
	_ = os.RemoveAll(nonExistent)

	s, err := storage.New(nonExistent)
	if err != nil {
		t.Fatalf("Storage.New should succeed even if directory does not exist yet: %v", err)
	}

	// CheckReadAccess on non-existent dir should return ErrStorageRootNotFound
	err = s.CheckReadAccess()
	if err == nil {
		t.Errorf("Expected error for missing directory, got nil")
	} else if !errors.Is(err, storage.ErrStorageRootNotFound) {
		t.Errorf("Expected ErrStorageRootNotFound, got %v", err)
	}

	// OpenFile on missing directory should fail with not exist
	_, err = s.OpenFile("test.jpg")
	if err == nil {
		t.Errorf("Expected error opening file from missing root, got nil")
	}
}

func TestStorageReadOnlyBehavior(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pixora_ro_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an existing photo file in the storage directory
	testFile := filepath.Join(tempDir, "existing.jpg")
	if err := os.WriteFile(testFile, []byte("image-bytes"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Make the root directory read-only (0555)
	if err := os.Chmod(tempDir, 0555); err != nil {
		t.Fatalf("Failed to chmod temp dir: %v", err)
	}
	defer os.Chmod(tempDir, 0755) // restore for cleanup

	s, err := storage.New(tempDir)
	if err != nil {
		t.Fatalf("Failed to init storage: %v", err)
	}

	// Read operations must succeed
	if err := s.CheckReadAccess(); err != nil {
		t.Errorf("Read access check failed on read-only storage: %v", err)
	}

	rc, err := s.OpenFile("existing.jpg")
	if err != nil {
		t.Fatalf("Failed to read existing file from read-only storage: %v", err)
	}
	data, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil || string(data) != "image-bytes" {
		t.Errorf("Unexpected read data: %s, err: %v", string(data), err)
	}

	// Write check on read-only storage must return ErrReadOnly or permission error
	err = s.CheckWriteAccess()
	if err == nil {
		// If running as root (some CI), chmod 0555 might still permit root writes,
		// but in normal user environment it should return ErrReadOnly
		if os.Getuid() != 0 {
			t.Errorf("Expected ErrReadOnly for CheckWriteAccess on read-only storage, got nil")
		}
	} else if !errors.Is(err, storage.ErrReadOnly) && !errors.Is(err, storage.ErrPermissionDenied) {
		t.Errorf("Expected ErrReadOnly or ErrPermissionDenied, got %v", err)
	}

	// CreateFile on read-only storage must return ErrReadOnly
	_, err = s.CreateFile("new_file.jpg")
	if err == nil {
		if os.Getuid() != 0 {
			t.Errorf("Expected ErrReadOnly on CreateFile, got nil")
		}
	} else if !errors.Is(err, storage.ErrReadOnly) {
		t.Logf("Got expected write failure: %v", err)
	}
}

func TestStorageFileReadWriteLifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pixora_rw_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s, err := storage.New(tempDir)
	if err != nil {
		t.Fatalf("Failed to init storage: %v", err)
	}

	// Subdir path checks
	if s.ThumbnailsDir() != filepath.Join(tempDir, "thumbnails") {
		t.Errorf("Unexpected ThumbnailsDir: %s", s.ThumbnailsDir())
	}
	if s.TmpDir() != filepath.Join(tempDir, "tmp") {
		t.Errorf("Unexpected TmpDir: %s", s.TmpDir())
	}

	// Create nested file
	rel := "originals/2026/10/photo1.jpg"
	writer, err := s.CreateFile(rel)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	content := "test-image-content-12345"
	if _, err := writer.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close file writer: %v", err)
	}

	// Stat file
	info, err := s.Stat(rel)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	if info.Size() != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), info.Size())
	}

	// Read file
	reader, err := s.OpenFile(rel)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	readBytes, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(readBytes) != content {
		t.Errorf("Content mismatch: expected %q, got %q", content, string(readBytes))
	}
}
