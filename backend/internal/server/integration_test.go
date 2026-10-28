package server_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"pixora/backend/internal/api"
	"pixora/backend/internal/config"
	"pixora/backend/internal/database"
	"pixora/backend/internal/media"
	"pixora/backend/internal/models"
	"pixora/backend/internal/repository"
	"pixora/backend/internal/scanner"
	"pixora/backend/internal/server"
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
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	buf := new(bytes.Buffer)
	_ = png.Encode(buf, img)
	return buf.Bytes()
}

func TestFrontendBackendUnifiedServing(t *testing.T) {
	// Look for frontend/dist relative to working dir or repo root
	candidates := []string{
		"../../../frontend/dist",
		"../../frontend/dist",
		"./frontend/dist",
	}

	var staticDir string
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(c)
			staticDir = abs
			break
		}
	}

	if staticDir == "" {
		t.Skip("Frontend dist not found, skipping unified integration test")
	}

	cfg := &config.Config{
		Port:      "8080",
		Host:      "0.0.0.0",
		StaticDir: staticDir,
		Env:       "production",
	}

	app := server.New(cfg)

	// 1. Health endpoint
	reqHealth := httptest.NewRequest("GET", "/api/health", nil)
	respHealth, err := app.Test(reqHealth, -1)
	if err != nil {
		t.Fatalf("Health check request failed: %v", err)
	}
	if respHealth.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200 for /api/health, got %d", respHealth.StatusCode)
	}

	var healthData api.HealthResponse
	bodyHealth, _ := io.ReadAll(respHealth.Body)
	if err := json.Unmarshal(bodyHealth, &healthData); err != nil {
		t.Fatalf("Failed to parse health JSON: %v", err)
	}
	if healthData.Status != "ok" || healthData.Service != "pixora-api" {
		t.Errorf("Unexpected health data: %+v", healthData)
	}

	// 2. Root path should serve index.html
	reqRoot := httptest.NewRequest("GET", "/", nil)
	respRoot, err := app.Test(reqRoot, -1)
	if err != nil {
		t.Fatalf("Root request failed: %v", err)
	}
	if respRoot.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200 for root, got %d", respRoot.StatusCode)
	}
	bodyRoot, _ := io.ReadAll(respRoot.Body)
	if !strings.Contains(string(bodyRoot), "Pixora") {
		t.Errorf("Expected index.html to contain 'Pixora', got %s", string(bodyRoot))
	}

	// 3. Client-side route (/photos) should serve index.html via SPA fallback
	reqPhotos := httptest.NewRequest("GET", "/photos", nil)
	respPhotos, err := app.Test(reqPhotos, -1)
	if err != nil {
		t.Fatalf("SPA route request failed: %v", err)
	}
	if respPhotos.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200 for SPA route /photos, got %d", respPhotos.StatusCode)
	}
	bodyPhotos, _ := io.ReadAll(respPhotos.Body)
	if !strings.Contains(string(bodyPhotos), "Pixora") {
		t.Errorf("Expected SPA fallback to return index.html containing 'Pixora', got %s", string(bodyPhotos))
	}

	// 4. Client-side route (/timeline) should serve index.html via SPA fallback
	reqTimeline := httptest.NewRequest("GET", "/timeline", nil)
	respTimeline, err := app.Test(reqTimeline, -1)
	if err != nil {
		t.Fatalf("SPA route request failed: %v", err)
	}
	if respTimeline.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200 for SPA route /timeline, got %d", respTimeline.StatusCode)
	}

	// 5. Unknown API route MUST return 404 JSON, NOT index.html
	reqUnknownAPI := httptest.NewRequest("GET", "/api/v1/photos/unknown-route", nil)
	respUnknownAPI, err := app.Test(reqUnknownAPI, -1)
	if err != nil {
		t.Fatalf("Unknown API request failed: %v", err)
	}
	if respUnknownAPI.StatusCode != fiber.StatusNotFound {
		t.Errorf("Expected status 404 for unknown API route, got %d", respUnknownAPI.StatusCode)
	}
	bodyUnknownAPI, _ := io.ReadAll(respUnknownAPI.Body)
	var errResp map[string]string
	if err := json.Unmarshal(bodyUnknownAPI, &errResp); err != nil {
		t.Fatalf("Expected JSON error response for unknown API route, got %s", string(bodyUnknownAPI))
	}
	if errResp["error"] != "not_found" {
		t.Errorf("Expected error 'not_found', got '%s'", errResp["error"])
	}

	// 6. Missing static file with extension should return 404, NOT index.html
	reqMissingAsset := httptest.NewRequest("GET", "/assets/nonexistent-file.css", nil)
	respMissingAsset, err := app.Test(reqMissingAsset, -1)
	if err != nil {
		t.Fatalf("Missing asset request failed: %v", err)
	}
	if respMissingAsset.StatusCode != fiber.StatusNotFound {
		t.Errorf("Expected status 404 for missing static asset, got %d", respMissingAsset.StatusCode)
	}
	bodyMissingAsset, _ := io.ReadAll(respMissingAsset.Body)
	if strings.Contains(string(bodyMissingAsset), "<html") {
		t.Errorf("Missing asset must NOT return index.html!")
	}
}

func TestPhase3FullServerLifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pixora_full_lifecycle_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mediaDir := filepath.Join(tempDir, "media")
	dbPath := filepath.Join(tempDir, "data", "pixora.db")
	_ = os.MkdirAll(mediaDir, 0755)
	_ = os.MkdirAll(filepath.Dir(dbPath), 0755)

	// Pre-populate mediaDir with an existing photo
	existingSubdir := filepath.Join(mediaDir, "nature", "forest")
	_ = os.MkdirAll(existingSubdir, 0755)
	existingPhotoPath := filepath.Join(existingSubdir, "tree.jpg")
	_ = os.WriteFile(existingPhotoPath, createTestJPEG(600, 400), 0644)

	cfg := &config.Config{
		Port:         "8080",
		Host:         "127.0.0.1",
		Env:          "testing",
		StorageRoot:  mediaDir,
		DatabasePath: dbPath,
	}

	store, err := storage.New(cfg.StorageRoot)
	if err != nil {
		t.Fatalf("Failed to init storage: %v", err)
	}

	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer db.Close()

	photoRepo := repository.NewSQLitePhotoRepository(db)
	albumRepo := repository.NewSQLiteAlbumRepository(db)
	thumbSvc := media.NewThumbnailService(store)
	mediaScanner := scanner.New(store, photoRepo)

	app := server.NewWithDependencies(server.Dependencies{
		Config:    cfg,
		DB:        db,
		Storage:   store,
		PhotoRepo: photoRepo,
		AlbumRepo: albumRepo,
		Thumbnail: thumbSvc,
		Scanner:   mediaScanner,
	})

	// 1. Initial list should be empty
	reqList0 := httptest.NewRequest("GET", "/api/photos", nil)
	respList0, _ := app.Test(reqList0, -1)
	var listResp0 struct {
		Photos []api.PhotoResponse `json:"photos"`
		Total  int                 `json:"total"`
	}
	body0, _ := io.ReadAll(respList0.Body)
	_ = json.Unmarshal(body0, &listResp0)
	if listResp0.Total != 0 || len(listResp0.Photos) != 0 {
		t.Fatalf("Expected 0 photos initially, got %d", listResp0.Total)
	}

	// 2. Discover existing photo via Scan API
	reqScan := httptest.NewRequest("POST", "/api/photos/scan?sync=true", nil)
	respScan, err := app.Test(reqScan, -1)
	if err != nil || respScan.StatusCode != fiber.StatusOK {
		t.Fatalf("Scan request failed with status %d: %v", respScan.StatusCode, err)
	}

	var scanStatus scanner.ScanStatus
	scanBody, _ := io.ReadAll(respScan.Body)
	_ = json.Unmarshal(scanBody, &scanStatus)
	if scanStatus.Indexed != 1 {
		t.Fatalf("Expected 1 indexed photo from scan, got %d", scanStatus.Indexed)
	}

	// 3. Verify photo is returned in /api/photos
	reqList1 := httptest.NewRequest("GET", "/api/photos", nil)
	respList1, _ := app.Test(reqList1, -1)
	var listResp1 struct {
		Photos []api.PhotoResponse `json:"photos"`
		Total  int                 `json:"total"`
	}
	body1, _ := io.ReadAll(respList1.Body)
	_ = json.Unmarshal(body1, &listResp1)
	if listResp1.Total != 1 || len(listResp1.Photos) != 1 {
		t.Fatalf("Expected 1 photo after scan, got %d", listResp1.Total)
	}
	indexedPhoto := listResp1.Photos[0]
	if indexedPhoto.OriginalFilename != "tree.jpg" {
		t.Errorf("Expected filename 'tree.jpg', got %s", indexedPhoto.OriginalFilename)
	}

	// 4. Serve original photo: GET /api/photos/:id/file
	reqFile := httptest.NewRequest("GET", indexedPhoto.URL, nil)
	respFile, err := app.Test(reqFile, -1)
	if err != nil || respFile.StatusCode != fiber.StatusOK {
		t.Fatalf("Failed to serve original file: status %d, err %v", respFile.StatusCode, err)
	}
	if respFile.Header.Get("Content-Type") != "image/jpeg" {
		t.Errorf("Expected image/jpeg Content-Type, got %s", respFile.Header.Get("Content-Type"))
	}

	// 5. Generate and serve thumbnail: GET /api/photos/:id/thumbnail
	reqThumb := httptest.NewRequest("GET", indexedPhoto.ThumbnailURL, nil)
	respThumb, err := app.Test(reqThumb, -1)
	if err != nil || respThumb.StatusCode != fiber.StatusOK {
		t.Fatalf("Failed to serve thumbnail: status %d, err %v", respThumb.StatusCode, err)
	}
	if respThumb.Header.Get("Content-Type") != "image/jpeg" {
		t.Errorf("Expected image/jpeg for thumbnail, got %s", respThumb.Header.Get("Content-Type"))
	}

	// 6. Upload new photo: POST /api/photos
	uploadBody := new(bytes.Buffer)
	writer := multipart.NewWriter(uploadBody)
	part, _ := writer.CreateFormFile("photos", "uploaded_lake.png")
	_, _ = part.Write(createTestPNG(400, 300))
	_ = writer.Close()

	reqUpload := httptest.NewRequest("POST", "/api/photos", uploadBody)
	reqUpload.Header.Set("Content-Type", writer.FormDataContentType())
	respUpload, err := app.Test(reqUpload, -1)
	if err != nil || respUpload.StatusCode != fiber.StatusCreated {
		t.Fatalf("Upload failed with status %d: %v", respUpload.StatusCode, err)
	}

	var uploadResp struct {
		Photos []api.PhotoResponse `json:"photos"`
	}
	upBody, _ := io.ReadAll(respUpload.Body)
	_ = json.Unmarshal(upBody, &uploadResp)
	if len(uploadResp.Photos) != 1 {
		t.Fatalf("Expected 1 uploaded photo, got %d", len(uploadResp.Photos))
	}
	uploadedPhoto := uploadResp.Photos[0]
	if uploadedPhoto.OriginalFilename != "uploaded_lake.png" {
		t.Errorf("Expected 'uploaded_lake.png', got %s", uploadedPhoto.OriginalFilename)
	}

	// 7. Verify total count is now 2
	reqList2 := httptest.NewRequest("GET", "/api/photos", nil)
	respList2, _ := app.Test(reqList2, -1)
	var listResp2 struct {
		Total int `json:"total"`
	}
	body2, _ := io.ReadAll(respList2.Body)
	_ = json.Unmarshal(body2, &listResp2)
	if listResp2.Total != 2 {
		t.Errorf("Expected 2 photos after upload, got %d", listResp2.Total)
	}

	// 8. Test Read-Only storage upload rejection
	_ = os.Chmod(mediaDir, 0555)
	defer func() {
		_ = os.Chmod(mediaDir, 0755)
	}()

	roBody := new(bytes.Buffer)
	roWriter := multipart.NewWriter(roBody)
	roPart, _ := roWriter.CreateFormFile("photos", "fail.png")
	_, _ = roPart.Write(createTestPNG(100, 100))
	_ = roWriter.Close()

	reqRO := httptest.NewRequest("POST", "/api/photos", roBody)
	reqRO.Header.Set("Content-Type", roWriter.FormDataContentType())
	respRO, _ := app.Test(reqRO, -1)
	if respRO.StatusCode != fiber.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for upload to read-only storage, got %d", respRO.StatusCode)
	}
}

func TestPhase4FullServerLifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pixora_phase4_lifecycle_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mediaDir := filepath.Join(tempDir, "media")
	dbPath := filepath.Join(tempDir, "data", "pixora.db")
	_ = os.MkdirAll(mediaDir, 0755)
	_ = os.MkdirAll(filepath.Dir(dbPath), 0755)

	// Seed photo in storage
	photoPath := filepath.Join(mediaDir, "nature.jpg")
	_ = os.WriteFile(photoPath, createTestJPEG(600, 400), 0644)

	cfg := &config.Config{
		Port:         "8080",
		Host:         "127.0.0.1",
		Env:          "testing",
		StorageRoot:  mediaDir,
		DatabasePath: dbPath,
	}

	store, err := storage.New(cfg.StorageRoot)
	if err != nil {
		t.Fatalf("Failed to init storage: %v", err)
	}

	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer db.Close()

	photoRepo := repository.NewSQLitePhotoRepository(db)
	albumRepo := repository.NewSQLiteAlbumRepository(db)
	thumbSvc := media.NewThumbnailService(store)
	mediaScanner := scanner.New(store, photoRepo)

	app := server.NewWithDependencies(server.Dependencies{
		Config:    cfg,
		DB:        db,
		Storage:   store,
		PhotoRepo: photoRepo,
		AlbumRepo: albumRepo,
		Thumbnail: thumbSvc,
		Scanner:   mediaScanner,
	})

	// 1. Scan photo
	reqScan := httptest.NewRequest("POST", "/api/photos/scan?sync=true", nil)
	respScan, err := app.Test(reqScan, -1)
	if err != nil || respScan.StatusCode != fiber.StatusOK {
		t.Fatalf("Scan failed: %v", err)
	}

	// 2. Fetch photo ID
	reqList := httptest.NewRequest("GET", "/api/photos", nil)
	respList, _ := app.Test(reqList, -1)
	var listResp struct {
		Photos []api.PhotoResponse `json:"photos"`
	}
	bodyP, _ := io.ReadAll(respList.Body)
	_ = json.Unmarshal(bodyP, &listResp)
	if len(listResp.Photos) != 1 {
		t.Fatalf("Expected 1 scanned photo, got %d", len(listResp.Photos))
	}
	photoID := listResp.Photos[0].ID

	// 3. Create virtual album: POST /api/albums
	createPayload := bytes.NewReader([]byte(`{"name": "Wilderness", "description": "Nature shots"}`))
	reqCreate := httptest.NewRequest("POST", "/api/albums", createPayload)
	reqCreate.Header.Set("Content-Type", "application/json")
	respCreate, err := app.Test(reqCreate, -1)
	if err != nil || respCreate.StatusCode != fiber.StatusCreated {
		t.Fatalf("Create album failed with %v, status %d", err, respCreate.StatusCode)
	}

	var createdAlbum models.Album
	bodyCA, _ := io.ReadAll(respCreate.Body)
	_ = json.Unmarshal(bodyCA, &createdAlbum)
	albumID := createdAlbum.ID

	// Duplicate album name must return 409 Conflict
	dupPayload := bytes.NewReader([]byte(`{"name": "wilderness"}`))
	reqDup := httptest.NewRequest("POST", "/api/albums", dupPayload)
	reqDup.Header.Set("Content-Type", "application/json")
	respDup, _ := app.Test(reqDup, -1)
	if respDup.StatusCode != fiber.StatusConflict {
		t.Errorf("Expected 409 Conflict on duplicate album, got %d", respDup.StatusCode)
	}

	// 4. Add photo to album: POST /api/albums/:id/photos
	addPayload := bytes.NewReader([]byte(fmt.Sprintf(`{"photoIds": ["%s"]}`, photoID)))
	reqAdd := httptest.NewRequest("POST", "/api/albums/"+albumID+"/photos", addPayload)
	reqAdd.Header.Set("Content-Type", "application/json")
	respAdd, err := app.Test(reqAdd, -1)
	if err != nil || respAdd.StatusCode != fiber.StatusOK {
		t.Fatalf("Add photo to album failed: %v, status %d", err, respAdd.StatusCode)
	}

	// 5. Inspect album photo count and cover thumbnail: GET /api/albums/:id
	reqGetAlbum := httptest.NewRequest("GET", "/api/albums/"+albumID, nil)
	respGetAlbum, _ := app.Test(reqGetAlbum, -1)
	var fetchedAlbum models.Album
	bodyFA, _ := io.ReadAll(respGetAlbum.Body)
	_ = json.Unmarshal(bodyFA, &fetchedAlbum)
	if fetchedAlbum.PhotoCount != 1 {
		t.Errorf("Expected photoCount 1, got %d", fetchedAlbum.PhotoCount)
	}
	if fetchedAlbum.CoverThumbnailURL == nil || *fetchedAlbum.CoverThumbnailURL == "" {
		t.Errorf("Expected non-empty cover thumbnail URL on album with photos")
	}

	// 6. List photos in album: GET /api/albums/:id/photos
	reqAlbPhotos := httptest.NewRequest("GET", "/api/albums/"+albumID+"/photos", nil)
	respAlbPhotos, _ := app.Test(reqAlbPhotos, -1)
	if respAlbPhotos.StatusCode != fiber.StatusOK {
		t.Fatalf("Failed to get album photos: %d", respAlbPhotos.StatusCode)
	}

	// 7. Rename album: PATCH /api/albums/:id
	renamePayload := bytes.NewReader([]byte(`{"name": "Wilderness Scenery"}`))
	reqRename := httptest.NewRequest("PATCH", "/api/albums/"+albumID, renamePayload)
	reqRename.Header.Set("Content-Type", "application/json")
	respRename, _ := app.Test(reqRename, -1)
	if respRename.StatusCode != fiber.StatusOK {
		t.Fatalf("Rename failed: %d", respRename.StatusCode)
	}

	// 8. Remove photo from album: DELETE /api/albums/:id/photos/:photoId
	reqRemove := httptest.NewRequest("DELETE", "/api/albums/"+albumID+"/photos/"+photoID, nil)
	respRemove, _ := app.Test(reqRemove, -1)
	if respRemove.StatusCode != fiber.StatusOK {
		t.Fatalf("Remove photo from album failed: %d", respRemove.StatusCode)
	}

	// Photo MUST still exist in global library
	reqCheckP := httptest.NewRequest("GET", "/api/photos/"+photoID, nil)
	respCheckP, _ := app.Test(reqCheckP, -1)
	if respCheckP.StatusCode != fiber.StatusOK {
		t.Errorf("Removing photo from album must NOT delete photo from library, got status %d", respCheckP.StatusCode)
	}

	// 9. Delete album: DELETE /api/albums/:id
	reqDelAlb := httptest.NewRequest("DELETE", "/api/albums/"+albumID, nil)
	respDelAlb, _ := app.Test(reqDelAlb, -1)
	if respDelAlb.StatusCode != fiber.StatusOK {
		t.Fatalf("Delete album failed: %d", respDelAlb.StatusCode)
	}

	// Original photo in DB and file on disk MUST still exist
	respCheckP2, _ := app.Test(reqCheckP, -1)
	if respCheckP2.StatusCode != fiber.StatusOK {
		t.Errorf("Deleting album must NOT delete photo from library, got status %d", respCheckP2.StatusCode)
	}
	if _, err := os.Stat(photoPath); os.IsNotExist(err) {
		t.Errorf("Deleting album must NEVER delete files in STORAGE_ROOT")
	}
}

func TestPhysicalFolderBrowsingFullLifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pixora_folder_lifecycle_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mediaDir := filepath.Join(tempDir, "media")
	dbPath := filepath.Join(tempDir, "data", "pixora.db")
	_ = os.MkdirAll(mediaDir, 0755)
	_ = os.MkdirAll(filepath.Dir(dbPath), 0755)

	// Create nested structure:
	// Pictures/Anime/Movies/film1.jpg
	// Pictures/pic1.jpg
	// Vacation/beach.jpg
	moviesDir := filepath.Join(mediaDir, "Pictures", "Anime", "Movies")
	vacationDir := filepath.Join(mediaDir, "Vacation")
	_ = os.MkdirAll(moviesDir, 0755)
	_ = os.MkdirAll(vacationDir, 0755)

	_ = os.WriteFile(filepath.Join(moviesDir, "film1.jpg"), createTestJPEG(600, 400), 0644)
	_ = os.WriteFile(filepath.Join(mediaDir, "Pictures", "pic1.jpg"), createTestJPEG(800, 600), 0644)
	_ = os.WriteFile(filepath.Join(vacationDir, "beach.jpg"), createTestJPEG(1024, 768), 0644)

	cfg := &config.Config{
		Port:         "8080",
		Host:         "127.0.0.1",
		Env:          "testing",
		StorageRoot:  mediaDir,
		DatabasePath: dbPath,
	}

	store, err := storage.New(cfg.StorageRoot)
	if err != nil {
		t.Fatalf("Failed to init storage: %v", err)
	}

	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer db.Close()

	photoRepo := repository.NewSQLitePhotoRepository(db)
	albumRepo := repository.NewSQLiteAlbumRepository(db)
	thumbSvc := media.NewThumbnailService(store)
	mediaScanner := scanner.New(store, photoRepo)

	app := server.NewWithDependencies(server.Dependencies{
		Config:    cfg,
		DB:        db,
		Storage:   store,
		PhotoRepo: photoRepo,
		AlbumRepo: albumRepo,
		Thumbnail: thumbSvc,
		Scanner:   mediaScanner,
	})

	// 1. Initial folder listing before scan: folders exist on disk with 0 indexed photos
	reqFolders0 := httptest.NewRequest("GET", "/api/folders", nil)
	respFolders0, err := app.Test(reqFolders0, -1)
	if err != nil || respFolders0.StatusCode != fiber.StatusOK {
		t.Fatalf("GET /api/folders failed: %v, status: %d", err, respFolders0.StatusCode)
	}

	var list0 api.FolderListResponse
	_ = json.NewDecoder(respFolders0.Body).Decode(&list0)
	if list0.Total != 2 {
		t.Errorf("Expected 2 top-level physical folders (Pictures, Vacation), got %d", list0.Total)
	}

	// 2. Scan entire media library: POST /api/photos/scan?sync=true
	reqScan := httptest.NewRequest("POST", "/api/photos/scan?sync=true", nil)
	respScan, err := app.Test(reqScan, -1)
	if err != nil || respScan.StatusCode != fiber.StatusOK {
		t.Fatalf("Scan failed: %v, status %d", err, respScan.StatusCode)
	}

	// 3. Inspect top-level folders after scan
	reqFolders1 := httptest.NewRequest("GET", "/api/folders", nil)
	respFolders1, _ := app.Test(reqFolders1, -1)
	var list1 api.FolderListResponse
	_ = json.NewDecoder(respFolders1.Body).Decode(&list1)

	for _, f := range list1.Folders {
		if f.Name == "Pictures" {
			// Pictures contains pic1.jpg (direct: 1) and film1.jpg in Anime/Movies (total: 2)
			if f.PhotoCount != 2 || f.DirectPhotoCount != 1 || f.SubfolderCount != 1 {
				t.Errorf("Pictures folder counts mismatch: %+v", f)
			}
		} else if f.Name == "Vacation" {
			if f.PhotoCount != 1 || f.DirectPhotoCount != 1 {
				t.Errorf("Vacation folder counts mismatch: %+v", f)
			}
		}
	}

	// 4. Navigate into Pictures: GET /api/folders/contents?path=Pictures
	reqPic := httptest.NewRequest("GET", "/api/folders/contents?path=Pictures", nil)
	respPic, _ := app.Test(reqPic, -1)
	var picContents api.FolderContentsResponse
	_ = json.NewDecoder(respPic.Body).Decode(&picContents)

	if picContents.TotalPhotos != 1 || len(picContents.Photos) != 1 {
		t.Errorf("Expected 1 direct photo in Pictures, got %d", picContents.TotalPhotos)
	}
	if len(picContents.Folders) != 1 || picContents.Folders[0].Name != "Anime" {
		t.Errorf("Expected 1 child folder 'Anime' in Pictures, got %+v", picContents.Folders)
	}
	if len(picContents.Breadcrumbs) != 2 {
		t.Errorf("Expected 2 breadcrumbs for Pictures, got %d", len(picContents.Breadcrumbs))
	}

	// 5. Navigate into nested Movies: GET /api/folders/contents?path=Pictures/Anime/Movies
	reqMovies := httptest.NewRequest("GET", "/api/folders/contents?path=Pictures/Anime/Movies", nil)
	respMovies, _ := app.Test(reqMovies, -1)
	var movieContents api.FolderContentsResponse
	_ = json.NewDecoder(respMovies.Body).Decode(&movieContents)

	if movieContents.TotalPhotos != 1 || len(movieContents.Photos) != 1 {
		t.Errorf("Expected 1 direct photo in Movies, got %d", movieContents.TotalPhotos)
	}
	if len(movieContents.Folders) != 0 {
		t.Errorf("Expected 0 subfolders in Movies, got %d", len(movieContents.Folders))
	}
	if len(movieContents.Breadcrumbs) != 4 {
		t.Errorf("Expected 4 breadcrumbs for Movies, got %d", len(movieContents.Breadcrumbs))
	}
	if movieContents.ParentPath == nil || *movieContents.ParentPath != "Pictures/Anime" {
		t.Errorf("Expected parentPath 'Pictures/Anime', got %v", movieContents.ParentPath)
	}

	// 6. Paginate photos in folder: GET /api/folders/photos?path=Pictures/Anime/Movies
	reqMoviePhotos := httptest.NewRequest("GET", "/api/folders/photos?path=Pictures/Anime/Movies&limit=5&offset=0", nil)
	respMoviePhotos, _ := app.Test(reqMoviePhotos, -1)
	if respMoviePhotos.StatusCode != fiber.StatusOK {
		t.Fatalf("GET /api/folders/photos failed: %d", respMoviePhotos.StatusCode)
	}

	// 7. Test traversal rejection: GET /api/folders/contents?path=../escape
	reqBad := httptest.NewRequest("GET", "/api/folders/contents?path=../escape", nil)
	respBad, _ := app.Test(reqBad, -1)
	if respBad.StatusCode != fiber.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for traversal, got %d", respBad.StatusCode)
	}

	// 8. Verify all original media files in mediaDir are intact and untouched
	for _, expectedFile := range []string{
		filepath.Join(moviesDir, "film1.jpg"),
		filepath.Join(mediaDir, "Pictures", "pic1.jpg"),
		filepath.Join(vacationDir, "beach.jpg"),
	} {
		if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
			t.Errorf("Original media file %s was unexpectedly removed or modified", expectedFile)
		}
	}
}

func TestPhase5TimelineAndViewerLifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pixora_phase5_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mediaDir := filepath.Join(tempDir, "media")
	dbPath := filepath.Join(tempDir, "data", "pixora.db")
	_ = os.MkdirAll(mediaDir, 0755)
	_ = os.MkdirAll(filepath.Dir(dbPath), 0755)

	// Create test image files in storage
	_ = os.WriteFile(filepath.Join(mediaDir, "nature_dated1.jpg"), createTestJPEG(600, 400), 0644)
	_ = os.WriteFile(filepath.Join(mediaDir, "nature_dated2.jpg"), createTestJPEG(800, 600), 0644)
	_ = os.WriteFile(filepath.Join(mediaDir, "graphic_undated.jpg"), createTestJPEG(400, 400), 0644)

	cfg := &config.Config{
		Port:         "8080",
		Host:         "127.0.0.1",
		Env:          "testing",
		StorageRoot:  mediaDir,
		DatabasePath: dbPath,
	}

	store, err := storage.New(cfg.StorageRoot)
	if err != nil {
		t.Fatalf("Failed to init storage: %v", err)
	}

	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer db.Close()

	photoRepo := repository.NewSQLitePhotoRepository(db)
	albumRepo := repository.NewSQLiteAlbumRepository(db)
	thumbSvc := media.NewThumbnailService(store)
	mediaScanner := scanner.New(store, photoRepo)

	app := server.NewWithDependencies(server.Dependencies{
		Config:    cfg,
		DB:        db,
		Storage:   store,
		PhotoRepo: photoRepo,
		AlbumRepo: albumRepo,
		Thumbnail: thumbSvc,
		Scanner:   mediaScanner,
	})

	// 1. Scan photos into database
	reqScan := httptest.NewRequest("POST", "/api/photos/scan?sync=true", nil)
	respScan, err := app.Test(reqScan, -1)
	if err != nil || respScan.StatusCode != fiber.StatusOK {
		t.Fatalf("Initial scan failed: %v, status %d", err, respScan.StatusCode)
	}

	// Retrieve scanned records
	allPhotos, _, err := photoRepo.List(t.Context(), 0, 10, "filename", "asc")
	if err != nil || len(allPhotos) != 3 {
		t.Fatalf("Expected 3 indexed photos, got %d (err: %v)", len(allPhotos), err)
	}

	// Update two photos to have verified capture dates (DateTimeOriginal)
	tCapture1 := time.Date(2026, 10, 24, 15, 30, 0, 0, time.UTC)
	tCapture2 := time.Date(2026, 10, 24, 10, 15, 0, 0, time.UTC)

	pDated1 := allPhotos[1] // nature_dated1.jpg
	pDated1.TakenAt = &tCapture1
	_ = photoRepo.Update(t.Context(), &pDated1)

	pDated2 := allPhotos[2] // nature_dated2.jpg
	pDated2.TakenAt = &tCapture2
	_ = photoRepo.Update(t.Context(), &pDated2)

	// pUndated remains with TakenAt = nil
	pUndated := allPhotos[0] // graphic_undated.jpg

	// 2. Query GET /api/timeline
	reqTimeline := httptest.NewRequest("GET", "/api/timeline", nil)
	respTimeline, err := app.Test(reqTimeline, -1)
	if err != nil {
		t.Fatalf("Timeline request failed: %v", err)
	}
	if respTimeline.StatusCode != fiber.StatusOK {
		t.Fatalf("Expected 200 for /api/timeline, got %d", respTimeline.StatusCode)
	}

	var timelineResp api.TimelineResponse
	_ = json.NewDecoder(respTimeline.Body).Decode(&timelineResp)

	if timelineResp.Total != 3 || len(timelineResp.Photos) != 3 {
		t.Fatalf("Expected 3 photos in timeline, got %d", len(timelineResp.Photos))
	}

	// Check chronological ordering:
	// pDated1 (15:30) -> pDated2 (10:15) -> pUndated (nil capture date)
	if timelineResp.Photos[0].ID != pDated1.ID {
		t.Errorf("First photo should be newest capture date %s, got %s", pDated1.ID, timelineResp.Photos[0].ID)
	}
	if timelineResp.Photos[1].ID != pDated2.ID {
		t.Errorf("Second photo should be earlier capture date %s, got %s", pDated2.ID, timelineResp.Photos[1].ID)
	}
	if timelineResp.Photos[2].ID != pUndated.ID {
		t.Errorf("Third photo should be undated %s, got %s", pUndated.ID, timelineResp.Photos[2].ID)
	}
	if !timelineResp.Photos[2].IsUndated {
		t.Errorf("Expected third photo to be flagged as isUndated=true")
	}

	// Check Day Grouping
	if len(timelineResp.Groups) != 2 {
		t.Fatalf("Expected 2 groups (dated day + undated), got %d", len(timelineResp.Groups))
	}
	datedGroup := timelineResp.Groups[0]
	if datedGroup.Date != "2026-10-24" || len(datedGroup.Photos) != 2 || datedGroup.IsUndated {
		t.Errorf("Unexpected dated group: %+v", datedGroup)
	}
	undatedGroup := timelineResp.Groups[1]
	if undatedGroup.Date != "undated" || len(undatedGroup.Photos) != 1 || !undatedGroup.IsUndated {
		t.Errorf("Unexpected undated group: %+v", undatedGroup)
	}

	// 3. Test cursor pagination
	reqCursor := httptest.NewRequest("GET", "/api/timeline?limit=1&offset=0", nil)
	respCursor, _ := app.Test(reqCursor, -1)
	var cursorResp api.TimelineResponse
	_ = json.NewDecoder(respCursor.Body).Decode(&cursorResp)
	if !cursorResp.HasMore || cursorResp.NextCursor == "" {
		t.Errorf("Expected hasMore=true and valid nextCursor for limit=1")
	}

	reqNext := httptest.NewRequest("GET", "/api/timeline?limit=1&cursor="+cursorResp.NextCursor, nil)
	respNext, _ := app.Test(reqNext, -1)
	var nextResp api.TimelineResponse
	_ = json.NewDecoder(respNext.Body).Decode(&nextResp)
	if len(nextResp.Photos) != 1 || nextResp.Photos[0].ID != pDated2.ID {
		t.Errorf("Expected second photo via cursor, got: %+v", nextResp.Photos)
	}

	// 4. Test GET /api/photos/:id with adjacent lookup
	reqAdj := httptest.NewRequest("GET", fmt.Sprintf("/api/photos/%s?collectionType=timeline", pDated2.ID), nil)
	respAdj, _ := app.Test(reqAdj, -1)
	var pAdj api.PhotoResponse
	_ = json.NewDecoder(respAdj.Body).Decode(&pAdj)

	if pAdj.PrevID == nil || *pAdj.PrevID != pDated1.ID {
		t.Errorf("Expected PrevID %s, got %v", pDated1.ID, pAdj.PrevID)
	}
	if pAdj.NextID == nil || *pAdj.NextID != pUndated.ID {
		t.Errorf("Expected NextID %s, got %v", pUndated.ID, pAdj.NextID)
	}

	// 5. Verify Photo Viewer endpoints
	reqFile := httptest.NewRequest("GET", fmt.Sprintf("/api/photos/%s/file", pDated1.ID), nil)
	respFile, _ := app.Test(reqFile, -1)
	if respFile.StatusCode != fiber.StatusOK || respFile.Header.Get("Content-Type") != "image/jpeg" {
		t.Errorf("Expected 200 image/jpeg serving original file, got status %d", respFile.StatusCode)
	}

	reqThumb := httptest.NewRequest("GET", fmt.Sprintf("/api/photos/%s/thumbnail", pDated1.ID), nil)
	respThumb, _ := app.Test(reqThumb, -1)
	if respThumb.StatusCode != fiber.StatusOK || respThumb.Header.Get("Content-Type") != "image/jpeg" {
		t.Errorf("Expected 200 image/jpeg serving thumbnail, got status %d", respThumb.StatusCode)
	}

	// 6. Test GET /api/photos/:id with nonexistent ID returns 404
	reqMissing := httptest.NewRequest("GET", "/api/photos/00000000-0000-0000-0000-000000000000", nil)
	respMissing, _ := app.Test(reqMissing, -1)
	if respMissing.StatusCode != fiber.StatusNotFound {
		t.Errorf("Expected 404 for missing photo ID, got %d", respMissing.StatusCode)
	}

	// 7. Non-destructive guarantee: files on disk still exist
	for _, f := range []string{"nature_dated1.jpg", "nature_dated2.jpg", "graphic_undated.jpg"} {
		if _, err := os.Stat(filepath.Join(mediaDir, f)); os.IsNotExist(err) {
			t.Errorf("File %s was unexpectedly removed", f)
		}
	}
}
