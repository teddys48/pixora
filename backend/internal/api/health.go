package api

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"

	"pixora/backend/internal/database"
	"pixora/backend/internal/storage"
)

// HealthResponse represents the health check response body.
type HealthResponse struct {
	Status   string `json:"status"`
	Service  string `json:"service"`
	Version  string `json:"version"`
	Database string `json:"database,omitempty"`
	Storage  string `json:"storage,omitempty"`
}

// HealthHandler returns a default static health check handler for backward compatibility.
func HealthHandler(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(HealthResponse{
		Status:  "ok",
		Service: "pixora-api",
		Version: "0.1.0",
	})
}

// NewHealthHandler returns a health check handler that inspects database and storage connectivity.
func NewHealthHandler(db *database.DB, store *storage.Storage) fiber.Handler {
	return func(c *fiber.Ctx) error {
		resp := HealthResponse{
			Status:  "ok",
			Service: "pixora-api",
			Version: "0.1.0",
		}

		if db != nil {
			ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
			defer cancel()

			if err := db.PingContext(ctx); err != nil {
				resp.Status = "degraded"
				resp.Database = "disconnected"
			} else {
				resp.Database = "connected"
			}
		}

		if store != nil {
			if err := store.CheckReadAccess(); err != nil {
				resp.Storage = "unavailable"
			} else {
				resp.Storage = "ready"
			}
		}

		return c.Status(fiber.StatusOK).JSON(resp)
	}
}
