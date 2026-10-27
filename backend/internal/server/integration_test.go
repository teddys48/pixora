package server_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"pixora/backend/internal/api"
	"pixora/backend/internal/config"
	"pixora/backend/internal/server"
)

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
	reqUnknownAPI := httptest.NewRequest("GET", "/api/v1/photos/upload", nil)
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
