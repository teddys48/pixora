package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"pixora/backend/internal/media"
	"pixora/backend/internal/models"
	"pixora/backend/internal/repository"
	"pixora/backend/internal/scanner"
	"pixora/backend/internal/storage"
)

const (
	// MaxUploadFileSize is 25MB per file.
	MaxUploadFileSize = 25 * 1024 * 1024

	// MaxUploadFilesCount is the maximum number of files in a single upload request.
	MaxUploadFilesCount = 20
)

// PhotoResponse represents a photo formatted for client consumption.
type PhotoResponse struct {
	ID               string     `json:"id"`
	RelativePath     string     `json:"relativePath"`
	OriginalFilename string     `json:"originalFilename"`
	MediaType        string     `json:"mediaType"`
	FileSize         int64      `json:"fileSize"`
	Width            *int       `json:"width,omitempty"`
	Height           *int       `json:"height,omitempty"`
	TakenAt          *time.Time `json:"takenAt,omitempty"`
	FileModifiedAt   time.Time  `json:"fileModifiedAt"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	URL              string     `json:"url"`
	ThumbnailURL     string     `json:"thumbnailUrl"`
}

// ToPhotoResponse converts a models.Photo into a PhotoResponse with API URLs.
func ToPhotoResponse(p models.Photo) PhotoResponse {
	return PhotoResponse{
		ID:               p.ID,
		RelativePath:     p.RelativePath,
		OriginalFilename: p.OriginalFilename,
		MediaType:        p.MediaType,
		FileSize:         p.FileSize,
		Width:            p.Width,
		Height:           p.Height,
		TakenAt:          p.TakenAt,
		FileModifiedAt:   p.FileModifiedAt,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
		URL:              fmt.Sprintf("/api/photos/%s/file", p.ID),
		ThumbnailURL:     fmt.Sprintf("/api/photos/%s/thumbnail", p.ID),
	}
}

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

		responseList := make([]PhotoResponse, len(photos))
		for i, p := range photos {
			responseList[i] = ToPhotoResponse(p)
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"photos": responseList,
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

		id := strings.TrimSpace(c.Params("id"))
		if id == "" {
			return SendError(c, fiber.StatusBadRequest, "validation_error", "photo ID cannot be empty")
		}

		photo, err := repo.GetByID(c.Context(), id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return SendError(c, fiber.StatusNotFound, "not_found", "photo not found")
			}
			return SendError(c, fiber.StatusInternalServerError, "database_error", "failed to get photo")
		}

		return c.Status(fiber.StatusOK).JSON(ToPhotoResponse(*photo))
	}
}

// ServeOriginalPhotoHandler handles GET /api/photos/:id/file securely.
func ServeOriginalPhotoHandler(repo repository.PhotoRepository, store *storage.Storage) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if repo == nil || store == nil {
			return SendError(c, fiber.StatusServiceUnavailable, "service_unavailable", "storage or repository unavailable")
		}

		id := strings.TrimSpace(c.Params("id"))
		if id == "" {
			return SendError(c, fiber.StatusBadRequest, "validation_error", "photo ID cannot be empty")
		}

		photo, err := repo.GetByID(c.Context(), id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return SendError(c, fiber.StatusNotFound, "not_found", "photo not found")
			}
			return SendError(c, fiber.StatusInternalServerError, "database_error", "failed to retrieve photo record")
		}

		resolvedPath, err := store.ResolvePath(photo.RelativePath)
		if err != nil {
			return SendError(c, fiber.StatusForbidden, "invalid_path", "security violation: path traversal detected")
		}

		info, err := os.Stat(resolvedPath)
		if err != nil {
			if os.IsNotExist(err) {
				return SendError(c, fiber.StatusNotFound, "file_missing", "original image file not found on disk")
			}
			return SendError(c, fiber.StatusInternalServerError, "filesystem_error", "failed to access image file")
		}

		// Security and caching headers
		c.Set("Content-Type", photo.MediaType)
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("Cache-Control", "public, max-age=86400")

		etag := fmt.Sprintf("\"%s-%d-%d\"", photo.ID, info.Size(), info.ModTime().Unix())
		c.Set("ETag", etag)
		c.Set("Last-Modified", info.ModTime().UTC().Format(time.RFC1123))

		if c.Get("If-None-Match") == etag {
			return c.SendStatus(fiber.StatusNotModified)
		}

		return c.SendFile(resolvedPath)
	}
}

// ServeThumbnailHandler handles GET /api/photos/:id/thumbnail.
func ServeThumbnailHandler(repo repository.PhotoRepository, store *storage.Storage, thumbSvc *media.ThumbnailService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if repo == nil || store == nil {
			return SendError(c, fiber.StatusServiceUnavailable, "service_unavailable", "storage or repository unavailable")
		}

		id := strings.TrimSpace(c.Params("id"))
		if id == "" {
			return SendError(c, fiber.StatusBadRequest, "validation_error", "photo ID cannot be empty")
		}

		photo, err := repo.GetByID(c.Context(), id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return SendError(c, fiber.StatusNotFound, "not_found", "photo not found")
			}
			return SendError(c, fiber.StatusInternalServerError, "database_error", "failed to retrieve photo record")
		}

		// If thumbnail service is available, get or generate thumbnail
		var thumbAbsPath string
		if thumbSvc != nil {
			path, err := thumbSvc.GetOrGenerate(c.Context(), photo)
			if err == nil {
				thumbAbsPath = path
			} else if errors.Is(err, storage.ErrReadOnly) {
				// Read-only storage fallback: serve original photo directly
				resolvedOrig, resErr := store.ResolvePath(photo.RelativePath)
				if resErr == nil {
					if _, statErr := os.Stat(resolvedOrig); statErr == nil {
						c.Set("Content-Type", photo.MediaType)
						c.Set("X-Content-Type-Options", "nosniff")
						c.Set("Cache-Control", "public, max-age=86400")
						return c.SendFile(resolvedOrig)
					}
				}
			}
		}

		// If no thumbnail generated, fallback to original
		if thumbAbsPath == "" {
			resolvedOrig, err := store.ResolvePath(photo.RelativePath)
			if err != nil {
				return SendError(c, fiber.StatusNotFound, "not_found", "image file not found")
			}
			thumbAbsPath = resolvedOrig
		}

		info, err := os.Stat(thumbAbsPath)
		if err != nil {
			return SendError(c, fiber.StatusNotFound, "file_missing", "thumbnail not available")
		}

		c.Set("Content-Type", "image/jpeg")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("Cache-Control", "public, max-age=604800")

		etag := fmt.Sprintf("\"thumb-%s-%d-%d\"", photo.ID, info.Size(), info.ModTime().Unix())
		c.Set("ETag", etag)
		c.Set("Last-Modified", info.ModTime().UTC().Format(time.RFC1123))

		if c.Get("If-None-Match") == etag {
			return c.SendStatus(fiber.StatusNotModified)
		}

		return c.SendFile(thumbAbsPath)
	}
}

// UploadPhotosHandler handles POST /api/photos and POST /api/photos/upload.
func UploadPhotosHandler(repo repository.PhotoRepository, store *storage.Storage, thumbSvc *media.ThumbnailService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if repo == nil || store == nil {
			return SendError(c, fiber.StatusServiceUnavailable, "service_unavailable", "storage or repository unavailable")
		}

		// Verify write access to storage root
		if err := store.CheckWriteAccess(); err != nil {
			if errors.Is(err, storage.ErrReadOnly) {
				return SendError(c, fiber.StatusForbidden, "storage_readonly", "storage is mounted read-only; uploading photos is disabled")
			}
			return SendError(c, fiber.StatusInternalServerError, "storage_error", "failed to verify storage write permissions")
		}

		form, err := c.MultipartForm()
		if err != nil {
			return SendError(c, fiber.StatusBadRequest, "invalid_form", "failed to parse multipart form")
		}

		var fileHeaders []*multipart.FileHeader
		for _, key := range []string{"photos", "file", "files"} {
			if headers, ok := form.File[key]; ok {
				fileHeaders = append(fileHeaders, headers...)
			}
		}
		if len(fileHeaders) == 0 {
			for _, headers := range form.File {
				fileHeaders = append(fileHeaders, headers...)
			}
		}

		if len(fileHeaders) == 0 {
			return SendError(c, fiber.StatusBadRequest, "no_files", "no files provided in upload request")
		}

		if len(fileHeaders) > MaxUploadFilesCount {
			return SendError(c, fiber.StatusBadRequest, "limit_exceeded", fmt.Sprintf("maximum %d files allowed per upload request", MaxUploadFilesCount))
		}

		// Ensure tmp and originals directories exist
		if err := os.MkdirAll(store.TmpDir(), 0755); err != nil {
			return SendError(c, fiber.StatusInternalServerError, "storage_error", "failed to create temporary upload directory")
		}
		originalsDir := filepath.Join(store.Root(), "originals")
		if err := os.MkdirAll(originalsDir, 0755); err != nil {
			return SendError(c, fiber.StatusInternalServerError, "storage_error", "failed to create originals directory")
		}

		type FileError struct {
			Filename string `json:"filename"`
			Error    string `json:"error"`
		}

		var createdPhotos []PhotoResponse
		var uploadErrors []FileError

		for _, fh := range fileHeaders {
			origName := filepath.Base(fh.Filename)

			// Basic extension check before reading
			if !media.IsSupportedExtension(origName) {
				uploadErrors = append(uploadErrors, FileError{
					Filename: origName,
					Error:    "unsupported file format: must be JPEG, PNG, WebP, GIF, or TIFF",
				})
				continue
			}

			// Open uploaded file
			file, err := fh.Open()
			if err != nil {
				uploadErrors = append(uploadErrors, FileError{
					Filename: origName,
					Error:    "failed to open uploaded file",
				})
				continue
			}

			// Stream to temporary file under tmp/
			tmpFilePath := filepath.Join(store.TmpDir(), fmt.Sprintf("upload_%s.tmp", uuid.New().String()))
			tmpFile, err := os.OpenFile(tmpFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
			if err != nil {
				_ = file.Close()
				uploadErrors = append(uploadErrors, FileError{
					Filename: origName,
					Error:    "failed to create temporary file for upload",
				})
				continue
			}

			// Read with limit (MaxUploadFileSize + 1 byte) to prevent excessive disk usage
			limitReader := io.LimitReader(file, MaxUploadFileSize+1)
			written, copyErr := io.Copy(tmpFile, limitReader)
			_ = tmpFile.Close()
			_ = file.Close()

			if copyErr != nil {
				_ = os.Remove(tmpFilePath)
				uploadErrors = append(uploadErrors, FileError{
					Filename: origName,
					Error:    "failed to write upload content",
				})
				continue
			}

			if written > MaxUploadFileSize {
				_ = os.Remove(tmpFilePath)
				uploadErrors = append(uploadErrors, FileError{
					Filename: origName,
					Error:    fmt.Sprintf("file exceeds maximum allowed size of %d MB", MaxUploadFileSize/(1024*1024)),
				})
				continue
			}

			// Validate image content and extract metadata
			tmpReader, err := os.Open(tmpFilePath)
			if err != nil {
				_ = os.Remove(tmpFilePath)
				uploadErrors = append(uploadErrors, FileError{
					Filename: origName,
					Error:    "failed to inspect uploaded file",
				})
				continue
			}

			meta, extractErr := media.ExtractMetadata(tmpReader)
			_ = tmpReader.Close()
			if extractErr != nil {
				_ = os.Remove(tmpFilePath)
				uploadErrors = append(uploadErrors, FileError{
					Filename: origName,
					Error:    fmt.Sprintf("invalid image content: %v", extractErr),
				})
				continue
			}

			// Assign stable photo UUID and final destination under originals/
			photoID := uuid.New().String()
			relPath := filepath.ToSlash(filepath.Join("originals", photoID+meta.Extension))
			finalPath, err := store.ResolvePath(relPath)
			if err != nil {
				_ = os.Remove(tmpFilePath)
				uploadErrors = append(uploadErrors, FileError{
					Filename: origName,
					Error:    "failed to resolve destination path",
				})
				continue
			}

			// Atomically move from tmp to final path
			if err := os.Rename(tmpFilePath, finalPath); err != nil {
				// If rename fails (e.g. cross-volume), copy and delete
				if cpErr := copyFile(tmpFilePath, finalPath); cpErr != nil {
					_ = os.Remove(tmpFilePath)
					uploadErrors = append(uploadErrors, FileError{
						Filename: origName,
						Error:    "failed to store image file",
					})
					continue
				}
				_ = os.Remove(tmpFilePath)
			}

			now := time.Now().UTC()
			var wPtr, hPtr *int
			if meta.Width > 0 && meta.Height > 0 {
				wPtr = &meta.Width
				hPtr = &meta.Height
			}

			photoModel := &models.Photo{
				ID:               photoID,
				RelativePath:     relPath,
				OriginalFilename: origName,
				MediaType:        meta.MediaType,
				FileSize:         written,
				Width:            wPtr,
				Height:           hPtr,
				TakenAt:          meta.TakenAt,
				FileModifiedAt:   now,
				CreatedAt:        now,
				UpdatedAt:        now,
			}

			if err := repo.Create(c.Context(), photoModel); err != nil {
				_ = os.Remove(finalPath)
				uploadErrors = append(uploadErrors, FileError{
					Filename: origName,
					Error:    "failed to persist photo metadata",
				})
				continue
			}

			// Generate thumbnail proactively
			if thumbSvc != nil {
				_, _ = thumbSvc.GetOrGenerate(c.Context(), photoModel)
			}

			createdPhotos = append(createdPhotos, ToPhotoResponse(*photoModel))
		}

		if len(createdPhotos) == 0 {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error":   "upload_failed",
				"message": "all uploaded files failed validation",
				"errors":  uploadErrors,
			})
		}

		statusCode := fiber.StatusCreated
		if len(uploadErrors) > 0 {
			statusCode = fiber.StatusMultiStatus
		}

		return c.Status(statusCode).JSON(fiber.Map{
			"photos": createdPhotos,
			"errors": uploadErrors,
		})
	}
}

// ScanPhotosHandler handles POST /api/photos/scan.
func ScanPhotosHandler(s *scanner.Scanner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if s == nil {
			return SendError(c, fiber.StatusServiceUnavailable, "service_unavailable", "media scanner unavailable")
		}

		isSync := c.Query("sync") == "true"

		if isSync {
			status, err := s.Scan(c.Context())
			if err != nil {
				if errors.Is(err, scanner.ErrScanInProgress) {
					return c.Status(fiber.StatusConflict).JSON(status)
				}
				return c.Status(fiber.StatusInternalServerError).JSON(status)
			}
			return c.Status(fiber.StatusOK).JSON(status)
		}

		// Asynchronous scan initiation
		cur := s.Status()
		if cur.Status == "running" {
			return c.Status(fiber.StatusConflict).JSON(cur)
		}

		go func() {
			_, _ = s.Scan(context.Background())
		}()

		// Small delay to allow the scanner goroutine to update status
		time.Sleep(15 * time.Millisecond)

		return c.Status(fiber.StatusAccepted).JSON(s.Status())
	}
}

// GetScanStatusHandler handles GET /api/photos/scan and GET /api/photos/scan/status.
func GetScanStatusHandler(s *scanner.Scanner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if s == nil {
			return SendError(c, fiber.StatusServiceUnavailable, "service_unavailable", "media scanner unavailable")
		}
		return c.Status(fiber.StatusOK).JSON(s.Status())
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
