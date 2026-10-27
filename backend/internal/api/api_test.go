package api_test

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
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"pixora/backend/internal/api"
	"pixora/backend/internal/database"
	"pixora/backend/internal/media"
	"pixora/backend/internal/models"
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
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	buf := new(bytes.Buffer)
	_ = png.Encode(buf, img)
	return buf.Bytes()
}

func setupTestApp(t *testing.T) (*fiber.App, *database.DB, *storage.Storage, *scanner.Scanner, *media.ThumbnailService, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "pixora_api_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	mediaDir := filepath.Join(tempDir, "media")
	dbDir := filepath.Join(tempDir, "data")
	_ = os.MkdirAll(mediaDir, 0755)
	_ = os.MkdirAll(dbDir, 0755)

	db, err := database.Open(filepath.Join(dbDir, "api_test.db"))
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
	albumRepo := repository.NewSQLiteAlbumRepository(db)
	thumbSvc := media.NewThumbnailService(store)
	scanSvc := scanner.New(store, photoRepo)

	app := fiber.New()
	apiGroup := app.Group("/api")

	// Albums
	apiGroup.Get("/albums", api.ListAlbumsHandler(albumRepo))
	apiGroup.Post("/albums", api.CreateAlbumHandler(albumRepo))
	apiGroup.Get("/albums/:id", api.GetAlbumHandler(albumRepo))

	// Photos
	apiGroup.Get("/photos", api.ListPhotosHandler(photoRepo))
	apiGroup.Post("/photos", api.UploadPhotosHandler(photoRepo, store, thumbSvc))
	apiGroup.Post("/photos/upload", api.UploadPhotosHandler(photoRepo, store, thumbSvc))
	apiGroup.Get("/photos/scan", api.GetScanStatusHandler(scanSvc))
	apiGroup.Get("/photos/scan/status", api.GetScanStatusHandler(scanSvc))
	apiGroup.Post("/photos/scan", api.ScanPhotosHandler(scanSvc))
	apiGroup.Get("/photos/:id", api.GetPhotoHandler(photoRepo))
	apiGroup.Get("/photos/:id/file", api.ServeOriginalPhotoHandler(photoRepo, store))
	apiGroup.Get("/photos/:id/thumbnail", api.ServeThumbnailHandler(photoRepo, store, thumbSvc))

	cleanup := func() {
		_ = db.Close()
		_ = os.RemoveAll(tempDir)
	}

	return app, db, store, scanSvc, thumbSvc, cleanup
}

func TestAlbumsAPI(t *testing.T) {
	app, _, _, _, _, cleanup := setupTestApp(t)
	defer cleanup()

	// 1. Validation failure: empty name
	badPayload := bytes.NewReader([]byte(`{"name": "   "}`))
	req := httptest.NewRequest("POST", "/api/albums", badPayload)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed request: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected 400 for empty name, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var errResp api.ErrorResponse
	_ = json.Unmarshal(body, &errResp)
	if errResp.Error != "validation_error" {
		t.Errorf("Expected error 'validation_error', got %s", errResp.Error)
	}

	// 2. Success: create album
	validPayload := bytes.NewReader([]byte(`{"name": "Nature", "description": "Outdoor photos"}`))
	req = httptest.NewRequest("POST", "/api/albums", validPayload)
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed request: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("Expected 201 Created, got %d", resp.StatusCode)
	}

	body, _ = io.ReadAll(resp.Body)
	var createdAlbum models.Album
	if err := json.Unmarshal(body, &createdAlbum); err != nil {
		t.Fatalf("Failed to parse created album: %v", err)
	}
	if createdAlbum.Name != "Nature" || createdAlbum.ID == "" {
		t.Errorf("Album creation data mismatch: %+v", createdAlbum)
	}

	// 3. GET /api/albums
	req = httptest.NewRequest("GET", "/api/albums", nil)
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed request: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	body, _ = io.ReadAll(resp.Body)
	var listResp struct {
		Albums []models.Album `json:"albums"`
	}
	_ = json.Unmarshal(body, &listResp)
	if len(listResp.Albums) != 1 || listResp.Albums[0].ID != createdAlbum.ID {
		t.Errorf("Expected 1 album in list, got %+v", listResp.Albums)
	}

	// 4. GET /api/albums/:id
	req = httptest.NewRequest("GET", "/api/albums/"+createdAlbum.ID, nil)
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed request: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	// 5. GET /api/albums/:id not found
	req = httptest.NewRequest("GET", "/api/albums/non-existent-id", nil)
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed request: %v", err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("Expected 404, got %d", resp.StatusCode)
	}
}

func TestPhotosAPI(t *testing.T) {
	app, db, store, _, _, cleanup := setupTestApp(t)
	defer cleanup()

	photoRepo := repository.NewSQLitePhotoRepository(db)
	ctx := t.Context()

	// Write an image file to storage
	relPath := "originals/api_photo.jpg"
	absPath, err := store.ResolvePath(relPath)
	if err != nil {
		t.Fatalf("Failed to resolve path: %v", err)
	}
	_ = os.MkdirAll(filepath.Dir(absPath), 0755)
	jpegBytes := createTestJPEG(640, 480)
	_ = os.WriteFile(absPath, jpegBytes, 0644)

	photo := &models.Photo{
		ID:               "p-api-1",
		RelativePath:     relPath,
		OriginalFilename: "api_photo.jpg",
		MediaType:        "image/jpeg",
		FileSize:         int64(len(jpegBytes)),
		FileModifiedAt:   time.Now().UTC(),
	}
	if err := photoRepo.Create(ctx, photo); err != nil {
		t.Fatalf("Failed to seed photo: %v", err)
	}

	// 1. GET /api/photos
	req := httptest.NewRequest("GET", "/api/photos?limit=10&offset=0", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed request: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var photoListResp struct {
		Photos []api.PhotoResponse `json:"photos"`
		Total  int                 `json:"total"`
		Limit  int                 `json:"limit"`
		Offset int                 `json:"offset"`
	}
	_ = json.Unmarshal(body, &photoListResp)
	if photoListResp.Total != 1 || len(photoListResp.Photos) != 1 {
		t.Errorf("Unexpected photos list: %+v", photoListResp)
	}
	if photoListResp.Photos[0].URL != "/api/photos/p-api-1/file" {
		t.Errorf("Expected URL /api/photos/p-api-1/file, got %s", photoListResp.Photos[0].URL)
	}
	if photoListResp.Photos[0].ThumbnailURL != "/api/photos/p-api-1/thumbnail" {
		t.Errorf("Expected ThumbnailURL /api/photos/p-api-1/thumbnail, got %s", photoListResp.Photos[0].ThumbnailURL)
	}

	// 2. GET /api/photos invalid limit
	req = httptest.NewRequest("GET", "/api/photos?limit=-5", nil)
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed request: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected 400 for negative limit, got %d", resp.StatusCode)
	}

	// 3. GET /api/photos/:id
	req = httptest.NewRequest("GET", "/api/photos/p-api-1", nil)
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed request: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	// 4. GET /api/photos/:id not found
	req = httptest.NewRequest("GET", "/api/photos/missing-photo", nil)
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed request: %v", err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("Expected 404 for missing photo, got %d", resp.StatusCode)
	}

	// 5. GET /api/photos/:id/file (Serve original)
	req = httptest.NewRequest("GET", "/api/photos/p-api-1/file", nil)
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed request to serve file: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("Expected 200 serving file, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "image/jpeg" {
		t.Errorf("Expected image/jpeg Content-Type, got %s", resp.Header.Get("Content-Type"))
	}
	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Error("Expected ETag header on original photo")
	}

	// 6. Test conditional 304 Not Modified
	req304 := httptest.NewRequest("GET", "/api/photos/p-api-1/file", nil)
	req304.Header.Set("If-None-Match", etag)
	resp304, _ := app.Test(req304, -1)
	if resp304.StatusCode != fiber.StatusNotModified {
		t.Errorf("Expected 304 Not Modified, got %d", resp304.StatusCode)
	}

	// 7. GET /api/photos/:id/thumbnail
	reqThumb := httptest.NewRequest("GET", "/api/photos/p-api-1/thumbnail", nil)
	respThumb, err := app.Test(reqThumb, -1)
	if err != nil {
		t.Fatalf("Failed request for thumbnail: %v", err)
	}
	if respThumb.StatusCode != fiber.StatusOK {
		t.Fatalf("Expected 200 serving thumbnail, got %d", respThumb.StatusCode)
	}
	if respThumb.Header.Get("Content-Type") != "image/jpeg" {
		t.Errorf("Expected image/jpeg for thumbnail, got %s", respThumb.Header.Get("Content-Type"))
	}
}

func TestUploadAPI(t *testing.T) {
	app, _, _, _, _, cleanup := setupTestApp(t)
	defer cleanup()

	// 1. Successful upload of JPEG and PNG
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	// Add valid JPEG
	part1, _ := writer.CreateFormFile("photos", "holiday.jpg")
	_, _ = part1.Write(createTestJPEG(400, 300))

	// Add valid PNG
	part2, _ := writer.CreateFormFile("photos", "logo.png")
	_, _ = part2.Write(createTestPNG(200, 200))

	_ = writer.Close()

	req := httptest.NewRequest("POST", "/api/photos", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Upload request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 201 Created, got %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	var uploadResp struct {
		Photos []api.PhotoResponse `json:"photos"`
	}
	_ = json.Unmarshal(respBody, &uploadResp)
	if len(uploadResp.Photos) != 2 {
		t.Fatalf("Expected 2 uploaded photos, got %d", len(uploadResp.Photos))
	}

	// Verify photos appear in GET /api/photos
	reqList := httptest.NewRequest("GET", "/api/photos", nil)
	respList, _ := app.Test(reqList, -1)
	var listResp struct {
		Total int `json:"total"`
	}
	listBody, _ := io.ReadAll(respList.Body)
	_ = json.Unmarshal(listBody, &listResp)
	if listResp.Total != 2 {
		t.Errorf("Expected total=2 after upload, got %d", listResp.Total)
	}

	// 2. Reject unsupported file format
	badBody := new(bytes.Buffer)
	badWriter := multipart.NewWriter(badBody)
	badPart, _ := badWriter.CreateFormFile("photos", "danger.exe")
	_, _ = badPart.Write([]byte("not an image"))
	_ = badWriter.Close()

	badReq := httptest.NewRequest("POST", "/api/photos", badBody)
	badReq.Header.Set("Content-Type", badWriter.FormDataContentType())
	badResp, err := app.Test(badReq, -1)
	if err != nil {
		t.Fatalf("Bad upload request failed: %v", err)
	}
	if badResp.StatusCode != fiber.StatusUnprocessableEntity {
		t.Errorf("Expected 422 for unsupported format, got %d", badResp.StatusCode)
	}

	// 3. Reject empty upload request
	emptyBody := new(bytes.Buffer)
	emptyWriter := multipart.NewWriter(emptyBody)
	_ = emptyWriter.Close()
	emptyReq := httptest.NewRequest("POST", "/api/photos", emptyBody)
	emptyReq.Header.Set("Content-Type", emptyWriter.FormDataContentType())
	emptyResp, _ := app.Test(emptyReq, -1)
	if emptyResp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected 400 for empty form, got %d", emptyResp.StatusCode)
	}
}

func TestScanAPI(t *testing.T) {
	app, _, store, _, _, cleanup := setupTestApp(t)
	defer cleanup()

	// Seed media directory with photos
	_ = os.WriteFile(filepath.Join(store.Root(), "camera1.jpg"), createTestJPEG(300, 200), 0644)
	_ = os.WriteFile(filepath.Join(store.Root(), "camera2.png"), createTestPNG(400, 400), 0644)

	// 1. Initial scan status check: GET /api/photos/scan
	reqStatus := httptest.NewRequest("GET", "/api/photos/scan", nil)
	respStatus, err := app.Test(reqStatus, -1)
	if err != nil {
		t.Fatalf("Failed to get scan status: %v", err)
	}
	if respStatus.StatusCode != fiber.StatusOK {
		t.Errorf("Expected 200 for scan status, got %d", respStatus.StatusCode)
	}

	// 2. Run synchronous scan: POST /api/photos/scan?sync=true
	reqScan := httptest.NewRequest("POST", "/api/photos/scan?sync=true", nil)
	respScan, err := app.Test(reqScan, -1)
	if err != nil {
		t.Fatalf("Failed to trigger scan: %v", err)
	}
	if respScan.StatusCode != fiber.StatusOK {
		t.Fatalf("Expected 200 for sync scan, got %d", respScan.StatusCode)
	}

	scanBody, _ := io.ReadAll(respScan.Body)
	var status struct {
		Status  string `json:"status"`
		Indexed int    `json:"indexed"`
		Scanned int    `json:"scanned"`
	}
	_ = json.Unmarshal(scanBody, &status)
	if status.Status != "completed" || status.Indexed != 2 {
		t.Errorf("Unexpected scan result: %+v", status)
	}

	// 3. Verify photos are returned in GET /api/photos
	reqPhotos := httptest.NewRequest("GET", "/api/photos", nil)
	respPhotos, _ := app.Test(reqPhotos, -1)
	var photosResp struct {
		Total int `json:"total"`
	}
	photosBody, _ := io.ReadAll(respPhotos.Body)
	_ = json.Unmarshal(photosBody, &photosResp)
	if photosResp.Total != 2 {
		t.Errorf("Expected 2 indexed photos, got %d", photosResp.Total)
	}
}
