package server_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v2"

	"pixora/backend/internal/config"
	"pixora/backend/internal/server"
)

func TestServerHealthRoute(t *testing.T) {
	cfg := &config.Config{
		Port:      "8080",
		Host:      "0.0.0.0",
		StaticDir: "./nonexistent_dir",
		Env:       "test",
	}

	app := server.New(cfg)

	req := httptest.NewRequest("GET", "/api/health", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestServerUnknownAPIRouteReturns404JSON(t *testing.T) {
	cfg := &config.Config{
		Port:      "8080",
		Host:      "0.0.0.0",
		StaticDir: "./nonexistent_dir",
		Env:       "test",
	}

	app := server.New(cfg)

	req := httptest.NewRequest("GET", "/api/photos/not-implemented", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var jsonResp map[string]string
	if err := json.Unmarshal(body, &jsonResp); err != nil {
		t.Fatalf("Expected JSON error response, got error: %v, body: %s", err, string(body))
	}

	if jsonResp["error"] != "not_found" {
		t.Errorf("Expected error 'not_found', got '%s'", jsonResp["error"])
	}
}

func TestServerStaticAndSPAFallback(t *testing.T) {
	// Create a temporary directory with index.html and a static file
	tempDir, err := os.MkdirTemp("", "pixora_static_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	indexHTML := "<html><head><title>Pixora</title></head><body><div id=\"root\"></div></body></html>"
	if err := os.WriteFile(filepath.Join(tempDir, "index.html"), []byte(indexHTML), 0644); err != nil {
		t.Fatalf("Failed to write index.html: %v", err)
	}

	sampleCSS := "body { margin: 0; }"
	if err := os.WriteFile(filepath.Join(tempDir, "style.css"), []byte(sampleCSS), 0644); err != nil {
		t.Fatalf("Failed to write style.css: %v", err)
	}

	cfg := &config.Config{
		Port:      "8080",
		Host:      "0.0.0.0",
		StaticDir: tempDir,
		Env:       "test",
	}

	app := server.New(cfg)

	// 1. Existing static file
	reqCSS := httptest.NewRequest("GET", "/style.css", nil)
	respCSS, err := app.Test(reqCSS, -1)
	if err != nil {
		t.Fatalf("Failed to test static asset: %v", err)
	}
	if respCSS.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200 for existing static file, got %d", respCSS.StatusCode)
	}

	// 2. SPA client-side route fallback to index.html
	reqSPA := httptest.NewRequest("GET", "/photos", nil)
	respSPA, err := app.Test(reqSPA, -1)
	if err != nil {
		t.Fatalf("Failed to test SPA route: %v", err)
	}
	if respSPA.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200 for SPA route, got %d", respSPA.StatusCode)
	}
	bodySPA, _ := io.ReadAll(respSPA.Body)
	if string(bodySPA) != indexHTML {
		t.Errorf("Expected index.html content for SPA route, got: %s", string(bodySPA))
	}

	// 3. Missing static asset with extension should return 404, not index.html
	reqMissingAsset := httptest.NewRequest("GET", "/assets/bundle-missing.js", nil)
	respMissingAsset, err := app.Test(reqMissingAsset, -1)
	if err != nil {
		t.Fatalf("Failed to test missing asset: %v", err)
	}
	if respMissingAsset.StatusCode != fiber.StatusNotFound {
		t.Errorf("Expected status 404 for missing static asset, got %d", respMissingAsset.StatusCode)
	}
}
