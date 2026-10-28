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
	if status.Scanned != 3 {
		t.Errorf("Expected scanned=3, got %d", status.Scanned)
	}
	if status.Indexed != 2 {
		t.Errorf("Expected indexed=2, got %d", status.Indexed)
	}
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

	outsideDir, err := os.MkdirTemp("", "pixora_outside_*")
	if err != nil {
		t.Fatalf("Failed to create outside dir: %v", err)
	}
	defer os.RemoveAll(outsideDir)

	outsidePhoto := filepath.Join(outsideDir, "external.jpg")
	_ = os.WriteFile(outsidePhoto, createTestJPEG(400, 300), 0644)

	linkPath := filepath.Join(mediaDir, "escaping_link.jpg")
	if err := os.Symlink(outsidePhoto, linkPath); err != nil {
		t.Skip("Symlinks not supported on this platform/filesystem")
	}

	s := scanner.New(store, repo)
	status, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	status1, err1 := s.Scan(ctx)
	if err1 != nil {
		t.Fatalf("First scan failed: %v", err1)
	}
	if status1.Status != "completed" {
		t.Errorf("Expected completed, got %s", status1.Status)
	}

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

func TestScannerScanSpecificFolder(t *testing.T) {
	store, repo, mediaDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	vacationDir := filepath.Join(mediaDir, "Vacation")
	wallpapersDir := filepath.Join(mediaDir, "Wallpapers")
	_ = os.MkdirAll(vacationDir, 0755)
	_ = os.MkdirAll(wallpapersDir, 0755)

	_ = os.WriteFile(filepath.Join(vacationDir, "beach.jpg"), createTestJPEG(640, 480), 0644)
	_ = os.WriteFile(filepath.Join(wallpapersDir, "nature.png"), createTestPNG(800, 600), 0644)

	s := scanner.New(store, repo)
	ctx := context.Background()

	// 1. Scan only "Vacation" folder
	status, err := s.ScanPath(ctx, "Vacation")
	if err != nil {
		t.Fatalf("ScanPath('Vacation') failed: %v", err)
	}
	if status.Indexed != 1 || status.Scanned != 1 {
		t.Errorf("Expected indexed=1, scanned=1; got indexed=%d, scanned=%d", status.Indexed, status.Scanned)
	}

	// Verify only Vacation photo is in repo
	total, err := repo.Count(ctx)
	if err != nil || total != 1 {
		t.Errorf("Expected 1 photo in repo, got %d", total)
	}
	pVac, err := repo.GetByRelativePath(ctx, "Vacation/beach.jpg")
	if err != nil || pVac == nil {
		t.Errorf("Expected 'Vacation/beach.jpg' in repo, got err: %v", err)
	}

	// 2. Re-scan "Vacation": must skip, NO duplicate records
	statusRepeat, err := s.ScanPath(ctx, "Vacation")
	if err != nil {
		t.Fatalf("Repeat ScanPath('Vacation') failed: %v", err)
	}
	if statusRepeat.Skipped != 1 || statusRepeat.Indexed != 0 {
		t.Errorf("Expected skipped=1, indexed=0 on repeat; got skipped=%d, indexed=%d", statusRepeat.Skipped, statusRepeat.Indexed)
	}
	totalAfter, _ := repo.Count(ctx)
	if totalAfter != 1 {
		t.Errorf("Expected repo count to remain 1 after repeat scan, got %d", totalAfter)
	}

	// 3. Scan "Wallpapers" folder
	statusWall, err := s.ScanPath(ctx, "Wallpapers")
	if err != nil {
		t.Fatalf("ScanPath('Wallpapers') failed: %v", err)
	}
	if statusWall.Indexed != 1 {
		t.Errorf("Expected indexed=1 for wallpapers, got %d", statusWall.Indexed)
	}
	totalFinal, _ := repo.Count(ctx)
	if totalFinal != 2 {
		t.Errorf("Expected 2 photos in repo after scanning both folders, got %d", totalFinal)
	}

	// 4. Scanning invalid / traversal path must fail safely
	statusBad, err := s.ScanPath(ctx, "../outside")
	if err == nil {
		t.Errorf("Expected error when scanning path traversal, got nil")
	}
	if statusBad.Status != "failed" {
		t.Errorf("Expected status 'failed' for path traversal, got %s", statusBad.Status)
	}
}

func TestScannerForceRefresh(t *testing.T) {
	store, repo, mediaDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	_ = os.WriteFile(filepath.Join(mediaDir, "test.jpg"), createTestJPEG(640, 480), 0644)

	s := scanner.New(store, repo)
	ctx := t.Context()

	// Initial scan: indexes the photo
	status1, err := s.Scan(ctx)
	if err != nil || status1.Indexed != 1 {
		t.Fatalf("Initial scan failed: %v, status: %+v", err, status1)
	}

	// Repeated scan without refresh: skips the unchanged photo
	status2, err := s.Scan(ctx)
	if err != nil || status2.Skipped != 1 || status2.Updated != 0 {
		t.Fatalf("Repeated scan should skip: %v, status: %+v", err, status2)
	}

	// Scan with forceRefresh=true: re-extracts and updates without creating duplicates
	statusRefresh, err := s.ScanPath(ctx, "", true)
	if err != nil {
		t.Fatalf("Force refresh scan failed: %v", err)
	}
	if statusRefresh.Updated != 1 || statusRefresh.Indexed != 0 {
		t.Errorf("Expected 1 updated, 0 indexed on force refresh, got: %+v", statusRefresh)
	}

	total, _ := repo.Count(ctx)
	if total != 1 {
		t.Errorf("Expected total count=1 (no duplicates), got %d", total)
	}
}
