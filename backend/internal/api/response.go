package api

import (
	"github.com/gofiber/fiber/v2"
)

// ErrorResponse represents a standardized JSON error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// SendError sends a structured JSON error response with an HTTP status code.
func SendError(c *fiber.Ctx, statusCode int, errorCode string, message string) error {
	return c.Status(statusCode).JSON(ErrorResponse{
		Error:   errorCode,
		Message: message,
	})
}
