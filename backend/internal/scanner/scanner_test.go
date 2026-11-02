package scanner_test

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

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

func createTestVideoFile(targetPath string) bool {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return false
	}

	cmd := exec.Command(ffmpegPath,
		"-y",
		"-f", "lavfi",
		"-i", "testsrc=duration=1:size=320x240:rate=10",
		"-pix_fmt", "yuv420p",
		targetPath,
	)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
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

	s := scanner.New(store, repo)
	ctx := t.Context()

	// Initial scan: should index 2 photos and skip non-photos / internal dirs
	status, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Initial scan failed: %v", err)
	}
	if status.Status != "completed" {
		t.Errorf("Expected status 'completed', got %s", status.Status)
	}
	if status.Indexed != 2 {
		t.Errorf("Expected 2 photos indexed, got %d", status.Indexed)
	}
	if status.Skipped != 0 {
		t.Errorf("Expected 0 skipped on first scan, got %d", status.Skipped)
	}
	if status.Failed != 0 {
		t.Errorf("Expected 0 failures, got %d (errors: %v)", status.Failed, status.Errors)
	}

	// Verify database records
	total, err := repo.Count(ctx)
	if err != nil || total != 2 {
		t.Errorf("Expected 2 photos in database, got count %d, err %v", total, err)
	}

	p1, err := repo.GetByRelativePath(ctx, "vacation/beach.jpg")
	if err != nil || p1 == nil {
		t.Fatalf("Failed to retrieve vacation/beach.jpg: %v", err)
	}
	if p1.Width == nil || *p1.Width != 640 || p1.Height == nil || *p1.Height != 480 {
		t.Errorf("Unexpected dimensions for beach.jpg: width=%v height=%v", p1.Width, p1.Height)
	}
	if p1.MediaType != "image/jpeg" {
		t.Errorf("Expected image/jpeg, got %s", p1.MediaType)
	}

	p2, err := repo.GetByRelativePath(ctx, "vacation/2026/mountains.png")
	if err != nil || p2 == nil {
		t.Fatalf("Failed to retrieve vacation/2026/mountains.png: %v", err)
	}
	if p2.MediaType != "image/png" {
		t.Errorf("Expected image/png, got %s", p2.MediaType)
	}

	// Repeated scan: all unchanged files must be skipped and no duplicates created
	statusRepeat, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Repeated scan failed: %v", err)
	}
	if statusRepeat.Indexed != 0 {
		t.Errorf("Expected 0 indexed on repeated scan, got %d", statusRepeat.Indexed)
	}
	if statusRepeat.Skipped != 2 {
		t.Errorf("Expected 2 skipped on repeated scan, got %d", statusRepeat.Skipped)
	}
	if statusRepeat.Updated != 0 {
		t.Errorf("Expected 0 updated on repeated scan, got %d", statusRepeat.Updated)
	}

	totalAfterRepeat, _ := repo.Count(ctx)
	if totalAfterRepeat != 2 {
		t.Errorf("Expected total count to remain 2 after repeated scan, got %d", totalAfterRepeat)
	}
}

func TestScannerVideoDiscoveryAndRepeatedScan(t *testing.T) {
	store, repo, mediaDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	clipsDir := filepath.Join(mediaDir, "Clips")
	_ = os.MkdirAll(clipsDir, 0755)

	// Create 1 photo and 1 video
	_ = os.WriteFile(filepath.Join(mediaDir, "photo.jpg"), createTestJPEG(640, 480), 0644)
	videoPath := filepath.Join(clipsDir, "video.mp4")
	if !createTestVideoFile(videoPath) {
		t.Skip("ffmpeg not available")
	}

	s := scanner.New(store, repo)
	ctx := t.Context()

	// 1. Initial scan
	status, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if status.Indexed != 2 {
		t.Fatalf("Expected 2 items indexed (1 photo, 1 video), got %d", status.Indexed)
	}

	// 2. Verify video record in database
	vRecord, err := repo.GetByRelativePath(ctx, "Clips/video.mp4")
	if err != nil || vRecord == nil {
		t.Fatalf("Failed to retrieve Clips/video.mp4: %v", err)
	}
	if vRecord.MediaType != "video/mp4" {
		t.Errorf("Expected media_type video/mp4, got %s", vRecord.MediaType)
	}
	if vRecord.Width == nil || *vRecord.Width != 320 {
		t.Errorf("Expected width 320, got %v", vRecord.Width)
	}
	if vRecord.Height == nil || *vRecord.Height != 240 {
		t.Errorf("Expected height 240, got %v", vRecord.Height)
	}
	if vRecord.Duration == nil || *vRecord.Duration <= 0 {
		t.Errorf("Expected duration > 0, got %v", vRecord.Duration)
	}
	if vRecord.VideoCodec == nil || *vRecord.VideoCodec != "h264" {
		t.Errorf("Expected video_codec h264, got %v", vRecord.VideoCodec)
	}

	// 3. Repeated scan: should skip both
	repeatStatus, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Repeated scan failed: %v", err)
	}
	if repeatStatus.Indexed != 0 || repeatStatus.Skipped != 2 {
		t.Errorf("Expected 0 indexed, 2 skipped on repeated scan, got indexed=%d, skipped=%d", repeatStatus.Indexed, repeatStatus.Skipped)
	}

	// 4. Test MediaType filtering in repository
	videos, vCount, err := repo.SearchAndFilter(ctx, repository.PhotoFilter{MediaType: "video"})
	if err != nil || vCount != 1 || len(videos) != 1 {
		t.Errorf("Expected 1 video filtered by mediaType=video, got count=%d, len=%d, err=%v", vCount, len(videos), err)
	}

	photos, pCount, err := repo.SearchAndFilter(ctx, repository.PhotoFilter{MediaType: "photo"})
	if err != nil || pCount != 1 || len(photos) != 1 {
		t.Errorf("Expected 1 photo filtered by mediaType=photo, got count=%d, len=%d, err=%v", pCount, len(photos), err)
	}

	allMedia, allCount, err := repo.SearchAndFilter(ctx, repository.PhotoFilter{MediaType: "all"})
	if err != nil || allCount != 2 || len(allMedia) != 2 {
		t.Errorf("Expected 2 items filtered by mediaType=all, got count=%d, len=%d, err=%v", allCount, len(allMedia), err)
	}
}

func TestScannerSymlinkEscapePrevention(t *testing.T) {
	store, repo, mediaDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	outsideDir, err := os.MkdirTemp("", "pixora_outside_*")
	if err != nil {
		t.Fatalf("Failed to create outside temp dir: %v", err)
	}
	defer os.RemoveAll(outsideDir)

	outsidePhoto := filepath.Join(outsideDir, "secret.jpg")
	_ = os.WriteFile(outsidePhoto, createTestJPEG(400, 300), 0644)

	symlinkPath := filepath.Join(mediaDir, "escape_link.jpg")
	if err := os.Symlink(outsidePhoto, symlinkPath); err != nil {
		t.Skip("Symlinks not supported in environment")
	}

	validPhoto := filepath.Join(mediaDir, "valid.jpg")
	_ = os.WriteFile(validPhoto, createTestJPEG(400, 300), 0644)

	s := scanner.New(store, repo)
	ctx := t.Context()

	status, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if status.Indexed != 1 {
		t.Errorf("Expected only 1 valid photo indexed, got %d", status.Indexed)
	}

	_, err = repo.GetByRelativePath(ctx, "escape_link.jpg")
	if err == nil {
		t.Error("Security violation: symlink escaping storage root was indexed")
	}
}

func TestScannerConcurrentScanProtection(t *testing.T) {
	store, repo, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	s := scanner.New(store, repo)
	ctx := t.Context()

	var wg sync.WaitGroup
	errs := make(chan error, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Scan(ctx)
			errs <- err
		}()
	}

	wg.Wait()
	close(errs)

	inProgressErrors := 0
	successfulScans := 0
	for err := range errs {
		if errors.Is(err, scanner.ErrScanInProgress) {
			inProgressErrors++
		} else if err == nil {
			successfulScans++
		}
	}

	if successfulScans < 1 {
		t.Errorf("Expected at least 1 successful scan, got %d", successfulScans)
	}
	if inProgressErrors < 1 {
		t.Errorf("Expected at least 1 ErrScanInProgress from concurrent attempts, got %d", inProgressErrors)
	}
}

func TestScannerMissingStorageRoot(t *testing.T) {
	_, repo, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	nonExistentDir := filepath.Join(os.TempDir(), "pixora_missing_dir_12345")
	store, err := storage.New(nonExistentDir)
	if err != nil {
		t.Fatalf("Storage creation failed: %v", err)
	}

	s := scanner.New(store, repo)
	ctx := t.Context()

	status, err := s.Scan(ctx)
	if err == nil {
		t.Error("Expected error when scanning missing storage root, got nil")
	}
	if status.Status != "failed" {
		t.Errorf("Expected status 'failed', got %s", status.Status)
	}
}

func TestScannerScanSpecificFolder(t *testing.T) {
	store, repo, mediaDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	familyDir := filepath.Join(mediaDir, "Family")
	wallDir := filepath.Join(mediaDir, "Wallpapers")
	_ = os.MkdirAll(familyDir, 0755)
	_ = os.MkdirAll(wallDir, 0755)

	_ = os.WriteFile(filepath.Join(familyDir, "reunion.jpg"), createTestJPEG(600, 400), 0644)
	_ = os.WriteFile(filepath.Join(wallDir, "desktop.jpg"), createTestJPEG(1920, 1080), 0644)

	s := scanner.New(store, repo)
	ctx := t.Context()

	// 1. Scan only "Family" folder
	statusFam, err := s.ScanPath(ctx, "Family")
	if err != nil {
		t.Fatalf("ScanPath('Family') failed: %v", err)
	}
	if statusFam.Indexed != 1 {
		t.Errorf("Expected indexed=1 for family, got %d", statusFam.Indexed)
	}

	// 2. Verify only Family photo is in repo
	totalAfterFam, _ := repo.Count(ctx)
	if totalAfterFam != 1 {
		t.Errorf("Expected 1 photo in repo, got %d", totalAfterFam)
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
