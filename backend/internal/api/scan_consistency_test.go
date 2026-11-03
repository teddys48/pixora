package api_test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v2"

	"pixora/backend/internal/api"
	"pixora/backend/internal/database"
	"pixora/backend/internal/repository"
	"pixora/backend/internal/scanner"
	"pixora/backend/internal/storage"
)

func setupComprehensiveTestApp(t *testing.T) (*fiber.App, *storage.Storage, repository.PhotoRepository, *scanner.Scanner, string, *database.DB, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "pixora_comprehensive_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	mediaDir := filepath.Join(tempDir, "media")
	dbDir := filepath.Join(tempDir, "data")
	_ = os.MkdirAll(mediaDir, 0755)
	_ = os.MkdirAll(dbDir, 0755)

	db, err := database.Open(filepath.Join(dbDir, "test.db"))
	if err != nil {
		_ = os.RemoveAll(tempDir)
		t.Fatalf("Failed to open test db: %v", err)
	}

	store, err := storage.New(mediaDir)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		t.Fatalf("Failed to create storage: %v", err)
	}

	photoRepo := repository.NewSQLitePhotoRepository(db)
	scanSvc := scanner.New(store, photoRepo)

	app := fiber.New()
	apiGroup := app.Group("/api")

	apiGroup.Get("/folders/contents", api.GetFolderContentsHandler(store, photoRepo))
	apiGroup.Get("/folders/photos", api.GetFolderPhotosHandler(store, photoRepo))
	apiGroup.Get("/folders", api.ListFoldersHandler(store, photoRepo))
	apiGroup.Get("/photos/scan", api.GetScanStatusHandler(scanSvc))
	apiGroup.Get("/photos/scan/status", api.GetScanStatusHandler(scanSvc))
	apiGroup.Post("/photos/scan", api.ScanPhotosHandler(scanSvc))
	apiGroup.Get("/photos", api.ListPhotosHandler(photoRepo))

	cleanup := func() {
		_ = db.Close()
		_ = os.RemoveAll(tempDir)
	}

	return app, store, photoRepo, scanSvc, mediaDir, db, cleanup
}

func Test134MediaFilesFolderAndPhotosScanSequence(t *testing.T) {
	app, _, photoRepo, _, mediaDir, _, cleanup := setupComprehensiveTestApp(t)
	defer cleanup()

	// 1. Create media files in nested directories.
	// Folder A: "2024_旅行_Été" (contains spaces and Unicode characters) with exactly 134 media files.
	folderUnicode := filepath.Join(mediaDir, "2024_旅行_Été")
	_ = os.MkdirAll(folderUnicode, 0755)

	// Folder B: "Nested/Deep/SubFolder" with 10 media files
	nestedFolder := filepath.Join(mediaDir, "Nested", "Deep", "SubFolder")
	_ = os.MkdirAll(nestedFolder, 0755)

	// Empty directory
	emptyDir := filepath.Join(mediaDir, "EmptyDir")
	_ = os.MkdirAll(emptyDir, 0755)

	// Direct root media files
	_ = os.WriteFile(filepath.Join(mediaDir, "root_photo_1.jpg"), createTestJPEG(320, 240), 0644)
	_ = os.WriteFile(filepath.Join(mediaDir, "root_photo_2.png"), createTestPNG(320, 240), 0644)

	// Unsupported files in folder A (should be ignored by scanner and discovery)
	_ = os.WriteFile(filepath.Join(folderUnicode, "notes.txt"), []byte("trip notes"), 0644)
	_ = os.WriteFile(filepath.Join(folderUnicode, "metadata.json"), []byte("{}"), 0644)

	// Create 134 supported media files in folder A (mixture of JPEG and PNG, spaces, Unicode)
	totalTarget := 134
	jpegData := createTestJPEG(200, 150)
	pngData := createTestPNG(200, 150)

	for i := 1; i <= totalTarget; i++ {
		var filename string
		var data []byte
		if i%3 == 0 {
			filename = fmt.Sprintf("Photo %03d 富士山.png", i)
			data = pngData
		} else if i%3 == 1 {
			filename = fmt.Sprintf("Image %03d Événement.jpg", i)
			data = jpegData
		} else {
			filename = fmt.Sprintf("vacation_snap_%03d.jpg", i)
			data = jpegData
		}
		if err := os.WriteFile(filepath.Join(folderUnicode, filename), data, 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	// Create 10 files in nested folder
	for i := 1; i <= 10; i++ {
		fn := fmt.Sprintf("nested_%02d.jpg", i)
		_ = os.WriteFile(filepath.Join(nestedFolder, fn), jpegData, 0644)
	}

	// 2. Trigger discovery from a physical folder: GET /api/folders/contents?path=2024_旅行_Été
	folderPathQuery := url.QueryEscape("2024_旅行_Été")
	reqDiscover := httptest.NewRequest("GET", "/api/folders/contents?path="+folderPathQuery, nil)
	respDiscover, err := app.Test(reqDiscover)
	if err != nil || respDiscover.StatusCode != 200 {
		t.Fatalf("Discovery failed: %v, status %d", err, respDiscover.StatusCode)
	}

	var disc api.FolderContentsResponse
	_ = json.NewDecoder(respDiscover.Body).Decode(&disc)

	// 3. Verify the unindexed count is 134
	if !disc.HasUnindexedFiles {
		t.Errorf("Expected HasUnindexedFiles=true, got false")
	}
	if disc.DiskMediaCount != 134 {
		t.Errorf("Expected DiskMediaCount=134, got %d", disc.DiskMediaCount)
	}
	if disc.UnindexedCount != 134 {
		t.Errorf("Expected UnindexedCount=134, got %d", disc.UnindexedCount)
	}
	if disc.TotalPhotos != 0 {
		t.Errorf("Expected TotalPhotos=0 before scan, got %d", disc.TotalPhotos)
	}

	// 4. Trigger scanning from that physical folder: POST /api/photos/scan?path=2024_旅行_Été&sync=true
	reqFolderScan := httptest.NewRequest("POST", "/api/photos/scan?path="+folderPathQuery+"&sync=true", nil)
	respFolderScan, err := app.Test(reqFolderScan, -1)
	if err != nil || respFolderScan.StatusCode != 200 {
		t.Fatalf("Folder scan failed: %v, status %d", err, respFolderScan.StatusCode)
	}

	var scanStatus scanner.ScanStatus
	_ = json.NewDecoder(respFolderScan.Body).Decode(&scanStatus)

	// 5. Verify database insertion
	if scanStatus.Indexed != 134 {
		t.Errorf("Expected folder scan to index 134 files, got indexed=%d (status=%+v)", scanStatus.Indexed, scanStatus)
	}
	if scanStatus.Failed != 0 {
		t.Errorf("Expected 0 failures during folder scan, got %d (errors: %v)", scanStatus.Failed, scanStatus.Errors)
	}

	totalInRepo, _ := photoRepo.Count(t.Context())
	if totalInRepo != 134 {
		t.Errorf("Expected 134 photos in DB after folder scan, got %d", totalInRepo)
	}

	// 6. Verify physical folder contents: GET /api/folders/contents?path=2024_旅行_Été
	reqPostScan := httptest.NewRequest("GET", "/api/folders/contents?path="+folderPathQuery, nil)
	respPostScan, err := app.Test(reqPostScan)
	if err != nil || respPostScan.StatusCode != 200 {
		t.Fatalf("Folder contents post-scan failed: %v, status %d", err, respPostScan.StatusCode)
	}

	var postScan api.FolderContentsResponse
	_ = json.NewDecoder(respPostScan.Body).Decode(&postScan)

	if postScan.HasUnindexedFiles {
		t.Errorf("Expected HasUnindexedFiles=false after scan, got true (unindexedCount=%d)", postScan.UnindexedCount)
	}
	if postScan.UnindexedCount != 0 {
		t.Errorf("Expected UnindexedCount=0 after scan, got %d", postScan.UnindexedCount)
	}
	if postScan.TotalPhotos != 134 {
		t.Errorf("Expected TotalPhotos=134 after scan, got %d", postScan.TotalPhotos)
	}
	if len(postScan.Photos) != 50 { // First page of 50 items
		t.Errorf("Expected 50 photos on first page, got %d", len(postScan.Photos))
	}
	if !postScan.HasMore || postScan.NextCursor == nil {
		t.Errorf("Expected HasMore=true and NextCursor present for 134 items with limit 50")
	}

	// Verify pagination cursor retrieves next pages
	reqPage2 := httptest.NewRequest("GET", fmt.Sprintf("/api/folders/contents?path=%s&cursor=%s", folderPathQuery, url.QueryEscape(*postScan.NextCursor)), nil)
	respPage2, _ := app.Test(reqPage2)
	var page2 api.FolderContentsResponse
	_ = json.NewDecoder(respPage2.Body).Decode(&page2)
	if len(page2.Photos) != 50 {
		t.Errorf("Expected 50 photos on page 2, got %d", len(page2.Photos))
	}
	if !page2.HasMore || page2.NextCursor == nil {
		t.Errorf("Expected HasMore=true for page 2")
	}

	reqPage3 := httptest.NewRequest("GET", fmt.Sprintf("/api/folders/contents?path=%s&cursor=%s", folderPathQuery, url.QueryEscape(*page2.NextCursor)), nil)
	respPage3, _ := app.Test(reqPage3)
	var page3 api.FolderContentsResponse
	_ = json.NewDecoder(respPage3.Body).Decode(&page3)
	if len(page3.Photos) != 34 { // 134 - 100 = 34
		t.Errorf("Expected 34 photos on page 3, got %d", len(page3.Photos))
	}
	if page3.HasMore {
		t.Errorf("Expected HasMore=false on last page")
	}

	// 7. Trigger scanning from the Photos page: POST /api/photos/scan?sync=true (scans entire library)
	reqPhotosScan := httptest.NewRequest("POST", "/api/photos/scan?sync=true", nil)
	respPhotosScan, err := app.Test(reqPhotosScan, -1)
	if err != nil || respPhotosScan.StatusCode != 200 {
		t.Fatalf("Photos scan failed: %v, status %d", err, respPhotosScan.StatusCode)
	}

	var photosScanStatus scanner.ScanStatus
	_ = json.NewDecoder(respPhotosScan.Body).Decode(&photosScanStatus)

	// 8. Verify the new-media count: 134 unchanged/skipped from folder A, 2 from root + 10 from Nested = 12 newly indexed
	if photosScanStatus.Indexed != 12 {
		t.Errorf("Expected Photos scan to index remaining 12 photos, got %d", photosScanStatus.Indexed)
	}
	if photosScanStatus.Skipped < 134 {
		t.Errorf("Expected at least 134 skipped photos (folder A already indexed), got %d", photosScanStatus.Skipped)
	}

	totalAfterAll, _ := photoRepo.Count(t.Context())
	if totalAfterAll != 146 { // 134 + 10 + 2
		t.Errorf("Expected 146 total photos in database, got %d", totalAfterAll)
	}

	// 9. Repeat both scan operations:
	// 9a. Repeat folder scan
	reqRepeatFolder := httptest.NewRequest("POST", "/api/photos/scan?path="+folderPathQuery+"&sync=true", nil)
	respRepeatFolder, _ := app.Test(reqRepeatFolder, -1)
	var repeatFolderStatus scanner.ScanStatus
	_ = json.NewDecoder(respRepeatFolder.Body).Decode(&repeatFolderStatus)

	// 9b. Repeat Photos page scan
	reqRepeatPhotos := httptest.NewRequest("POST", "/api/photos/scan?sync=true", nil)
	respRepeatPhotos, _ := app.Test(reqRepeatPhotos, -1)
	var repeatPhotosStatus scanner.ScanStatus
	_ = json.NewDecoder(respRepeatPhotos.Body).Decode(&repeatPhotosStatus)

	// 10. Verify no duplicate records are created and no false unindexed notifications remain.
	if repeatFolderStatus.Indexed != 0 || repeatFolderStatus.Skipped != 134 {
		t.Errorf("Repeat folder scan should index 0 and skip 134, got indexed=%d, skipped=%d", repeatFolderStatus.Indexed, repeatFolderStatus.Skipped)
	}
	if repeatPhotosStatus.Indexed != 0 {
		t.Errorf("Repeat Photos scan should report 0 new media, got indexed=%d", repeatPhotosStatus.Indexed)
	}

	finalTotal, _ := photoRepo.Count(t.Context())
	if finalTotal != 146 {
		t.Errorf("Total photos changed after repeated scans: got %d, want 146", finalTotal)
	}

	// Verify unindexed notifications on both folder and root
	reqCheckFolder := httptest.NewRequest("GET", "/api/folders/contents?path="+folderPathQuery, nil)
	respCheckFolder, _ := app.Test(reqCheckFolder)
	var checkFolder api.FolderContentsResponse
	_ = json.NewDecoder(respCheckFolder.Body).Decode(&checkFolder)
	if checkFolder.HasUnindexedFiles || checkFolder.UnindexedCount != 0 {
		t.Errorf("False unindexed notification remained on folder: %+v", checkFolder)
	}

	// Check root folder contents (should only contain 2 direct root photos)
	reqRoot := httptest.NewRequest("GET", "/api/folders/contents?path=", nil)
	respRoot, _ := app.Test(reqRoot)
	var rootContents api.FolderContentsResponse
	_ = json.NewDecoder(respRoot.Body).Decode(&rootContents)
	if rootContents.TotalPhotos != 2 {
		t.Errorf("Root folder should have 2 direct photos, got %d", rootContents.TotalPhotos)
	}
	if rootContents.HasUnindexedFiles || rootContents.UnindexedCount != 0 {
		t.Errorf("Root folder has false unindexed notification: %+v", rootContents)
	}
}

func TestEdgeCasesAndFailureHandling(t *testing.T) {
	app, _, photoRepo, _, mediaDir, _, cleanup := setupComprehensiveTestApp(t)
	defer cleanup()

	// 1. Path Normalization: root with "/", ".", and trailing slashes
	subDir := filepath.Join(mediaDir, "SubDir")
	_ = os.MkdirAll(subDir, 0755)
	_ = os.WriteFile(filepath.Join(subDir, "pic.jpg"), createTestJPEG(100, 100), 0644)

	// Valid root representations should succeed
	for _, p := range []string{"", "/", "."} {
		req := httptest.NewRequest("GET", "/api/folders/contents?path="+url.QueryEscape(p), nil)
		resp, err := app.Test(req)
		if err != nil || resp.StatusCode != 200 {
			t.Errorf("Path %q should resolve root successfully, got status %d", p, resp.StatusCode)
		}
	}

	// Absolute path traversal attempt must be rejected with 403
	reqEsc := httptest.NewRequest("GET", "/api/folders/contents?path=/etc", nil)
	respEsc, _ := app.Test(reqEsc)
	if respEsc.StatusCode != 403 {
		t.Errorf("Expected 403 for /etc, got %d", respEsc.StatusCode)
	}

	// 2. Corrupt / Invalid image failure handling in Scanner
	corruptDir := filepath.Join(mediaDir, "CorruptFolder")
	_ = os.MkdirAll(corruptDir, 0755)
	_ = os.WriteFile(filepath.Join(corruptDir, "broken.jpg"), []byte("this is not a valid jpeg"), 0644)
	_ = os.WriteFile(filepath.Join(corruptDir, "valid.jpg"), createTestJPEG(100, 100), 0644)

	reqScanCorrupt := httptest.NewRequest("POST", "/api/photos/scan?path=CorruptFolder&sync=true", nil)
	respScanCorrupt, _ := app.Test(reqScanCorrupt, -1)
	var corruptStatus scanner.ScanStatus
	_ = json.NewDecoder(respScanCorrupt.Body).Decode(&corruptStatus)

	if corruptStatus.Failed != 1 {
		t.Errorf("Expected 1 failed file for corrupt image, got %d", corruptStatus.Failed)
	}
	if corruptStatus.Indexed != 1 {
		t.Errorf("Expected 1 indexed file for valid image, got %d", corruptStatus.Indexed)
	}
	if len(corruptStatus.Errors) == 0 {
		t.Errorf("Expected error list to contain failure details")
	}

	// Total failures only: if all files fail, status should be 'failed'
	allBadDir := filepath.Join(mediaDir, "AllBad")
	_ = os.MkdirAll(allBadDir, 0755)
	_ = os.WriteFile(filepath.Join(allBadDir, "bad1.jpg"), []byte("not jpeg 1"), 0644)
	_ = os.WriteFile(filepath.Join(allBadDir, "bad2.jpg"), []byte("not jpeg 2"), 0644)

	reqScanAllBad := httptest.NewRequest("POST", "/api/photos/scan?path=AllBad&sync=true", nil)
	respScanAllBad, _ := app.Test(reqScanAllBad, -1)
	var allBadStatus scanner.ScanStatus
	_ = json.NewDecoder(respScanAllBad.Body).Decode(&allBadStatus)

	if allBadStatus.Status != "failed" {
		t.Errorf("Expected scan status='failed' when all files fail, got %q", allBadStatus.Status)
	}
	if allBadStatus.Failed != 2 {
		t.Errorf("Expected failed=2, got %d", allBadStatus.Failed)
	}

	_ = photoRepo
}
