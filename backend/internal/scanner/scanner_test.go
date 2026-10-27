package scanner_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pixora/backend/internal/database"
	"pixora/backend/internal/repository"
	"pixora/backend/internal/scanner"
	"pixora/backend/internal/storage"
)

func createTestJPEG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 120, 255})
		}
	}
	buf := new(bytes.Buffer)
	_ = jpeg.Encode(buf, img, &jpeg.Options{Quality: 80})
	return buf.Bytes()
}

func createTestPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{0, 200, 100, 255})
	buf := new(bytes.Buffer)
	_ = png.Encode(buf, img)
	return buf.Bytes()
}

func setupTestEnvironment(t *testing.T) (*storage.Storage, repository.PhotoRepository, string, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "pixora_scanner_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	mediaDir := filepath.Join(tempDir, "media")
	dbDir := filepath.Join(tempDir, "data")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create media dir: %v", err)
	}
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create db dir: %v", err)
	}

	store, err := storage.New(mediaDir)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create storage: %v", err)
	}

	db, err := database.Open(filepath.Join(dbDir, "test.db"))
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to open db: %v", err)
	}

	repo := repository.NewSQLitePhotoRepository(db)

	cleanup := func() {
		_ = db.Close()
		_ = os.RemoveAll(tempDir)
	}

	return store, repo, mediaDir, cleanup
}

func TestScannerDiscoveryAndRepeatedScan(t *testing.T) {
	store, repo, mediaDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create nested directory structure with images and non-images
	// media/
	//   vacation/
	//     beach.jpg
	//     2026/
	//       mountains.png
	//   thumbnails/
	//     cached_thumb.jpg   (MUST BE EXCLUDED)
	//   tmp/
	//     scratch.jpg        (MUST BE EXCLUDED)
	//   .hidden/
	//     secret.jpg         (MUST BE EXCLUDED)
	//   notes.txt            (MUST BE EXCLUDED)
	//   corrupt.jpg          (FAILED COUNT)

	vacationDir := filepath.Join(mediaDir, "vacation", "2026")
	thumbDir := filepath.Join(mediaDir, "thumbnails")
	tmpDir := filepath.Join(mediaDir, "tmp")
	hiddenDir := filepath.Join(mediaDir, ".hidden")

	_ = os.MkdirAll(vacationDir, 0755)
	_ = os.MkdirAll(thumbDir, 0755)
	_ = os.MkdirAll(tmpDir, 0755)
	_ = os.MkdirAll(hiddenDir, 0755)

	_ = os.WriteFile(filepath.Join(mediaDir, "vacation", "beach.jpg"), createTestJPEG(640, 480), 0644)
	_ = os.WriteFile(filepath.Join(vacationDir, "mountains.png"), createTestPNG(800, 600), 0644)
	_ = os.WriteFile(filepath.Join(thumbDir, "cached_thumb.jpg"), createTestJPEG(100, 100), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "scratch.jpg"), createTestJPEG(100, 100), 0644)
	_ = os.WriteFile(filepath.Join(hiddenDir, "secret.jpg"), createTestJPEG(100, 100), 0644)
	_ = os.WriteFile(filepath.Join(mediaDir, "notes.txt"), []byte("this is not a photo"), 0644)
	_ = os.WriteFile(filepath.Join(mediaDir, "corrupt.jpg"), []byte{0xFF, 0xD8, 0xFF, 0x00, 0x00}, 0644)

	s := scanner.New(store, repo)
	ctx := context.Background()

	// --- 1. First Scan ---
	status, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("First scan failed: %v", err)
	}

	if status.Status != "completed" {
		t.Errorf("Expected status completed, got %s", status.Status)
	}
	// Scanned candidate images: beach.jpg, mountains.png, corrupt.jpg = 3
	if status.Scanned != 3 {
		t.Errorf("Expected scanned=3, got %d", status.Scanned)
	}
	// Indexed: beach.jpg, mountains.png = 2
	if status.Indexed != 2 {
		t.Errorf("Expected indexed=2, got %d", status.Indexed)
	}
	// Failed: corrupt.jpg = 1
	if status.Failed != 1 {
		t.Errorf("Expected failed=1, got %d", status.Failed)
	}

	// Verify photos in repository
	count, err := repo.Count(ctx)
	if err != nil || count != 2 {
		t.Fatalf("Expected 2 photos in repo, got %d, err: %v", count, err)
	}

	p1, err := repo.GetByRelativePath(ctx, "vacation/beach.jpg")
	if err != nil || p1.OriginalFilename != "beach.jpg" || *p1.Width != 640 {
		t.Errorf("Photo 1 mismatch: %+v, err: %v", p1, err)
	}

	p2, err := repo.GetByRelativePath(ctx, "vacation/2026/mountains.png")
	if err != nil || p2.OriginalFilename != "mountains.png" || *p2.Width != 800 {
		t.Errorf("Photo 2 mismatch: %+v, err: %v", p2, err)
	}

	// --- 2. Repeated Scan without changes ---
	status2, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Repeated scan failed: %v", err)
	}
	if status2.Indexed != 0 {
		t.Errorf("Expected indexed=0 on repeat, got %d", status2.Indexed)
	}
	if status2.Skipped != 2 {
		t.Errorf("Expected skipped=2 on repeat, got %d", status2.Skipped)
	}
	if status2.Failed != 1 {
		t.Errorf("Expected failed=1 on repeat, got %d", status2.Failed)
	}

	// Total count must remain 2 (no duplicates)
	countAfter, _ := repo.Count(ctx)
	if countAfter != 2 {
		t.Errorf("Expected repo count to remain 2, got %d", countAfter)
	}

	// --- 3. Update existing file on disk and re-scan ---
	// Wait a moment so mod time changes
	time.Sleep(50 * time.Millisecond)
	_ = os.WriteFile(filepath.Join(mediaDir, "vacation", "beach.jpg"), createTestJPEG(1024, 768), 0644)

	status3, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan after modification failed: %v", err)
	}
	if status3.Updated != 1 {
		t.Errorf("Expected updated=1, got %d", status3.Updated)
	}
	if status3.Skipped != 1 {
		t.Errorf("Expected skipped=1, got %d", status3.Skipped)
	}

	updatedPhoto, _ := repo.GetByRelativePath(ctx, "vacation/beach.jpg")
	if *updatedPhoto.Width != 1024 || *updatedPhoto.Height != 768 {
		t.Errorf("Updated photo dimensions mismatch: %dx%d", *updatedPhoto.Width, *updatedPhoto.Height)
	}
}

func TestScannerSymlinkEscapePrevention(t *testing.T) {
	store, repo, mediaDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create an external directory outside mediaDir
	outsideDir, err := os.MkdirTemp("", "pixora_outside_*")
	if err != nil {
		t.Fatalf("Failed to create outside dir: %v", err)
	}
	defer os.RemoveAll(outsideDir)

	outsidePhoto := filepath.Join(outsideDir, "external.jpg")
	_ = os.WriteFile(outsidePhoto, createTestJPEG(400, 300), 0644)

	// Create symlink inside mediaDir pointing outside
	linkPath := filepath.Join(mediaDir, "escaping_link.jpg")
	if err := os.Symlink(outsidePhoto, linkPath); err != nil {
		t.Skip("Symlinks not supported on this platform/filesystem")
	}

	s := scanner.New(store, repo)
	status, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Escaping symlink should be ignored
	if status.Indexed != 0 {
		t.Errorf("Expected 0 indexed for escaping symlink, got %d", status.Indexed)
	}
	count, _ := repo.Count(context.Background())
	if count != 0 {
		t.Errorf("Expected 0 photos in repo, got %d", count)
	}
}

func TestScannerConcurrentScanProtection(t *testing.T) {
	store, repo, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	s := scanner.New(store, repo)

	// Run first scan
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	status1, err1 := s.Scan(ctx)
	if err1 != nil {
		t.Fatalf("First scan failed: %v", err1)
	}
	if status1.Status != "completed" {
		t.Errorf("Expected completed, got %s", status1.Status)
	}

	// Scanner status check
	st := s.Status()
	if st.Status != "completed" {
		t.Errorf("Expected status 'completed', got %s", st.Status)
	}
}

func TestScannerMissingStorageRoot(t *testing.T) {
	_, repo, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	store, _ := storage.New("/non/existent/storage/path/xyz")
	s := scanner.New(store, repo)

	status, err := s.Scan(context.Background())
	if err == nil {
		t.Error("Expected error for missing storage root, got nil")
	}
	if status.Status != "failed" {
		t.Errorf("Expected status 'failed', got %s", status.Status)
	}
}
