package api_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v2"

	"pixora/backend/internal/api"
	"pixora/backend/internal/database"
	"pixora/backend/internal/storage"
)

func TestHealthHandler(t *testing.T) {
	app := fiber.New()
	app.Get("/api/health", api.HealthHandler)

	req := httptest.NewRequest("GET", "/api/health", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var healthResp api.HealthResponse
	if err := json.Unmarshal(body, &healthResp); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if healthResp.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", healthResp.Status)
	}

	if healthResp.Service != "pixora-api" {
		t.Errorf("Expected service 'pixora-api', got '%s'", healthResp.Service)
	}

	if healthResp.Version == "" {
		t.Errorf("Expected non-empty version, got '%s'", healthResp.Version)
	}
}

func TestNewHealthHandlerWithDBAndStorage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pixora_health_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	db, err := database.Open(filepath.Join(tempDir, "health.db"))
	if err != nil {
		t.Fatalf("Failed to open test db: %v", err)
	}
	defer db.Close()

	store, err := storage.New(tempDir)
	if err != nil {
		t.Fatalf("Failed to init storage: %v", err)
	}

	app := fiber.New()
	app.Get("/api/health", api.NewHealthHandler(db, store))

	req := httptest.NewRequest("GET", "/api/health", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var healthResp api.HealthResponse
	if err := json.Unmarshal(body, &healthResp); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if healthResp.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", healthResp.Status)
	}
	if healthResp.Database != "connected" {
		t.Errorf("Expected database 'connected', got '%s'", healthResp.Database)
	}
	if healthResp.Storage != "ready" {
		t.Errorf("Expected storage 'ready', got '%s'", healthResp.Storage)
	}
}
