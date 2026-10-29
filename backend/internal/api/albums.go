package api

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
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

// UpdateAlbumRequest defines the payload for updating/renaming an album.
type UpdateAlbumRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

// AddPhotosRequest defines the payload for adding photos to an album.
type AddPhotosRequest struct {
	PhotoIDs []string `json:"photoIds"`
	PhotoID  string   `json:"photoId"`
}

// ListAlbumsHandler handles GET /api/albums with optional search and sorting.
func ListAlbumsHandler(repo repository.AlbumRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if repo == nil {
			return SendError(c, fiber.StatusServiceUnavailable, "service_unavailable", "database repository unavailable")
		}

		albums, err := repo.List(c.Context())
		if err != nil {
			return SendError(c, fiber.StatusInternalServerError, "database_error", "failed to list albums")
		}

		// Optional client search filter
		search := strings.TrimSpace(strings.ToLower(c.Query("search", "")))
		if search != "" {
			filtered := make([]models.Album, 0, len(albums))
			for _, a := range albums {
				if strings.Contains(strings.ToLower(a.Name), search) || strings.Contains(strings.ToLower(a.Description), search) {
					filtered = append(filtered, a)
				}
			}
			albums = filtered
		}

		// Optional sort
		sortBy := strings.ToLower(c.Query("sort", "name"))
		order := strings.ToLower(c.Query("order", "asc"))

		switch sortBy {
		case "photos", "count":
			sort.SliceStable(albums, func(i, j int) bool {
				if order == "desc" {
					return albums[i].PhotoCount > albums[j].PhotoCount
				}
				return albums[i].PhotoCount < albums[j].PhotoCount
			})
		case "updated":
			sort.SliceStable(albums, func(i, j int) bool {
				if order == "desc" {
					return albums[i].UpdatedAt.After(albums[j].UpdatedAt)
				}
				return albums[i].UpdatedAt.Before(albums[j].UpdatedAt)
			})
		case "created":
			sort.SliceStable(albums, func(i, j int) bool {
				if order == "desc" {
					return albums[i].CreatedAt.After(albums[j].CreatedAt)
				}
				return albums[i].CreatedAt.Before(albums[j].CreatedAt)
			})
		case "name":
			if order == "desc" {
				sort.SliceStable(albums, func(i, j int) bool {
					return strings.ToLower(albums[i].Name) > strings.ToLower(albums[j].Name)
				})
			}
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
				return SendError(c, fiber.StatusConflict, "conflict", "an album with this name already exists")
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

		id := strings.TrimSpace(c.Params("id"))
		if id == "" {
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

// UpdateAlbumHandler handles PATCH /api/albums/:id to rename or update an album.
func UpdateAlbumHandler(repo repository.AlbumRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if repo == nil {
			return SendError(c, fiber.StatusServiceUnavailable, "service_unavailable", "database repository unavailable")
		}

		id := strings.TrimSpace(c.Params("id"))
		if id == "" {
			return SendError(c, fiber.StatusBadRequest, "validation_error", "album ID cannot be empty")
		}

		album, err := repo.GetByID(c.Context(), id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return SendError(c, fiber.StatusNotFound, "not_found", "album not found")
			}
			return SendError(c, fiber.StatusInternalServerError, "database_error", "failed to get album")
		}

		var req UpdateAlbumRequest
		if err := c.BodyParser(&req); err != nil {
			return SendError(c, fiber.StatusBadRequest, "invalid_request", "invalid JSON request body")
		}

		if req.Name != nil {
			trimmedName := strings.TrimSpace(*req.Name)
			if trimmedName == "" {
				return SendError(c, fiber.StatusBadRequest, "validation_error", "album name cannot be empty")
			}
			if len(trimmedName) > 100 {
				return SendError(c, fiber.StatusBadRequest, "validation_error", "album name exceeds maximum length of 100 characters")
			}
			album.Name = trimmedName
		}

		if req.Description != nil {
			album.Description = strings.TrimSpace(*req.Description)
		}

		if err := repo.Update(c.Context(), album); err != nil {
			if errors.Is(err, repository.ErrConflict) {
				return SendError(c, fiber.StatusConflict, "conflict", "an album with this name already exists")
			}
			if errors.Is(err, repository.ErrNotFound) {
				return SendError(c, fiber.StatusNotFound, "not_found", "album not found")
			}
			return SendError(c, fiber.StatusInternalServerError, "database_error", "failed to update album")
		}

		// Re-fetch to return fresh photoCount and cover thumbnail
		updatedAlbum, err := repo.GetByID(c.Context(), id)
		if err != nil {
			return c.Status(fiber.StatusOK).JSON(album)
		}

		return c.Status(fiber.StatusOK).JSON(updatedAlbum)
	}
}

// DeleteAlbumHandler handles DELETE /api/albums/:id.
// Removes the album and photo associations without touching the original media or photos table.
func DeleteAlbumHandler(repo repository.AlbumRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if repo == nil {
			return SendError(c, fiber.StatusServiceUnavailable, "service_unavailable", "database repository unavailable")
		}

		id := strings.TrimSpace(c.Params("id"))
		if id == "" {
			return SendError(c, fiber.StatusBadRequest, "validation_error", "album ID cannot be empty")
		}

		if err := repo.Delete(c.Context(), id); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return SendError(c, fiber.StatusNotFound, "not_found", "album not found")
			}
			return SendError(c, fiber.StatusInternalServerError, "database_error", "failed to delete album")
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"message": "album deleted successfully",
		})
	}
}

// GetAlbumPhotosHandler handles GET /api/albums/:id/photos with cursor-based pagination.
func GetAlbumPhotosHandler(repo repository.AlbumRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if repo == nil {
			return SendError(c, fiber.StatusServiceUnavailable, "service_unavailable", "database repository unavailable")
		}

		id := strings.TrimSpace(c.Params("id"))
		if id == "" {
			return SendError(c, fiber.StatusBadRequest, "validation_error", "album ID cannot be empty")
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

		scope := ComputeAlbumScope(id)
		var cursor *repository.CursorData
		if cursorStr := c.Query("cursor"); cursorStr != "" {
			cData, err := DecodeCursor(cursorStr, scope)
			if err != nil {
				return SendError(c, fiber.StatusBadRequest, "validation_error", fmt.Sprintf("invalid cursor: %v", err))
			}
			cursor = cData
		}

		var photos []models.Photo
		var total int
		var hasMore bool
		var nextCursorData *repository.CursorData
		var err error

		if cursor != nil || (c.Query("offset") == "" && c.Query("cursor") == "") {
			photos, total, hasMore, nextCursorData, err = repo.GetPhotosCursor(c.Context(), id, limit, cursor)
		} else {
			photos, total, err = repo.GetPhotos(c.Context(), id, offset, limit)
			hasMore = (offset + len(photos)) < total
		}

		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return SendError(c, fiber.StatusNotFound, "not_found", "album not found")
			}
			return SendError(c, fiber.StatusInternalServerError, "database_error", "failed to get album photos")
		}

		responses := make([]PhotoResponse, 0, len(photos))
		for _, p := range photos {
			responses = append(responses, ToPhotoResponse(p))
		}

		var nextCursor *string
		if hasMore && nextCursorData != nil {
			token, err := EncodeCursor(nextCursorData, scope)
			if err == nil {
				nextCursor = &token
			}
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"photos":     responses,
			"items":      responses,
			"total":      total,
			"limit":      limit,
			"offset":     offset,
			"nextCursor": nextCursor,
			"hasMore":    hasMore,
		})
	}
}

// AddPhotosToAlbumHandler handles POST /api/albums/:id/photos.
// Associates one or more existing photo IDs with an album.
func AddPhotosToAlbumHandler(repo repository.AlbumRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if repo == nil {
			return SendError(c, fiber.StatusServiceUnavailable, "service_unavailable", "database repository unavailable")
		}

		id := strings.TrimSpace(c.Params("id"))
		if id == "" {
			return SendError(c, fiber.StatusBadRequest, "validation_error", "album ID cannot be empty")
		}

		var req AddPhotosRequest
		if err := c.BodyParser(&req); err != nil {
			return SendError(c, fiber.StatusBadRequest, "invalid_request", "invalid JSON request body")
		}

		photoIDs := make([]string, 0, len(req.PhotoIDs)+1)
		for _, pid := range req.PhotoIDs {
			trimmed := strings.TrimSpace(pid)
			if trimmed != "" {
				photoIDs = append(photoIDs, trimmed)
			}
		}
		if req.PhotoID != "" {
			trimmed := strings.TrimSpace(req.PhotoID)
			if trimmed != "" {
				photoIDs = append(photoIDs, trimmed)
			}
		}

		if len(photoIDs) == 0 {
			return SendError(c, fiber.StatusBadRequest, "validation_error", "at least one photo ID must be provided")
		}

		addedCount, err := repo.AddPhotos(c.Context(), id, photoIDs)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return SendError(c, fiber.StatusNotFound, "not_found", err.Error())
			}
			return SendError(c, fiber.StatusInternalServerError, "database_error", "failed to add photos to album")
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"message": "photos added to album successfully",
			"added":   addedCount,
		})
	}
}

// RemovePhotoFromAlbumHandler handles DELETE /api/albums/:id/photos/:photoId.
// Removes the association without deleting the original photo from SQLite or disk.
func RemovePhotoFromAlbumHandler(repo repository.AlbumRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if repo == nil {
			return SendError(c, fiber.StatusServiceUnavailable, "service_unavailable", "database repository unavailable")
		}

		id := strings.TrimSpace(c.Params("id"))
		if id == "" {
			return SendError(c, fiber.StatusBadRequest, "validation_error", "album ID cannot be empty")
		}

		photoID := strings.TrimSpace(c.Params("photoId"))
		if photoID == "" {
			return SendError(c, fiber.StatusBadRequest, "validation_error", "photo ID cannot be empty")
		}

		if err := repo.RemovePhoto(c.Context(), photoID, id); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return SendError(c, fiber.StatusNotFound, "not_found", "photo or album not found")
			}
			return SendError(c, fiber.StatusInternalServerError, "database_error", "failed to remove photo from album")
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"message": "photo removed from album successfully",
		})
	}
}
