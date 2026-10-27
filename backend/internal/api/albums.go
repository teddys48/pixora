package api

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"pixora/backend/internal/models"
	"pixora/backend/internal/repository"
)

// CreateAlbumRequest defines the payload for creating an album.
type CreateAlbumRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ListAlbumsHandler handles GET /api/albums.
func ListAlbumsHandler(repo repository.AlbumRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if repo == nil {
			return SendError(c, fiber.StatusServiceUnavailable, "service_unavailable", "database repository unavailable")
		}
		albums, err := repo.List(c.Context())
		if err != nil {
			return SendError(c, fiber.StatusInternalServerError, "database_error", "failed to list albums")
		}
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"albums": albums,
		})
	}
}

// CreateAlbumHandler handles POST /api/albums.
func CreateAlbumHandler(repo repository.AlbumRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if repo == nil {
			return SendError(c, fiber.StatusServiceUnavailable, "service_unavailable", "database repository unavailable")
		}

		var req CreateAlbumRequest
		if err := c.BodyParser(&req); err != nil {
			return SendError(c, fiber.StatusBadRequest, "invalid_request", "invalid JSON request body")
		}

		name := strings.TrimSpace(req.Name)
		if name == "" {
			return SendError(c, fiber.StatusBadRequest, "validation_error", "album name cannot be empty")
		}
		if len(name) > 100 {
			return SendError(c, fiber.StatusBadRequest, "validation_error", "album name exceeds maximum length of 100 characters")
		}

		now := time.Now().UTC()
		album := &models.Album{
			ID:          uuid.NewString(),
			Name:        name,
			Description: strings.TrimSpace(req.Description),
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		if err := repo.Create(c.Context(), album); err != nil {
			if errors.Is(err, repository.ErrConflict) {
				return SendError(c, fiber.StatusConflict, "conflict", "album already exists")
			}
			return SendError(c, fiber.StatusInternalServerError, "database_error", "failed to create album")
		}

		return c.Status(fiber.StatusCreated).JSON(album)
	}
}

// GetAlbumHandler handles GET /api/albums/:id.
func GetAlbumHandler(repo repository.AlbumRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if repo == nil {
			return SendError(c, fiber.StatusServiceUnavailable, "service_unavailable", "database repository unavailable")
		}

		id := c.Params("id")
		if strings.TrimSpace(id) == "" {
			return SendError(c, fiber.StatusBadRequest, "validation_error", "album ID cannot be empty")
		}

		album, err := repo.GetByID(c.Context(), id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return SendError(c, fiber.StatusNotFound, "not_found", "album not found")
			}
			return SendError(c, fiber.StatusInternalServerError, "database_error", "failed to get album")
		}

		return c.Status(fiber.StatusOK).JSON(album)
	}
}
