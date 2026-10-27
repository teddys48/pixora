package server_test

import (
	"bytes"
	"encoding/json"
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

	"github.com/gofiber/fiber/v2"

	"pixora/backend/internal/api"
	"pixora/backend/internal/config"
	"pixora/backend/internal/database"
	"pixora/backend/internal/media"
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
