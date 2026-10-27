package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v2"

	"pixora/backend/internal/api"
	"pixora/backend/internal/database"
	"pixora/backend/internal/models"
	"pixora/backend/internal/repository"
)

func setupTestApp(t *testing.T) (*fiber.App, *database.DB, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "pixora_api_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	db, err := database.Open(filepath.Join(tempDir, "api_test.db"))
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to open test db: %v", err)
	}

	photoRepo := repository.NewSQLitePhotoRepository(db)
	albumRepo := repository.NewSQLiteAlbumRepository(db)

	app := fiber.New()
	apiGroup := app.Group("/api")
	apiGroup.Get("/albums", api.ListAlbumsHandler(albumRepo))
	apiGroup.Post("/albums", api.CreateAlbumHandler(albumRepo))
	apiGroup.Get("/albums/:id", api.GetAlbumHandler(albumRepo))
	apiGroup.Get("/photos", api.ListPhotosHandler(photoRepo))
	apiGroup.Get("/photos/:id", api.GetPhotoHandler(photoRepo))

	cleanup := func() {
		_ = db.Close()
		_ = os.RemoveAll(tempDir)
	}

	return app, db, cleanup
}

func TestAlbumsAPI(t *testing.T) {
	app, _, cleanup := setupTestApp(t)
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
	app, db, cleanup := setupTestApp(t)
	defer cleanup()

	photoRepo := repository.NewSQLitePhotoRepository(db)
	ctx := t.Context()

	photo := &models.Photo{
		ID:               "p-api-1",
		RelativePath:     "originals/api_photo.jpg",
		OriginalFilename: "api_photo.jpg",
		MediaType:        "image/jpeg",
		FileSize:         50000,
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
		Photos []models.Photo `json:"photos"`
		Total  int            `json:"total"`
		Limit  int            `json:"limit"`
		Offset int            `json:"offset"`
	}
	_ = json.Unmarshal(body, &photoListResp)
	if photoListResp.Total != 1 || len(photoListResp.Photos) != 1 {
		t.Errorf("Unexpected photos list: %+v", photoListResp)
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
}
