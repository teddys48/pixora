package api_test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"pixora/backend/internal/api"
	"pixora/backend/internal/database"
	"pixora/backend/internal/models"
	"pixora/backend/internal/repository"
	"pixora/backend/internal/scanner"
	"pixora/backend/internal/storage"
)

func setupFolderTestApp(t *testing.T) (*fiber.App, *storage.Storage, repository.PhotoRepository, *scanner.Scanner, string, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "pixora_folder_api_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	mediaDir := filepath.Join(tempDir, "media")
	dbDir := filepath.Join(tempDir, "data")
	_ = os.MkdirAll(mediaDir, 0755)
	_ = os.MkdirAll(dbDir, 0755)

	db, err := database.Open(filepath.Join(dbDir, "folder_api_test.db"))
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to open test db: %v", err)
	}

	store, err := storage.New(mediaDir)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create storage: %v", err)
	}

	photoRepo := repository.NewSQLitePhotoRepository(db)
	scanSvc := scanner.New(store, photoRepo)

	app := fiber.New()
	apiGroup := app.Group("/api")

	apiGroup.Get("/folders/contents", api.GetFolderContentsHandler(store, photoRepo))
	apiGroup.Get("/folders/photos", api.GetFolderPhotosHandler(store, photoRepo))
	apiGroup.Get("/folders", api.ListFoldersHandler(store, photoRepo))
	apiGroup.Post("/photos/scan", api.ScanPhotosHandler(scanSvc))

	cleanup := func() {
		_ = db.Close()
		_ = os.RemoveAll(tempDir)
	}

	return app, store, photoRepo, scanSvc, mediaDir, cleanup
}

func TestListFoldersAndContents(t *testing.T) {
	app, _, photoRepo, _, mediaDir, cleanup := setupFolderTestApp(t)
	defer cleanup()

	// Create directories:
	// media/
	//   Pictures/
	//     Anime/
	//       Movies/
	//     Wallpapers/
	//   Vacation/
	//   EmptyFolder/
	//   thumbnails/ (internal, must be ignored)
	//   tmp/        (internal, must be ignored)
	//   .hidden/    (hidden, must be ignored)
	picDir := filepath.Join(mediaDir, "Pictures")
	animeDir := filepath.Join(picDir, "Anime")
	moviesDir := filepath.Join(animeDir, "Movies")
	wallDir := filepath.Join(picDir, "Wallpapers")
	vacDir := filepath.Join(mediaDir, "Vacation")
	emptyDir := filepath.Join(mediaDir, "EmptyFolder")

	_ = os.MkdirAll(moviesDir, 0755)
	_ = os.MkdirAll(wallDir, 0755)
	_ = os.MkdirAll(vacDir, 0755)
	_ = os.MkdirAll(emptyDir, 0755)
	_ = os.MkdirAll(filepath.Join(mediaDir, "thumbnails"), 0755)
	_ = os.MkdirAll(filepath.Join(mediaDir, "tmp"), 0755)
	_ = os.MkdirAll(filepath.Join(mediaDir, ".hidden"), 0755)

	// Add files on disk
	_ = os.WriteFile(filepath.Join(mediaDir, "root.jpg"), createTestJPEG(640, 480), 0644)
	_ = os.WriteFile(filepath.Join(picDir, "pic1.jpg"), createTestJPEG(800, 600), 0644)
	_ = os.WriteFile(filepath.Join(animeDir, "anime1.jpg"), createTestJPEG(1024, 768), 0644)
	_ = os.WriteFile(filepath.Join(moviesDir, "movie1.jpg"), createTestJPEG(1280, 720), 0644)
	_ = os.WriteFile(filepath.Join(vacDir, "beach.jpg"), createTestJPEG(1920, 1080), 0644)
	_ = os.WriteFile(filepath.Join(emptyDir, "document.txt"), []byte("non-image text"), 0644)

	// Index photos in DB for some of them
	ctx := t.Context()
	now := time.Now().UTC()
	t1 := now.Add(-3 * time.Hour)
	t2 := now.Add(-2 * time.Hour)
	t3 := now.Add(-1 * time.Hour)

	_ = photoRepo.Create(ctx, &models.Photo{
		ID: "p-root", RelativePath: "root.jpg", OriginalFilename: "root.jpg", MediaType: "image/jpeg", FileSize: 100, TakenAt: &t1,
	})
	_ = photoRepo.Create(ctx, &models.Photo{
		ID: "p-pic1", RelativePath: "Pictures/pic1.jpg", OriginalFilename: "pic1.jpg", MediaType: "image/jpeg", FileSize: 200, TakenAt: &t2,
	})
	_ = photoRepo.Create(ctx, &models.Photo{
		ID: "p-movie1", RelativePath: "Pictures/Anime/Movies/movie1.jpg", OriginalFilename: "movie1.jpg", MediaType: "image/jpeg", FileSize: 300, TakenAt: &t3,
	})

	// 1. GET /api/folders (top-level folders)
	req := httptest.NewRequest("GET", "/api/folders", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("GET /api/folders failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", resp.StatusCode)
	}

	var listResp api.FolderListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Top-level folders should be Pictures, Vacation, EmptyFolder (thumbnails, tmp, .hidden excluded)
	if listResp.Total != 3 {
		t.Errorf("Expected 3 top-level folders, got %d: %+v", listResp.Total, listResp.Folders)
	}

	folderMap := make(map[string]api.PhysicalFolderItem)
	for _, f := range listResp.Folders {
		folderMap[f.Name] = f
	}

	picFolder, ok := folderMap["Pictures"]
	if !ok {
		t.Fatalf("Expected 'Pictures' folder in top-level listing")
	}
	// Pictures has pic1.jpg (1) + movie1.jpg (1) = 2 recursive photos
	if picFolder.PhotoCount != 2 {
		t.Errorf("Pictures PhotoCount = %d, want 2", picFolder.PhotoCount)
	}
	if picFolder.DirectPhotoCount != 1 {
		t.Errorf("Pictures DirectPhotoCount = %d, want 1", picFolder.DirectPhotoCount)
	}
	if picFolder.SubfolderCount != 2 { // Anime, Wallpapers
		t.Errorf("Pictures SubfolderCount = %d, want 2", picFolder.SubfolderCount)
	}

	// EmptyFolder has 0 photos
	emptyFolder, ok := folderMap["EmptyFolder"]
	if !ok || emptyFolder.PhotoCount != 0 {
		t.Errorf("Expected EmptyFolder with 0 photos, got %+v", emptyFolder)
	}

	// 2. GET /api/folders/contents?path=Pictures
	reqPic := httptest.NewRequest("GET", "/api/folders/contents?path=Pictures", nil)
	respPic, err := app.Test(reqPic)
	if err != nil || respPic.StatusCode != fiber.StatusOK {
		t.Fatalf("GET /api/folders/contents?path=Pictures failed with %v, status %d", err, respPic.StatusCode)
	}

	var contentsResp api.FolderContentsResponse
	if err := json.NewDecoder(respPic.Body).Decode(&contentsResp); err != nil {
		t.Fatalf("Failed to decode contents: %v", err)
	}

	if contentsResp.Name != "Pictures" || contentsResp.Path != "Pictures" {
		t.Errorf("Contents metadata mismatch: Name=%s, Path=%s", contentsResp.Name, contentsResp.Path)
	}
	if contentsResp.TotalPhotos != 1 {
		t.Errorf("Expected 1 direct photo in Pictures, got %d", contentsResp.TotalPhotos)
	}
	if len(contentsResp.Photos) != 1 || contentsResp.Photos[0].ID != "p-pic1" {
		t.Errorf("Photos in Pictures mismatch: %+v", contentsResp.Photos)
	}
	if len(contentsResp.Folders) != 2 { // Anime, Wallpapers
		t.Errorf("Expected 2 subfolders in Pictures, got %d", len(contentsResp.Folders))
	}
	// Breadcrumbs check
	if len(contentsResp.Breadcrumbs) != 2 {
		t.Errorf("Expected 2 breadcrumbs, got %d: %+v", len(contentsResp.Breadcrumbs), contentsResp.Breadcrumbs)
	}
	if contentsResp.Breadcrumbs[0].Path != "" || contentsResp.Breadcrumbs[1].Path != "Pictures" {
		t.Errorf("Breadcrumb paths mismatch: %+v", contentsResp.Breadcrumbs)
	}

	// 3. GET /api/folders/contents for nested directory: path=Pictures/Anime
	reqAnime := httptest.NewRequest("GET", "/api/folders/contents?path=Pictures/Anime", nil)
	respAnime, err := app.Test(reqAnime)
	if err != nil || respAnime.StatusCode != fiber.StatusOK {
		t.Fatalf("GET /api/folders/contents?path=Pictures/Anime failed: %v", err)
	}

	var animeContents api.FolderContentsResponse
	_ = json.NewDecoder(respAnime.Body).Decode(&animeContents)

	if len(animeContents.Folders) != 1 || animeContents.Folders[0].Name != "Movies" {
		t.Errorf("Expected 1 subfolder 'Movies' in Anime, got %+v", animeContents.Folders)
	}
	if len(animeContents.Breadcrumbs) != 3 {
		t.Errorf("Expected 3 breadcrumbs for Anime, got %d", len(animeContents.Breadcrumbs))
	}
	if animeContents.ParentPath == nil || *animeContents.ParentPath != "Pictures" {
		t.Errorf("Expected parentPath 'Pictures', got %v", animeContents.ParentPath)
	}
	// In Anime, anime1.jpg exists on disk but is NOT yet indexed in DB
	// diskMediaCount should be 1, totalPhotos = 0, unindexedCount = 1, hasUnindexedFiles = true
	if !animeContents.HasUnindexedFiles || animeContents.UnindexedCount != 1 {
		t.Errorf("Expected unindexed media indication for Anime: HasUnindexedFiles=%v, UnindexedCount=%d",
			animeContents.HasUnindexedFiles, animeContents.UnindexedCount)
	}

	// 4. GET /api/folders/photos?path=Pictures
	reqPhotos := httptest.NewRequest("GET", "/api/folders/photos?path=Pictures&limit=10&offset=0", nil)
	respPhotos, err := app.Test(reqPhotos)
	if err != nil || respPhotos.StatusCode != fiber.StatusOK {
		t.Fatalf("GET /api/folders/photos failed: %v", err)
	}
	var photosListResp struct {
		Path   string              `json:"path"`
		Photos []api.PhotoResponse `json:"photos"`
		Total  int                 `json:"total"`
	}
	_ = json.NewDecoder(respPhotos.Body).Decode(&photosListResp)
	if photosListResp.Total != 1 || len(photosListResp.Photos) != 1 || photosListResp.Photos[0].ID != "p-pic1" {
		t.Errorf("Folder photos listing mismatch: %+v", photosListResp)
	}
}

func TestFolderSecurityAndPathTraversal(t *testing.T) {
	app, _, _, _, mediaDir, cleanup := setupFolderTestApp(t)
	defer cleanup()

	// Create valid dir
	validDir := filepath.Join(mediaDir, "ValidFolder")
	_ = os.MkdirAll(validDir, 0755)

	// Create external dir outside mediaDir and symlink inside
	extDir, err := os.MkdirTemp("", "pixora_ext_symlink_*")
	if err != nil {
		t.Fatalf("Failed to create ext dir: %v", err)
	}
	defer os.RemoveAll(extDir)

	evilSymlink := filepath.Join(mediaDir, "evil_symlink")
	_ = os.Symlink(extDir, evilSymlink)

	// Sample file (not a folder)
	sampleFile := filepath.Join(mediaDir, "image.jpg")
	_ = os.WriteFile(sampleFile, []byte("data"), 0644)

	// 1. Path traversal attempts
	traversalPaths := []string{
		"../",
		"../../",
		"ValidFolder/../../escape",
		"/etc",
		"ValidFolder/%00/sub",
		"..",
	}

	for _, p := range traversalPaths {
		url := fmt.Sprintf("/api/folders?path=%s", p)
		req := httptest.NewRequest("GET", url, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("Request %q failed: %v", url, err)
		}
		if resp.StatusCode != fiber.StatusForbidden && resp.StatusCode != fiber.StatusBadRequest {
			t.Errorf("Expected 403 or 400 for path traversal %q, got HTTP %d", p, resp.StatusCode)
		}

		// Also check /api/folders/contents
		urlContents := fmt.Sprintf("/api/folders/contents?path=%s", p)
		reqC := httptest.NewRequest("GET", urlContents, nil)
		respC, _ := app.Test(reqC)
		if respC.StatusCode != fiber.StatusForbidden && respC.StatusCode != fiber.StatusBadRequest {
			t.Errorf("Expected 403 or 400 for contents path traversal %q, got HTTP %d", p, respC.StatusCode)
		}
	}

	// 2. Symlink escape check
	reqSymlink := httptest.NewRequest("GET", "/api/folders?path=evil_symlink", nil)
	respSymlink, _ := app.Test(reqSymlink)
	if respSymlink.StatusCode != fiber.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for symlink escaping storage root, got HTTP %d", respSymlink.StatusCode)
	}

	// 3. Nonexistent directory: must return 404
	req404 := httptest.NewRequest("GET", "/api/folders/contents?path=DoesNotExist", nil)
	resp404, _ := app.Test(req404)
	if resp404.StatusCode != fiber.StatusNotFound {
		t.Errorf("Expected 404 for non-existent folder, got HTTP %d", resp404.StatusCode)
	}

	// 4. File passed as folder: must return 400 validation error
	reqFile := httptest.NewRequest("GET", "/api/folders/contents?path=image.jpg", nil)
	respFile, _ := app.Test(reqFile)
	if respFile.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected 400 for file passed as folder, got HTTP %d", respFile.StatusCode)
	}
}

func TestScanFolderIntegration(t *testing.T) {
	app, _, photoRepo, _, mediaDir, cleanup := setupFolderTestApp(t)
	defer cleanup()

	folder1 := filepath.Join(mediaDir, "Vacation")
	folder2 := filepath.Join(mediaDir, "Wallpapers")
	_ = os.MkdirAll(folder1, 0755)
	_ = os.MkdirAll(folder2, 0755)

	_ = os.WriteFile(filepath.Join(folder1, "vac1.jpg"), createTestJPEG(640, 480), 0644)
	_ = os.WriteFile(filepath.Join(folder2, "wall1.png"), createTestPNG(800, 600), 0644)

	// 1. Initial contents check: Vacation has 1 unindexed file
	reqC1 := httptest.NewRequest("GET", "/api/folders/contents?path=Vacation", nil)
	respC1, _ := app.Test(reqC1)
	var c1 api.FolderContentsResponse
	_ = json.NewDecoder(respC1.Body).Decode(&c1)
	if !c1.HasUnindexedFiles || c1.UnindexedCount != 1 {
		t.Errorf("Expected Vacation to have 1 unindexed file before scan, got %+v", c1)
	}

	// 2. Scan only "Vacation" folder via POST /api/photos/scan?path=Vacation&sync=true
	reqScan := httptest.NewRequest("POST", "/api/photos/scan?path=Vacation&sync=true", nil)
	respScan, err := app.Test(reqScan)
	if err != nil || respScan.StatusCode != fiber.StatusOK {
		t.Fatalf("POST /api/photos/scan?path=Vacation failed: %v, status %d", err, respScan.StatusCode)
	}

	// 3. Verify Vacation contents now show 0 unindexed and 1 photo
	reqC2 := httptest.NewRequest("GET", "/api/folders/contents?path=Vacation", nil)
	respC2, _ := app.Test(reqC2)
	var c2 api.FolderContentsResponse
	_ = json.NewDecoder(respC2.Body).Decode(&c2)
	if c2.HasUnindexedFiles || c2.UnindexedCount != 0 || c2.TotalPhotos != 1 {
		t.Errorf("Expected Vacation to have 0 unindexed and 1 photo after scan, got %+v", c2)
	}

	// 4. Verify Wallpapers has NOT been scanned yet
	reqCWall := httptest.NewRequest("GET", "/api/folders/contents?path=Wallpapers", nil)
	respCWall, _ := app.Test(reqCWall)
	var cWall api.FolderContentsResponse
	_ = json.NewDecoder(respCWall.Body).Decode(&cWall)
	if !cWall.HasUnindexedFiles || cWall.UnindexedCount != 1 || cWall.TotalPhotos != 0 {
		t.Errorf("Expected Wallpapers to still have 1 unindexed and 0 photos, got %+v", cWall)
	}

	// 5. Total repo count must be 1 (only Vacation photo)
	total, _ := photoRepo.Count(t.Context())
	if total != 1 {
		t.Errorf("Expected total repo count 1, got %d", total)
	}

	// 6. Repeat scan of Vacation must NOT create duplicate records
	reqRepeat := httptest.NewRequest("POST", "/api/photos/scan?path=Vacation&sync=true", nil)
	_, _ = app.Test(reqRepeat)
	totalAfterRepeat, _ := photoRepo.Count(t.Context())
	if totalAfterRepeat != 1 {
		t.Errorf("Repeat scan created duplicate records! Total count = %d, want 1", totalAfterRepeat)
	}
}
