package scanner_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
			img.Set(x, y, color.RGBA{uint8((x * 255) / w), uint8((y * 255) / h), 100, 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80})
	return buf.Bytes()
}

func createTestVideoFile(t *testing.T, targetPath string, durationSec int, width, height int) bool {
	t.Helper()
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return false
	}

	_ = os.MkdirAll(filepath.Dir(targetPath), 0755)

	cmd := exec.Command(ffmpegPath,
		"-y",
		"-f", "lavfi",
		"-i", "testsrc=duration=1:size=320x240:rate=10",
		"-c:v", "libx264",
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
	_ = os.MkdirAll(mediaDir, 0755)
	_ = os.MkdirAll(dbDir, 0755)

	store, err := storage.New(mediaDir)
	if err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	dbPath := filepath.Join(dbDir, "pixora_test.db")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	photoRepo := repository.NewSQLitePhotoRepository(db)

	cleanup := func() {
		_ = db.Close()
		_ = os.RemoveAll(tempDir)
	}

	return store, photoRepo, mediaDir, cleanup
}

func TestScannerDiscoveryAndRepeatedScan(t *testing.T) {
	store, repo, mediaDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// 1. Create a directory structure with images and non-image files
	vacationDir := filepath.Join(mediaDir, "Vacation", "Summer")
	_ = os.MkdirAll(vacationDir, 0755)
	_ = os.WriteFile(filepath.Join(vacationDir, "beach.jpg"), createTestJPEG(640, 480), 0644)
	_ = os.WriteFile(filepath.Join(vacationDir, "sunset.png"), createTestJPEG(800, 600), 0644)
	_ = os.WriteFile(filepath.Join(vacationDir, "notes.txt"), []byte("not a photo"), 0644)

	// Subfolder with hidden files
	hiddenDir := filepath.Join(mediaDir, ".hidden")
	_ = os.MkdirAll(hiddenDir, 0755)
	_ = os.WriteFile(filepath.Join(hiddenDir, "secret.jpg"), createTestJPEG(100, 100), 0644)

	s := scanner.New(store, repo)
	ctx := t.Context()

	// Initial scan: should discover the 2 photos, skip text file and hidden dir
	status, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if status.Status != "completed" {
		t.Errorf("Expected status 'completed', got %q", status.Status)
	}
	if status.Indexed != 2 {
		t.Errorf("Expected 2 photos indexed, got %d", status.Indexed)
	}
	if status.Scanned != 2 {
		t.Errorf("Expected 2 files scanned, got %d", status.Scanned)
	}
	if status.Skipped != 0 {
		t.Errorf("Expected 0 skipped on initial scan, got %d", status.Skipped)
	}

	// Verify records were inserted into SQLite
	total, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to count photos in repository: %v", err)
	}
	if total != 2 {
		t.Errorf("Expected 2 photos in database, got %d", total)
	}

	// Check relative_path format (forward slashes)
	beach, err := repo.GetByRelativePath(ctx, "Vacation/Summer/beach.jpg")
	if err != nil || beach == nil {
		t.Errorf("Expected to find Vacation/Summer/beach.jpg, err: %v", err)
	} else if beach.Width == nil || *beach.Width != 640 {
		t.Errorf("Expected width 640, got %v", beach.Width)
	}

	// 2. Repeated scan without changes: should skip all unchanged files
	repeatStatus, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Repeated scan failed: %v", err)
	}
	if repeatStatus.Indexed != 0 {
		t.Errorf("Expected 0 indexed on repeated scan, got %d", repeatStatus.Indexed)
	}
	if repeatStatus.Skipped != 2 {
		t.Errorf("Expected 2 skipped on repeated scan, got %d", repeatStatus.Skipped)
	}
	if repeatStatus.Updated != 0 {
		t.Errorf("Expected 0 updated on repeated scan, got %d", repeatStatus.Updated)
	}

	// Verify database count is still 2 (deduplication)
	totalAfter, _ := repo.Count(ctx)
	if totalAfter != 2 {
		t.Errorf("Expected count to remain 2, got %d", totalAfter)
	}

	// 3. Modify an existing file: should trigger update
	time.Sleep(10 * time.Millisecond) // ensure mtime differs
	_ = os.WriteFile(filepath.Join(vacationDir, "beach.jpg"), createTestJPEG(1024, 768), 0644)

	updateStatus, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Update scan failed: %v", err)
	}
	if updateStatus.Updated != 1 {
		t.Errorf("Expected 1 updated photo, got %d", updateStatus.Updated)
	}
	if updateStatus.Skipped != 1 {
		t.Errorf("Expected 1 skipped photo, got %d", updateStatus.Skipped)
	}

	// Verify updated dimensions
	updatedBeach, _ := repo.GetByRelativePath(ctx, "Vacation/Summer/beach.jpg")
	if updatedBeach != nil && (updatedBeach.Width == nil || *updatedBeach.Width != 1024) {
		t.Errorf("Expected updated width 1024, got %v", updatedBeach.Width)
	}
}

func TestScannerVideoDiscoveryAndRepeatedScan(t *testing.T) {
	store, repo, mediaDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create test image and test video
	clipsDir := filepath.Join(mediaDir, "Clips")
	_ = os.MkdirAll(clipsDir, 0755)
	_ = os.WriteFile(filepath.Join(clipsDir, "pic.jpg"), createTestJPEG(320, 240), 0644)

	videoPath := filepath.Join(clipsDir, "video.mp4")
	if !createTestVideoFile(t, videoPath, 1, 320, 240) {
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
}

func TestScannerSymlinkEscapePrevention(t *testing.T) {
	store, repo, mediaDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create an external directory outside mediaDir
	outsideDir, err := os.MkdirTemp("", "pixora_outside_*")
	if err != nil {
		t.Fatalf("Failed to create outside temp dir: %v", err)
	}
	defer os.RemoveAll(outsideDir)

	outsidePhoto := filepath.Join(outsideDir, "secret_outside.jpg")
	_ = os.WriteFile(outsidePhoto, createTestJPEG(200, 200), 0644)

	// Create a symlink inside mediaDir that points outside
	symlinkPath := filepath.Join(mediaDir, "escape_link.jpg")
	if err := os.Symlink(outsidePhoto, symlinkPath); err != nil {
		t.Skip("Symlinks not supported on this platform/filesystem")
	}

	// Create a valid normal photo
	_ = os.WriteFile(filepath.Join(mediaDir, "valid.jpg"), createTestJPEG(200, 200), 0644)

	s := scanner.New(store, repo)
	ctx := t.Context()

	status, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Only valid.jpg should be indexed; escape_link.jpg must be skipped
	if status.Indexed != 1 {
		t.Errorf("Expected exactly 1 indexed photo (symlink escaped should be skipped), got %d", status.Indexed)
	}

	escaped, _ := repo.GetByRelativePath(ctx, "escape_link.jpg")
	if escaped != nil {
		t.Error("Security violation: symlink pointing outside storage root was indexed into database!")
	}
}

func TestScannerConcurrentScanProtection(t *testing.T) {
	store, repo, mediaDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	_ = os.WriteFile(filepath.Join(mediaDir, "one.jpg"), createTestJPEG(100, 100), 0644)

	s := scanner.New(store, repo)
	ctx := t.Context()

	// Launch first scan
	done := make(chan struct{})
	var err1, err2 error

	go func() {
		_, err1 = s.Scan(ctx)
		close(done)
	}()

	// Small yield then attempt second concurrent scan
	time.Sleep(1 * time.Millisecond)
	_, err2 = s.Scan(ctx)

	<-done

	// One of the calls or the second call should succeed or report in progress
	if err1 != nil && err2 != nil {
		t.Errorf("At least one scan should proceed without error, got err1=%v, err2=%v", err1, err2)
	}
}

func TestScannerMissingStorageRoot(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pixora_missing_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	nonExistentRoot := filepath.Join(tempDir, "absent_photos")
	store, _ := storage.New(nonExistentRoot)

	dbPath := filepath.Join(tempDir, "pixora.db")
	db, _ := database.Open(dbPath)
	defer db.Close()

	repo := repository.NewSQLitePhotoRepository(db)
	s := scanner.New(store, repo)

	status, err := s.Scan(context.Background())
	if err == nil {
		t.Error("Expected error when storage root is missing, got nil")
	}
	if status.Status != "failed" {
		t.Errorf("Expected status 'failed', got %s", status.Status)
	}
}

func TestScannerScanSpecificFolder(t *testing.T) {
	store, repo, mediaDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create two folders
	dirA := filepath.Join(mediaDir, "FolderA")
	dirB := filepath.Join(mediaDir, "FolderB")
	_ = os.MkdirAll(dirA, 0755)
	_ = os.MkdirAll(dirB, 0755)

	_ = os.WriteFile(filepath.Join(dirA, "photoA.jpg"), createTestJPEG(400, 300), 0644)
	_ = os.WriteFile(filepath.Join(dirB, "photoB.jpg"), createTestJPEG(400, 300), 0644)

	s := scanner.New(store, repo)
	ctx := t.Context()

	// 1. Scan only FolderA
	statusA, err := s.ScanPath(ctx, "FolderA")
	if err != nil {
		t.Fatalf("ScanPath(FolderA) failed: %v", err)
	}
	if statusA.Indexed != 1 {
		t.Errorf("Expected 1 photo indexed for FolderA, got %d", statusA.Indexed)
	}

	// Verify only FolderA photo is indexed
	photoA, err := repo.GetByRelativePath(ctx, "FolderA/photoA.jpg")
	if err != nil || photoA == nil {
		t.Errorf("Expected FolderA/photoA.jpg to be indexed")
	}
	photoB, _ := repo.GetByRelativePath(ctx, "FolderB/photoB.jpg")
	if photoB != nil {
		t.Errorf("FolderB/photoB.jpg should not have been indexed by targeted scan of FolderA")
	}

	// 2. Scan FolderB targeted
	statusB, err := s.ScanPath(ctx, "FolderB")
	if err != nil {
		t.Fatalf("ScanPath(FolderB) failed: %v", err)
	}
	if statusB.Indexed != 1 {
		t.Errorf("Expected 1 photo indexed for FolderB, got %d", statusB.Indexed)
	}

	// 3. Both are now in repo
	photoBAfter, err := repo.GetByRelativePath(ctx, "FolderB/photoB.jpg")
	if err != nil || photoBAfter == nil {
		t.Errorf("Expected FolderB/photoB.jpg to now be indexed")
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

func TestScannerCancel(t *testing.T) {
	store, repo, mediaDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create 20 images
	for i := 0; i < 20; i++ {
		fname := filepath.Join(mediaDir, strings.Repeat("sub/", i%3), filepath.Base(filepath.Join(mediaDir, string(rune('a'+i))+".jpg")))
		_ = os.MkdirAll(filepath.Dir(fname), 0755)
		_ = os.WriteFile(fname, createTestJPEG(100, 100), 0644)
	}

	s := scanner.New(store, repo)

	// Call Cancel while not scanning: should not panic
	s.Cancel()

	// Test cancellation via context
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	status, err := s.Scan(cancelCtx)
	if status.Status != "canceled" && status.Status != "failed" {
		t.Errorf("Expected scan status canceled/failed when context is cancelled, got %s, err: %v", status.Status, err)
	}
}
