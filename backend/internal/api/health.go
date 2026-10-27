package api

import (
	"github.com/gofiber/fiber/v2"
)

// HealthResponse represents the health check response body
type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version"`
}

// HealthHandler handles the GET /api/health endpoint
func HealthHandler(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(HealthResponse{
		Status:  "ok",
		Service: "pixora-api",
		Version: "0.1.0",
	})
}
