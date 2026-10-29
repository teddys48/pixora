package api

import (
	"context"
	"encoding/json"
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

// DateSource indicates the source and credibility of a photo's date.
type DateSource string

const (
	// DateSourceExif indicates the timestamp was extracted from verified EXIF capture metadata.
	DateSourceExif DateSource = "exif"
	// DateSourceFileModified indicates the timestamp is a filesystem modification fallback.
	DateSourceFileModified DateSource = "file_modified"
	// DateSourceNone indicates no reliable date is available.
	DateSourceNone DateSource = "none"
)

// PhotoResponse represents a photo formatted for client consumption.
type PhotoResponse struct {
	ID               string             `json:"id"`
	RelativePath     string             `json:"relativePath"`
	Folder           string             `json:"folder"` // relative physical folder path e.g. "Pictures/vacation" or ""
	OriginalFilename string             `json:"originalFilename"`
	MediaType        string             `json:"mediaType"`
	FileSize         int64              `json:"fileSize"`
	Width            *int               `json:"width,omitempty"`
	Height           *int               `json:"height,omitempty"`
	TakenAt          *time.Time         `json:"takenAt,omitempty"`
	FileModifiedAt   time.Time          `json:"fileModifiedAt"`
	CreatedAt        time.Time          `json:"createdAt"`
	UpdatedAt        time.Time          `json:"updatedAt"`
	URL              string             `json:"url"`
	ThumbnailURL     string             `json:"thumbnailUrl"`
	IsUndated        bool               `json:"isUndated"`
	DateSource       string             `json:"dateSource"` // "exif", "file_modified", "none"
	Exif             *media.ExifDetails `json:"exif,omitempty"`
	PrevID           *string            `json:"prevId,omitempty"`
	NextID           *string            `json:"nextId,omitempty"`
}

// TimelineGroupResponse groups photos by day or an undated fallback group.
type TimelineGroupResponse struct {
	Date      string          `json:"date"`  // "2026-10-24" or "undated"
	Label     string          `json:"label"` // "Monday, October 24, 2026" or "Undated Photos"
	Year      int             `json:"year,omitempty"`
	Month     string          `json:"month,omitempty"`
	Day       int             `json:"day,omitempty"`
	IsUndated bool            `json:"isUndated"`
	Photos    []PhotoResponse `json:"photos"`
}

// TimelineResponse represents the response for GET /api/timeline.
type TimelineResponse struct {
	Photos     []PhotoResponse         `json:"photos"`
	Items      []PhotoResponse         `json:"items"`
	Groups     []TimelineGroupResponse `json:"groups"`
	Total      int                     `json:"total"`
	Limit      int                     `json:"limit"`
	Offset     int                     `json:"offset"`
	NextCursor string                  `json:"nextCursor"`
	HasMore    bool                    `json:"hasMore"`
}

func (r TimelineResponse) MarshalJSON() ([]byte, error) {
	type Alias TimelineResponse
	aux := struct {
		Alias
		NextCursor *string `json:"nextCursor"`
	}{
		Alias: Alias(r),
	}
	if r.NextCursor != "" {
		aux.NextCursor = &r.NextCursor
	}
	return json.Marshal(aux)
}

func (r *TimelineResponse) UnmarshalJSON(data []byte) error {
	type Alias TimelineResponse
	aux := struct {
		*Alias
		NextCursor *string `json:"nextCursor"`
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.NextCursor != nil {
		r.NextCursor = *aux.NextCursor
	} else {
		r.NextCursor = ""
	}
	return nil
}

// parseFilterDate parses date query parameters safely into UTC time.
// Supports RFC3339 and standard YYYY-MM-DD.
// When isEnd is true, YYYY-MM-DD dates resolve to 23:59:59.999999999 UTC (inclusive end-of-day).
// When isEnd is false, YYYY-MM-DD dates resolve to 00:00:00 UTC (inclusive start-of-day).
func parseFilterDate(s string, isEnd bool) (*time.Time, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil, nil
	}

	// 1. Try RFC3339 format
	if t, err := time.Parse(time.RFC3339, trimmed); err == nil {
		utc := t.UTC()
		return &utc, nil
	}

	// 2. Try YYYY-MM-DD date format
	if t, err := time.Parse("2006-01-02", trimmed); err == nil {
		if isEnd {
			endOfDay := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, time.UTC)
			return &endOfDay, nil
		}
		startOfDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		return &startOfDay, nil
	}

	// 3. Try ISO-like datetime YYYY-MM-DDTHH:MM:SS
	if t, err := time.Parse("2006-01-02T15:04:05", trimmed); err == nil {
		utc := t.UTC()
		return &utc, nil
	}

	return nil, fmt.Errorf("invalid date format %q: expected YYYY-MM-DD or RFC3339", trimmed)
}

func parsePhotoFilterParams(c *fiber.Ctx) (repository.PhotoFilter, error) {
	q := c.Query("q", c.Query("search", ""))

	// Date filtering
	startStr := c.Query("startDate", c.Query("start_date", ""))
	startDate, err := parseFilterDate(startStr, false)
	if err != nil {
		return repository.PhotoFilter{}, fmt.Errorf("invalid startDate: %w", err)
	}

	endStr := c.Query("endDate", c.Query("end_date", ""))
	endDate, err := parseFilterDate(endStr, true)
	if err != nil {
		return repository.PhotoFilter{}, fmt.Errorf("invalid endDate: %w", err)
	}

	if startDate != nil && endDate != nil && startDate.After(*endDate) {
		return repository.PhotoFilter{}, errors.New("startDate cannot be after endDate")
	}

	// Folder filtering with path traversal prevention
	folder := c.Query("folder", c.Query("folderPath", ""))
	if folder != "" {
		cleaned := filepath.Clean(folder)
		if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, "\x00") {
			return repository.PhotoFilter{}, errors.New("invalid folder: path traversal is not permitted")
		}
	}

	// Recursive default is true unless explicitly "false"
	recursive := true
	if rStr := c.Query("recursive"); rStr != "" {
		if r, err := strconv.ParseBool(rStr); err == nil {
			recursive = r
		}
	}

	albumID := c.Query("albumId", c.Query("album_id", ""))
	mediaType := c.Query("mediaType", c.Query("media_type", ""))

	var isUndated *bool
	if uStr := c.Query("isUndated", c.Query("undated", "")); uStr != "" {
		if b, err := strconv.ParseBool(uStr); err == nil {
			isUndated = &b
		}
	}

	// Sorting
	sortBy := c.Query("sortBy", c.Query("sort", "date"))
	sortOrder := c.Query("sortOrder", c.Query("order", "desc"))

	allowedSorts := map[string]bool{
		"date": true, "taken_at": true,
		"modified": true, "file_modified_at": true,
		"filename": true, "name": true, "title": true,
		"size": true, "file_size": true,
		"created": true, "created_at": true,
	}
	if sortBy != "" && !allowedSorts[strings.ToLower(sortBy)] {
		return repository.PhotoFilter{}, fmt.Errorf("invalid sort option %q; allowed: date, filename, size, modified, created", sortBy)
	}

	allowedOrders := map[string]bool{
		"asc": true, "desc": true,
	}
	if sortOrder != "" && !allowedOrders[strings.ToLower(sortOrder)] {
		return repository.PhotoFilter{}, fmt.Errorf("invalid sort order %q; allowed: asc, desc", sortOrder)
	}

	limit := 50
	if l := c.Query("limit"); l != "" {
		parsed, err := strconv.Atoi(l)
		if err != nil || parsed < 1 {
			return repository.PhotoFilter{}, errors.New("limit must be a positive integer")
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
			return repository.PhotoFilter{}, errors.New("offset must be a non-negative integer")
		}
		offset = parsed
	}

	return repository.PhotoFilter{
		Query:     q,
		StartDate: startDate,
		EndDate:   endDate,
		Folder:    folder,
		Recursive: recursive,
		AlbumID:   albumID,
		MediaType: mediaType,
		IsUndated: isUndated,
		SortBy:    sortBy,
		SortOrder: sortOrder,
		Limit:     limit,
		Offset:    offset,
	}, nil
}

// ToPhotoResponse converts a models.Photo into a PhotoResponse with API URLs, normalized folder path, and date source.
func ToPhotoResponse(p models.Photo) PhotoResponse {
	folder := filepath.ToSlash(filepath.Dir(p.RelativePath))
	if folder == "." {
		folder = ""
	}

	isUndated := p.TakenAt == nil || p.TakenAt.IsZero()
	var dateSource string
	if !isUndated {
		dateSource = string(DateSourceExif)
	} else if !p.FileModifiedAt.IsZero() && p.FileModifiedAt.Year() > 1970 {
		dateSource = string(DateSourceFileModified)
	} else {
		dateSource = string(DateSourceNone)
	}

	return PhotoResponse{
		ID:               p.ID,
		RelativePath:     p.RelativePath,
		Folder:           folder,
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
		IsUndated:        isUndated,
		DateSource:       dateSource,
	}
}

// ListPhotosHandler handles GET /api/photos with search, filters, cursor pagination, and sorting.
func ListPhotosHandler(repo repository.PhotoRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if repo == nil {
			return SendError(c, fiber.StatusServiceUnavailable, "service_unavailable", "database repository unavailable")
		}

		filter, err := parsePhotoFilterParams(c)
		if err != nil {
			return SendError(c, fiber.StatusBadRequest, "validation_error", err.Error())
		}

		scope := ComputePhotoScope(filter)

		if cursorStr := c.Query("cursor"); cursorStr != "" {
			cData, err := DecodeCursor(cursorStr, scope)
			if err != nil {
				return SendError(c, fiber.StatusBadRequest, "validation_error", fmt.Sprintf("invalid cursor: %v", err))
			}
			filter.Cursor = cData
		}

		photos, total, hasMore, nextCursorData, err := repo.SearchAndFilterCursor(c.Context(), filter)
		if err != nil {
			return SendError(c, fiber.StatusInternalServerError, "database_error", "failed to list photos")
		}

		responseList := make([]PhotoResponse, len(photos))
		for i, p := range photos {
			responseList[i] = ToPhotoResponse(p)
		}

		var nextCursor *string
		if hasMore && nextCursorData != nil {
			token, err := EncodeCursor(nextCursorData, scope)
			if err == nil {
				nextCursor = &token
			}
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"photos":     responseList,
			"items":      responseList,
			"total":      total,
			"limit":      filter.Limit,
			"offset":     filter.Offset,
			"nextCursor": nextCursor,
			"hasMore":    hasMore,
		})
	}
}

// TimelineHandler handles GET /api/timeline with chronological ordering, day grouping, undated handling, and cursor pagination.
func TimelineHandler(repo repository.PhotoRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if repo == nil {
			return SendError(c, fiber.StatusServiceUnavailable, "service_unavailable", "database repository unavailable")
		}

		filter, err := parsePhotoFilterParams(c)
		if err != nil {
			return SendError(c, fiber.StatusBadRequest, "validation_error", err.Error())
		}

		limit := 50
		if l := c.Query("limit"); l != "" {
			parsed, err := strconv.Atoi(l)
			if err != nil || parsed < 1 {
				return SendError(c, fiber.StatusBadRequest, "validation_error", "limit must be a positive integer")
			}
			if parsed > 200 {
				parsed = 200
			}
			limit = parsed
		}
		filter.Limit = limit

		scope := ComputeTimelineScope(filter)

		if cursorStr := c.Query("cursor"); cursorStr != "" {
			cData, err := DecodeCursor(cursorStr, scope)
			if err != nil {
				return SendError(c, fiber.StatusBadRequest, "validation_error", fmt.Sprintf("invalid cursor: %v", err))
			}
			filter.Cursor = cData
		} else if o := c.Query("offset"); o != "" {
			parsed, err := strconv.Atoi(o)
			if err != nil || parsed < 0 {
				return SendError(c, fiber.StatusBadRequest, "validation_error", "offset must be a non-negative integer")
			}
			filter.Offset = parsed
		}

		photos, total, hasMore, nextCursorData, err := repo.ListTimelineCursor(c.Context(), filter)
		if err != nil {
			return SendError(c, fiber.StatusInternalServerError, "database_error", "failed to list timeline photos")
		}

		responseList := make([]PhotoResponse, len(photos))
		for i, p := range photos {
			responseList[i] = ToPhotoResponse(p)
		}

		// Group into day sections or undated section
		var groups []TimelineGroupResponse
		groupMap := make(map[string]*TimelineGroupResponse)

		for _, p := range responseList {
			var groupKey, groupLabel, month string
			var year, day int
			var isUndated bool

			if p.TakenAt != nil && !p.IsUndated {
				t := p.TakenAt.UTC()
				groupKey = t.Format("2006-01-02")
				groupLabel = t.Format("Monday, January 2, 2006")
				year = t.Year()
				month = t.Format("January")
				day = t.Day()
				isUndated = false
			} else {
				groupKey = "undated"
				groupLabel = "Undated Photos"
				isUndated = true
			}

			g, exists := groupMap[groupKey]
			if !exists {
				groups = append(groups, TimelineGroupResponse{
					Date:      groupKey,
					Label:     groupLabel,
					Year:      year,
					Month:     month,
					Day:       day,
					IsUndated: isUndated,
					Photos:    []PhotoResponse{p},
				})
				groupMap[groupKey] = &groups[len(groups)-1]
			} else {
				g.Photos = append(g.Photos, p)
			}
		}

		var nextCursor string
		if hasMore && nextCursorData != nil {
			token, err := EncodeCursor(nextCursorData, scope)
			if err == nil {
				nextCursor = token
			}
		}

		return c.Status(fiber.StatusOK).JSON(TimelineResponse{
			Photos:     responseList,
			Items:      responseList,
			Groups:     groups,
			Total:      total,
			Limit:      limit,
			Offset:     filter.Offset,
			NextCursor: nextCursor,
			HasMore:    hasMore,
		})
	}
}

// GetPhotoHandler handles GET /api/photos/:id.
// It supports optional query parameters collectionType and collectionId for prev/next photo navigation,
// and optional storage parameter to extract extended EXIF metadata.
func GetPhotoHandler(repo repository.PhotoRepository, store ...*storage.Storage) fiber.Handler {
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

		resp := ToPhotoResponse(*photo)

		// Context-aware adjacent photo lookup
		collectionType := c.Query("collectionType", c.Query("collection", ""))
		collectionID := c.Query("collectionId", c.Query("collection_id", ""))
		if collectionType != "" {
			prevID, nextID, err := repo.GetAdjacentPhotos(c.Context(), id, collectionType, collectionID)
			if err == nil {
				resp.PrevID = prevID
				resp.NextID = nextID
			}
		}

		// On-demand EXIF extraction for detailed viewer inspection
		if len(store) > 0 && store[0] != nil {
			resolvedPath, err := store[0].ResolvePath(photo.RelativePath)
			if err == nil {
				f, err := os.Open(resolvedPath)
				if err == nil {
					defer f.Close()
					if details, err := media.ExtractExifDetails(f); err == nil && details != nil {
						resp.Exif = details
					}
				}
			}
		}

		return c.Status(fiber.StatusOK).JSON(resp)
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
			if !media.IsSupportedExtension(origName) {
				uploadErrors = append(uploadErrors, FileError{
					Filename: origName,
					Error:    "unsupported image format",
				})
				continue
			}

			if fh.Size > MaxUploadFileSize {
				uploadErrors = append(uploadErrors, FileError{
					Filename: origName,
					Error:    fmt.Sprintf("file size %d exceeds 25MB limit", fh.Size),
				})
				continue
			}

			srcFile, err := fh.Open()
			if err != nil {
				uploadErrors = append(uploadErrors, FileError{
					Filename: origName,
					Error:    "failed to open uploaded file",
				})
				continue
			}

			// Stream into bounded temporary file
			tmpFileName := fmt.Sprintf("upload_%s_%d.tmp", uuid.New().String(), time.Now().UnixNano())
			tmpFilePath := filepath.Join(store.TmpDir(), tmpFileName)
			tmpDst, err := os.OpenFile(tmpFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err != nil {
				srcFile.Close()
				uploadErrors = append(uploadErrors, FileError{
					Filename: origName,
					Error:    "failed to create temporary storage file",
				})
				continue
			}

			lr := io.LimitReader(srcFile, MaxUploadFileSize+1)
			written, err := io.Copy(tmpDst, lr)
			tmpDst.Close()
			srcFile.Close()

			if err != nil {
				_ = os.Remove(tmpFilePath)
				uploadErrors = append(uploadErrors, FileError{
					Filename: origName,
					Error:    "failed to stream file to storage",
				})
				continue
			}

			if written > MaxUploadFileSize {
				_ = os.Remove(tmpFilePath)
				uploadErrors = append(uploadErrors, FileError{
					Filename: origName,
					Error:    "file exceeded maximum size limit",
				})
				continue
			}

			// Validate image content and extract metadata safely
			tmpReader, err := os.Open(tmpFilePath)
			if err != nil {
				_ = os.Remove(tmpFilePath)
				uploadErrors = append(uploadErrors, FileError{
					Filename: origName,
					Error:    "failed to read uploaded file for validation",
				})
				continue
			}

			meta, extractErr := media.ExtractMetadata(tmpReader)
			tmpReader.Close()
			if extractErr != nil {
				_ = os.Remove(tmpFilePath)
				uploadErrors = append(uploadErrors, FileError{
					Filename: origName,
					Error:    fmt.Sprintf("invalid image file: %v", extractErr),
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

			// Proactively generate thumbnail
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

// ScanPhotosHandler handles POST /api/photos/scan with optional ?path=..., ?sync=true, and ?refresh=true.
func ScanPhotosHandler(s *scanner.Scanner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if s == nil {
			return SendError(c, fiber.StatusServiceUnavailable, "service_unavailable", "media scanner unavailable")
		}

		isSync := c.Query("sync") == "true"
		targetPath := c.Query("path")
		forceRefresh := c.Query("refresh") == "true"

		if isSync {
			status, err := s.ScanPath(c.Context(), targetPath, forceRefresh)
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
			_, _ = s.ScanPath(context.Background(), targetPath, forceRefresh)
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
