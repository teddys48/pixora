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
	"strings"
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
	apiGroup.Patch("/albums/:id", api.UpdateAlbumHandler(albumRepo))
	apiGroup.Delete("/albums/:id", api.DeleteAlbumHandler(albumRepo))
	apiGroup.Get("/albums/:id/photos", api.GetAlbumPhotosHandler(albumRepo))
	apiGroup.Post("/albums/:id/photos", api.AddPhotosToAlbumHandler(albumRepo))
	apiGroup.Delete("/albums/:id/photos/:photoId", api.RemovePhotoFromAlbumHandler(albumRepo))

	// Timeline
	apiGroup.Get("/timeline", api.TimelineHandler(photoRepo))

	// Photos
	apiGroup.Get("/photos", api.ListPhotosHandler(photoRepo))
	apiGroup.Post("/photos", api.UploadPhotosHandler(photoRepo, store, thumbSvc))
	apiGroup.Post("/photos/upload", api.UploadPhotosHandler(photoRepo, store, thumbSvc))
	apiGroup.Get("/photos/scan", api.GetScanStatusHandler(scanSvc))
	apiGroup.Get("/photos/scan/status", api.GetScanStatusHandler(scanSvc))
	apiGroup.Post("/photos/scan", api.ScanPhotosHandler(scanSvc))
	apiGroup.Get("/photos/:id", api.GetPhotoHandler(photoRepo, store))
	apiGroup.Get("/photos/:id/file", api.ServeOriginalPhotoHandler(photoRepo, store))
	apiGroup.Get("/photos/:id/thumbnail", api.ServeThumbnailHandler(photoRepo, store, thumbSvc))

	cleanup := func() {
		_ = db.Close()
		_ = os.RemoveAll(tempDir)
	}

	return app, db, store, scanSvc, thumbSvc, cleanup
}

func TestAlbumsAPI(t *testing.T) {
	app, db, store, _, _, cleanup := setupTestApp(t)
	defer cleanup()

	photoRepo := repository.NewSQLitePhotoRepository(db)
	ctx := t.Context()

	// Seed two real photos in storage and repository
	relPath1 := "originals/nature1.jpg"
	absPath1, _ := store.ResolvePath(relPath1)
	_ = os.MkdirAll(filepath.Dir(absPath1), 0755)
	jpeg1 := createTestJPEG(400, 300)
	_ = os.WriteFile(absPath1, jpeg1, 0644)

	p1 := &models.Photo{
		ID:               "photo-alb-1",
		RelativePath:     relPath1,
		OriginalFilename: "nature1.jpg",
		MediaType:        "image/jpeg",
		FileSize:         int64(len(jpeg1)),
		FileModifiedAt:   time.Now().UTC(),
	}
	if err := photoRepo.Create(ctx, p1); err != nil {
		t.Fatalf("Failed to seed photo 1: %v", err)
	}

	relPath2 := "originals/nature2.jpg"
	absPath2, _ := store.ResolvePath(relPath2)
	jpeg2 := createTestJPEG(400, 300)
	_ = os.WriteFile(absPath2, jpeg2, 0644)

	p2 := &models.Photo{
		ID:               "photo-alb-2",
		RelativePath:     relPath2,
		OriginalFilename: "nature2.jpg",
		MediaType:        "image/jpeg",
		FileSize:         int64(len(jpeg2)),
		FileModifiedAt:   time.Now().UTC(),
	}
	if err := photoRepo.Create(ctx, p2); err != nil {
		t.Fatalf("Failed to seed photo 2: %v", err)
	}

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

	// 2. Validation failure: name too long (>100 characters)
	longName := strings.Repeat("a", 101)
	longPayload := bytes.NewReader([]byte(`{"name": "` + longName + `"}`))
	req = httptest.NewRequest("POST", "/api/albums", longPayload)
	req.Header.Set("Content-Type", "application/json")
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected 400 for name > 100 chars, got %d", resp.StatusCode)
	}

	// 3. Success: create album
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

	body, _ := io.ReadAll(resp.Body)
	var createdAlbum models.Album
	if err := json.Unmarshal(body, &createdAlbum); err != nil {
		t.Fatalf("Failed to parse created album: %v", err)
	}
	if createdAlbum.Name != "Nature" || createdAlbum.ID == "" {
		t.Errorf("Album creation data mismatch: %+v", createdAlbum)
	}

	// 4. Duplicate name (case-insensitive) returns 409 Conflict
	dupPayload := bytes.NewReader([]byte(`{"name": "nature"}`))
	req = httptest.NewRequest("POST", "/api/albums", dupPayload)
	req.Header.Set("Content-Type", "application/json")
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != fiber.StatusConflict {
		t.Errorf("Expected 409 Conflict for duplicate case-insensitive name, got %d", resp.StatusCode)
	}

	// 5. GET /api/albums with search & sorting
	req = httptest.NewRequest("GET", "/api/albums?search=nat&sort=name&order=asc", nil)
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	body, _ = io.ReadAll(resp.Body)
	var listResp struct {
		Albums []models.Album `json:"albums"`
	}
	_ = json.Unmarshal(body, &listResp)
	if len(listResp.Albums) != 1 || listResp.Albums[0].ID != createdAlbum.ID {
		t.Errorf("Expected 1 matching album in list, got %+v", listResp.Albums)
	}

	// 6. GET /api/albums/:id
	req = httptest.NewRequest("GET", "/api/albums/"+createdAlbum.ID, nil)
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	// 7. GET /api/albums/:id not found
	req = httptest.NewRequest("GET", "/api/albums/non-existent-id", nil)
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("Expected 404, got %d", resp.StatusCode)
	}

	// 8. POST /api/albums/:id/photos (Batch add photos)
	addPhotosPayload := bytes.NewReader([]byte(`{"photoIds": ["photo-alb-1", "photo-alb-2"]}`))
	req = httptest.NewRequest("POST", "/api/albums/"+createdAlbum.ID+"/photos", addPhotosPayload)
	req.Header.Set("Content-Type", "application/json")
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("Expected 200 adding photos, got %d", resp.StatusCode)
	}
	var addResp struct {
		Added int `json:"added"`
	}
	body, _ = io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &addResp)
	if addResp.Added != 2 {
		t.Errorf("Expected 2 added photos, got %d", addResp.Added)
	}

	// 9. Re-adding existing photos should be idempotent and prevent duplicate associations
	req = httptest.NewRequest("POST", "/api/albums/"+createdAlbum.ID+"/photos", bytes.NewReader([]byte(`{"photoIds": ["photo-alb-1"]}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected 200 on duplicate add, got %d", resp.StatusCode)
	}

	// 10. Adding nonexistent photo ID returns 404 Not Found
	badPhotoPayload := bytes.NewReader([]byte(`{"photoIds": ["missing-photo-id"]}`))
	req = httptest.NewRequest("POST", "/api/albums/"+createdAlbum.ID+"/photos", badPhotoPayload)
	req.Header.Set("Content-Type", "application/json")
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("Expected 404 adding nonexistent photo, got %d", resp.StatusCode)
	}

	// 11. GET /api/albums/:id/photos
	req = httptest.NewRequest("GET", "/api/albums/"+createdAlbum.ID+"/photos?limit=10&offset=0", nil)
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("Expected 200 getting album photos, got %d", resp.StatusCode)
	}
	var albumPhotosResp struct {
		Photos []api.PhotoResponse `json:"photos"`
		Total  int                 `json:"total"`
	}
	body, _ = io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &albumPhotosResp)
	if albumPhotosResp.Total != 2 || len(albumPhotosResp.Photos) != 2 {
		t.Errorf("Expected 2 album photos, got total=%d len=%d", albumPhotosResp.Total, len(albumPhotosResp.Photos))
	}
	if albumPhotosResp.Photos[0].URL == "" || albumPhotosResp.Photos[0].ThumbnailURL == "" {
		t.Errorf("Expected valid URL and ThumbnailURL on album photos: %+v", albumPhotosResp.Photos[0])
	}

	// 12. GET /api/albums/:id/photos on nonexistent album returns 404
	req = httptest.NewRequest("GET", "/api/albums/nonexistent-album/photos", nil)
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("Expected 404 for nonexistent album photos, got %d", resp.StatusCode)
	}

	// 13. PATCH /api/albums/:id (Rename album)
	renamePayload := bytes.NewReader([]byte(`{"name": "Wilderness", "description": "Updated outdoor wilderness photos"}`))
	req = httptest.NewRequest("PATCH", "/api/albums/"+createdAlbum.ID, renamePayload)
	req.Header.Set("Content-Type", "application/json")
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("Expected 200 renaming album, got %d", resp.StatusCode)
	}
	var renamed models.Album
	body, _ = io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &renamed)
	if renamed.Name != "Wilderness" || renamed.PhotoCount != 2 {
		t.Errorf("Rename mismatch: got %+v", renamed)
	}

	// 14. PATCH with empty name returns 400
	emptyRenamePayload := bytes.NewReader([]byte(`{"name": "   "}`))
	req = httptest.NewRequest("PATCH", "/api/albums/"+createdAlbum.ID, emptyRenamePayload)
	req.Header.Set("Content-Type", "application/json")
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected 400 for empty rename, got %d", resp.StatusCode)
	}

	// 15. DELETE /api/albums/:id/photos/:photoId (Remove photo from album)
	req = httptest.NewRequest("DELETE", "/api/albums/"+createdAlbum.ID+"/photos/photo-alb-1", nil)
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("Expected 200 removing photo from album, got %d", resp.StatusCode)
	}

	// Verify original photo in photos table STILL exists
	reqPhoto := httptest.NewRequest("GET", "/api/photos/photo-alb-1", nil)
	respPhoto, _ := app.Test(reqPhoto, -1)
	if respPhoto.StatusCode != fiber.StatusOK {
		t.Errorf("Removing photo from album must not delete the photo from /api/photos/:id, got status %d", respPhoto.StatusCode)
	}

	// 16. DELETE /api/albums/:id (Delete album)
	req = httptest.NewRequest("DELETE", "/api/albums/"+createdAlbum.ID, nil)
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("Expected 200 deleting album, got %d", resp.StatusCode)
	}

	// Verify album is gone
	req = httptest.NewRequest("GET", "/api/albums/"+createdAlbum.ID, nil)
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("Expected 404 for deleted album, got %d", resp.StatusCode)
	}

	// Verify original photos and storage media files are completely untouched
	reqPhoto2 := httptest.NewRequest("GET", "/api/photos/photo-alb-2", nil)
	respPhoto2, _ := app.Test(reqPhoto2, -1)
	if respPhoto2.StatusCode != fiber.StatusOK {
		t.Errorf("Deleting album must never delete original photo records, got status %d", respPhoto2.StatusCode)
	}

	if _, err := os.Stat(absPath1); os.IsNotExist(err) {
		t.Errorf("Original media file 1 was deleted from storage")
	}
	if _, err := os.Stat(absPath2); os.IsNotExist(err) {
		t.Errorf("Original media file 2 was deleted from storage")
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

	// 4. Force refresh scan: POST /api/photos/scan?sync=true&refresh=true
	reqRefresh := httptest.NewRequest("POST", "/api/photos/scan?sync=true&refresh=true", nil)
	respRefresh, err := app.Test(reqRefresh, -1)
	if err != nil || respRefresh.StatusCode != fiber.StatusOK {
		t.Fatalf("Failed to trigger refresh scan: %v", err)
	}
	refreshBody, _ := io.ReadAll(respRefresh.Body)
	var refreshStatus struct {
		Status  string 
		Updated int    
		Skipped int    
	}
	_ = json.Unmarshal(refreshBody, &refreshStatus)
	if refreshStatus.Updated != 2 {
		t.Errorf("Expected updated=2 on refresh scan, got %+v", refreshStatus)
	}
}

func TestTimelineAndPhotoViewerAPI(t *testing.T) {
	app, db, _, _, _, cleanup := setupTestApp(t)
	defer cleanup()

	photoRepo := repository.NewSQLitePhotoRepository(db)
	ctx := t.Context()

	// Seed 3 photos:
	// p1: TakenAt = 2026-10-24 14:00:00 (dated, newest)
	// p2: TakenAt = 2026-10-24 10:00:00 (dated, same day earlier)
	// p3: TakenAt = nil (undated), modtime = 2026-10-25
	t1 := time.Date(2026, 10, 24, 14, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC)
	m3 := time.Date(2026, 10, 25, 8, 0, 0, 0, time.UTC)

	_ = photoRepo.Create(ctx, &models.Photo{
		ID:               "p-time-1",
		RelativePath:     "originals/p1.jpg",
		OriginalFilename: "p1.jpg",
		MediaType:        "image/jpeg",
		FileSize:         1000,
		TakenAt:          &t1,
		FileModifiedAt:   time.Now().UTC(),
	})
	_ = photoRepo.Create(ctx, &models.Photo{
		ID:               "p-time-2",
		RelativePath:     "originals/p2.jpg",
		OriginalFilename: "p2.jpg",
		MediaType:        "image/jpeg",
		FileSize:         1000,
		TakenAt:          &t2,
		FileModifiedAt:   time.Now().UTC(),
	})
	_ = photoRepo.Create(ctx, &models.Photo{
		ID:               "p-time-3",
		RelativePath:     "originals/p3.jpg",
		OriginalFilename: "p3.jpg",
		MediaType:        "image/jpeg",
		FileSize:         1000,
		TakenAt:          nil,
		FileModifiedAt:   m3,
	})

	// 1. GET /api/timeline
	req := httptest.NewRequest("GET", "/api/timeline?limit=10&offset=0", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Timeline request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var timeResp api.TimelineResponse
	if err := json.Unmarshal(body, &timeResp); err != nil {
		t.Fatalf("Failed to unmarshal timeline response: %v", err)
	}

	if timeResp.Total != 3 || len(timeResp.Photos) != 3 {
		t.Fatalf("Expected 3 photos in timeline, got total=%d len=%d", timeResp.Total, len(timeResp.Photos))
	}

	// Verify photos list order: p-time-1, p-time-2, then p-time-3
	if timeResp.Photos[0].ID != "p-time-1" || timeResp.Photos[1].ID != "p-time-2" || timeResp.Photos[2].ID != "p-time-3" {
		t.Errorf("Unexpected photo sequence in timeline: %+v", timeResp.Photos)
	}
	if timeResp.Photos[2].IsUndated != true {
		t.Errorf("Expected p-time-3 to be marked isUndated=true")
	}
	if timeResp.Photos[0].DateSource != "exif" || timeResp.Photos[0].IsUndated != false {
		t.Errorf("Expected p-time-1 DateSource=exif IsUndated=false, got %s, %v", timeResp.Photos[0].DateSource, timeResp.Photos[0].IsUndated)
	}
	if timeResp.Photos[1].DateSource != "exif" || timeResp.Photos[1].IsUndated != false {
		t.Errorf("Expected p-time-2 DateSource=exif IsUndated=false, got %s, %v", timeResp.Photos[1].DateSource, timeResp.Photos[1].IsUndated)
	}
	if timeResp.Photos[2].DateSource != "file_modified" || timeResp.Photos[2].IsUndated != true {
		t.Errorf("Expected p-time-3 DateSource=file_modified IsUndated=true, got %s, %v", timeResp.Photos[2].DateSource, timeResp.Photos[2].IsUndated)
	}

	// Verify day grouping: exactly 2 groups (2026-10-24 with 2 photos, and undated with 1 photo)
	if len(timeResp.Groups) != 2 {
		t.Fatalf("Expected 2 groups, got %d", len(timeResp.Groups))
	}
	if timeResp.Groups[0].Date != "2026-10-24" || len(timeResp.Groups[0].Photos) != 2 {
		t.Errorf("Unexpected group 0: %+v", timeResp.Groups[0])
	}
	if timeResp.Groups[0].IsUndated != false {
		t.Errorf("Group 0 should not be undated")
	}
	if timeResp.Groups[1].Date != "undated" || len(timeResp.Groups[1].Photos) != 1 || !timeResp.Groups[1].IsUndated {
		t.Errorf("Unexpected group 1 (undated): %+v", timeResp.Groups[1])
	}

	// 2. Cursor pagination on timeline: limit=2
	reqPage1 := httptest.NewRequest("GET", "/api/timeline?limit=2&offset=0", nil)
	respPage1, _ := app.Test(reqPage1, -1)
	bodyPage1, _ := io.ReadAll(respPage1.Body)
	var timePage1 api.TimelineResponse
	_ = json.Unmarshal(bodyPage1, &timePage1)

	if !timePage1.HasMore || timePage1.NextCursor == "" {
		t.Errorf("Expected hasMore=true and nextCursor on page 1")
	}

	// Fetch page 2 using cursor
	reqPage2 := httptest.NewRequest("GET", "/api/timeline?cursor="+timePage1.NextCursor+"&limit=2", nil)
	respPage2, _ := app.Test(reqPage2, -1)
	bodyPage2, _ := io.ReadAll(respPage2.Body)
	var timePage2 api.TimelineResponse
	_ = json.Unmarshal(bodyPage2, &timePage2)

	if len(timePage2.Photos) != 1 || timePage2.Photos[0].ID != "p-time-3" {
		t.Errorf("Unexpected photos on cursor page 2: %+v", timePage2.Photos)
	}

	// 3. GET /api/photos/:id with adjacent navigation context (timeline)
	reqAdj := httptest.NewRequest("GET", "/api/photos/p-time-2?collectionType=timeline", nil)
	respAdj, _ := app.Test(reqAdj, -1)
	bodyAdj, _ := io.ReadAll(respAdj.Body)
	var pAdj api.PhotoResponse
	_ = json.Unmarshal(bodyAdj, &pAdj)

	if pAdj.PrevID == nil || *pAdj.PrevID != "p-time-1" {
		t.Errorf("Expected prevId 'p-time-1', got %v", pAdj.PrevID)
	}
	if pAdj.NextID == nil || *pAdj.NextID != "p-time-3" {
		t.Errorf("Expected nextId 'p-time-3', got %v", pAdj.NextID)
	}

	// 4. Verify relative folder path in PhotoResponse (never absolute server path)
	if pAdj.Folder != "originals" {
		t.Errorf("Expected relative folder 'originals', got %q", pAdj.Folder)
	}
}

func TestPhotoSearchAndFilteringAPI(t *testing.T) {
	app, db, _, _, _, cleanup := setupTestApp(t)
	defer cleanup()

	photoRepo := repository.NewSQLitePhotoRepository(db)
	albumRepo := repository.NewSQLiteAlbumRepository(db)
	ctx := t.Context()

	d1 := time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC)
	m3 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	_ = photoRepo.Create(ctx, &models.Photo{
		ID:               "p-alps",
		RelativePath:     "Nature/Alps_Peak.jpg",
		OriginalFilename: "Alps_Peak.jpg",
		MediaType:        "image/jpeg",
		FileSize:         250000,
		TakenAt:          &d1,
		FileModifiedAt:   d1,
	})
	_ = photoRepo.Create(ctx, &models.Photo{
		ID:               "p-lake",
		RelativePath:     "Nature/Blue_Lake.png",
		OriginalFilename: "Blue_Lake.png",
		MediaType:        "image/png",
		FileSize:         450000,
		TakenAt:          &d2,
		FileModifiedAt:   d2,
	})
	_ = photoRepo.Create(ctx, &models.Photo{
		ID:               "p-scan",
		RelativePath:     "Scans/Invoice_100%.jpg",
		OriginalFilename: "Invoice_100%.jpg",
		MediaType:        "image/jpeg",
		FileSize:         80000,
		TakenAt:          nil,
		FileModifiedAt:   m3,
	})

	album := &models.Album{
		ID:          "alb-mountains",
		Name:        "Mountain Trip",
		Description: "Photos of Alps",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	_ = albumRepo.Create(ctx, album)
	_, _ = albumRepo.AddPhotos(ctx, album.ID, []string{"p-alps"})

	// 1. Search by filename: GET /api/photos?q=alps
	req := httptest.NewRequest("GET", "/api/photos?q=alps", nil)
	resp, err := app.Test(req, -1)
	if err != nil || resp.StatusCode != fiber.StatusOK {
		t.Fatalf("Search failed: %v, status %d", err, resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var searchResp struct {
		Photos []api.PhotoResponse `json:"photos"`
		Total  int                 `json:"total"`
	}
	_ = json.Unmarshal(body, &searchResp)
	if searchResp.Total != 1 || searchResp.Photos[0].ID != "p-alps" {
		t.Errorf("Expected 1 result p-alps for q=alps, got total=%d", searchResp.Total)
	}

	// 2. Search with wildcard characters: GET /api/photos?q=100%
	req = httptest.NewRequest("GET", "/api/photos?q=100%25", nil)
	resp, _ = app.Test(req, -1)
	body, _ = io.ReadAll(resp.Body)
	var wildResp struct {
		Photos []api.PhotoResponse `json:"photos"`
		Total  int                 `json:"total"`
	}
	_ = json.Unmarshal(body, &wildResp)
	if wildResp.Total != 1 || wildResp.Photos[0].ID != "p-scan" {
		t.Errorf("Expected p-scan for wildcard search, got total=%d", wildResp.Total)
	}

	// 3. Date range filter: GET /api/photos?startDate=2026-03-01&endDate=2026-03-31
	req = httptest.NewRequest("GET", "/api/photos?startDate=2026-03-01&endDate=2026-03-31", nil)
	resp, _ = app.Test(req, -1)
	body, _ = io.ReadAll(resp.Body)
	var dateResp struct {
		Photos []api.PhotoResponse `json:"photos"`
		Total  int                 `json:"total"`
	}
	_ = json.Unmarshal(body, &dateResp)
	if dateResp.Total != 1 || dateResp.Photos[0].ID != "p-alps" {
		t.Errorf("Expected 1 photo p-alps in March 2026, got %d", dateResp.Total)
	}

	// 4. Validation error: startDate after endDate
	req = httptest.NewRequest("GET", "/api/photos?startDate=2026-05-01&endDate=2026-04-01", nil)
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected 400 for startDate > endDate, got %d", resp.StatusCode)
	}

	// 5. Validation error: invalid date format
	req = httptest.NewRequest("GET", "/api/photos?startDate=invalid-date", nil)
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected 400 for invalid date format, got %d", resp.StatusCode)
	}

	// 6. Validation error: path traversal in folder
	req = httptest.NewRequest("GET", "/api/photos?folder=../escape", nil)
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected 400 for path traversal folder, got %d", resp.StatusCode)
	}

	// 7. Validation error: invalid sort option
	req = httptest.NewRequest("GET", "/api/photos?sort=malicious_col", nil)
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected 400 for invalid sort option, got %d", resp.StatusCode)
	}

	// 8. Validation error: invalid sort order
	req = httptest.NewRequest("GET", "/api/photos?order=sideways", nil)
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected 400 for invalid sort order, got %d", resp.StatusCode)
	}

	// 9. Media type filter: GET /api/photos?mediaType=image/png
	req = httptest.NewRequest("GET", "/api/photos?mediaType=image/png", nil)
	resp, _ = app.Test(req, -1)
	body, _ = io.ReadAll(resp.Body)
	var mtResp struct {
		Photos []api.PhotoResponse `json:"photos"`
		Total  int                 `json:"total"`
	}
	_ = json.Unmarshal(body, &mtResp)
	if mtResp.Total != 1 || mtResp.Photos[0].ID != "p-lake" {
		t.Errorf("Expected p-lake for mediaType=image/png, got total=%d", mtResp.Total)
	}

	// 10. Album filter: GET /api/photos?albumId=alb-mountains
	req = httptest.NewRequest("GET", "/api/photos?albumId=alb-mountains", nil)
	resp, _ = app.Test(req, -1)
	body, _ = io.ReadAll(resp.Body)
	var albResp struct {
		Photos []api.PhotoResponse `json:"photos"`
		Total  int                 `json:"total"`
	}
	_ = json.Unmarshal(body, &albResp)
	if albResp.Total != 1 || albResp.Photos[0].ID != "p-alps" {
		t.Errorf("Expected p-alps for albumId=alb-mountains, got total=%d", albResp.Total)
	}

	// 11. Undated filter: GET /api/photos?isUndated=true
	req = httptest.NewRequest("GET", "/api/photos?isUndated=true", nil)
	resp, _ = app.Test(req, -1)
	body, _ = io.ReadAll(resp.Body)
	var undatedResp struct {
		Photos []api.PhotoResponse `json:"photos"`
		Total  int                 `json:"total"`
	}
	_ = json.Unmarshal(body, &undatedResp)
	if undatedResp.Total != 1 || undatedResp.Photos[0].ID != "p-scan" {
		t.Errorf("Expected p-scan for isUndated=true, got total=%d", undatedResp.Total)
	}

	// 12. Combined discovery: GET /api/photos?q=Nature&mediaType=image/jpeg&sort=date&order=desc
	req = httptest.NewRequest("GET", "/api/photos?q=Nature&mediaType=image/jpeg&sort=date&order=desc", nil)
	resp, _ = app.Test(req, -1)
	body, _ = io.ReadAll(resp.Body)
	var combResp struct {
		Photos []api.PhotoResponse `json:"photos"`
		Total  int                 `json:"total"`
	}
	_ = json.Unmarshal(body, &combResp)
	if combResp.Total != 1 || combResp.Photos[0].ID != "p-alps" {
		t.Errorf("Expected 1 result p-alps for combined query, got %d", combResp.Total)
	}

	// 13. Timeline with search query: GET /api/timeline?q=lake
	req = httptest.NewRequest("GET", "/api/timeline?q=lake", nil)
	resp, _ = app.Test(req, -1)
	body, _ = io.ReadAll(resp.Body)
	var tlResp api.TimelineResponse
	_ = json.Unmarshal(body, &tlResp)
	if tlResp.Total != 1 || len(tlResp.Photos) != 1 || tlResp.Photos[0].ID != "p-lake" {
		t.Errorf("Timeline search q=lake failed: total=%d", tlResp.Total)
	}
}
