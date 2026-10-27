package api

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"pixora/backend/internal/repository"
)

// ListPhotosHandler handles GET /api/photos with pagination and sorting.
func ListPhotosHandler(repo repository.PhotoRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if repo == nil {
			return SendError(c, fiber.StatusServiceUnavailable, "service_unavailable", "database repository unavailable")
		}

		limit := 50
		if l := c.Query("limit"); l != "" {
			parsed, err := strconv.Atoi(l)
			if err != nil || parsed < 1 {
				return SendError(c, fiber.StatusBadRequest, "validation_error", "limit must be a positive integer")
			}
			if parsed > 100 {
				parsed = 100
			}
			limit = parsed
		}

		offset := 0
		if o := c.Query("offset"); o != "" {
			parsed, err := strconv.Atoi(o)
			if err != nil || parsed < 0 {
				return SendError(c, fiber.StatusBadRequest, "validation_error", "offset must be a non-negative integer")
			}
			offset = parsed
		}

		sortBy := c.Query("sort", "date")
		order := c.Query("order", "desc")

		photos, total, err := repo.List(c.Context(), offset, limit, sortBy, order)
		if err != nil {
			return SendError(c, fiber.StatusInternalServerError, "database_error", "failed to list photos")
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"photos": photos,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
	}
}

// GetPhotoHandler handles GET /api/photos/:id.
func GetPhotoHandler(repo repository.PhotoRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if repo == nil {
			return SendError(c, fiber.StatusServiceUnavailable, "service_unavailable", "database repository unavailable")
		}

		id := c.Params("id")
		if strings.TrimSpace(id) == "" {
			return SendError(c, fiber.StatusBadRequest, "validation_error", "photo ID cannot be empty")
		}

		photo, err := repo.GetByID(c.Context(), id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return SendError(c, fiber.StatusNotFound, "not_found", "photo not found")
			}
			return SendError(c, fiber.StatusInternalServerError, "database_error", "failed to get photo")
		}

		return c.Status(fiber.StatusOK).JSON(photo)
	}
}
