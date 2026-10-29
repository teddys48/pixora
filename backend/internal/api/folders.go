package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"pixora/backend/internal/media"
	"pixora/backend/internal/models"
	"pixora/backend/internal/repository"
	"pixora/backend/internal/storage"
)

// PhysicalFolderItem represents a single immediate child directory under a physical folder.
type PhysicalFolderItem struct {
	Name              string  `json:"name"`
	Path              string  `json:"path"`
	PhotoCount        int     `json:"photoCount"`
	DirectPhotoCount  int     `json:"directPhotoCount"`
	SubfolderCount    int     `json:"subfolderCount"`
	CoverPhotoID      *string `json:"coverPhotoId,omitempty"`
	CoverThumbnailURL *string `json:"coverThumbnailUrl,omitempty"`
	HasUnindexedFiles bool    `json:"hasUnindexedFiles"`
}

// BreadcrumbItem represents one hop in the directory hierarchy.
type BreadcrumbItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// FolderListResponse represents the payload of GET /api/folders.
type FolderListResponse struct {
	Path    string               `json:"path"`
	Folders []PhysicalFolderItem `json:"folders"`
	Total   int                  `json:"total"`
}

// ListFoldersResponse is an alias for FolderListResponse for backward compatibility.
type ListFoldersResponse = FolderListResponse

// FolderContentsResponse represents the payload of GET /api/folders/contents.
type FolderContentsResponse struct {
	Name              string               `json:"name"`
	Path              string               `json:"path"`
	ParentPath        *string              `json:"parentPath"`
	Breadcrumbs       []BreadcrumbItem     `json:"breadcrumbs"`
	Folders           []PhysicalFolderItem `json:"folders"`
	Photos            []PhotoResponse      `json:"photos"`
	Items             []PhotoResponse      `json:"items"`
	TotalPhotos       int                  `json:"totalPhotos"`
	Limit             int                  `json:"limit"`
	Offset            int                  `json:"offset"`
	NextCursor        *string              `json:"nextCursor"`
	HasMore           bool                 `json:"hasMore"`
	DiskMediaCount    int                  `json:"diskMediaCount"`
	UnindexedCount    int                  `json:"unindexedCount"`
	HasUnindexedFiles bool                 `json:"hasUnindexedFiles"`
}

func handleFolderResolutionError(c *fiber.Ctx, err error, userPath string) error {
	if errors.Is(err, storage.ErrPathTraversal) {
		return SendError(c, fiber.StatusForbidden, "invalid_path", "security violation: path traversal detected")
	}
	if errors.Is(err, storage.ErrInvalidPath) {
		return SendError(c, fiber.StatusBadRequest, "validation_error", "invalid folder path")
	}
	if errors.Is(err, storage.ErrNotDirectory) {
		return SendError(c, fiber.StatusBadRequest, "validation_error", "path is not a directory")
	}
	if errors.Is(err, storage.ErrPermissionDenied) {
		return SendError(c, fiber.StatusForbidden, "permission_denied", "access to folder is denied")
	}
	if os.IsNotExist(err) {
		return SendError(c, fiber.StatusNotFound, "not_found", fmt.Sprintf("folder %q not found", userPath))
	}
	return SendError(c, fiber.StatusInternalServerError, "filesystem_error", "failed to resolve folder path")
}

func discoverImmediateFolders(
	ctx context.Context,
	store *storage.Storage,
	repo repository.PhotoRepository,
	resolvedAbsPath string,
	canonicalRelPath string,
) ([]PhysicalFolderItem, int, error) {
	entries, err := os.ReadDir(resolvedAbsPath)
	if err != nil {
		return nil, 0, err
	}

	realRoot, err := filepath.EvalSymlinks(store.Root())
	if err != nil {
		realRoot = store.Root()
	}

	folders := make([]PhysicalFolderItem, 0)
	var diskMediaCount int

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		isDir := entry.IsDir()
		fullPath := filepath.Join(resolvedAbsPath, name)

		if entry.Type()&os.ModeSymlink != 0 {
			targetPath, evalErr := filepath.EvalSymlinks(fullPath)
			if evalErr == nil {
				if rel, relErr := filepath.Rel(realRoot, targetPath); relErr == nil && !strings.HasPrefix(rel, "..") {
					if info, statErr := os.Stat(targetPath); statErr == nil {
						if info.IsDir() {
							isDir = true
						} else if media.IsSupportedExtension(name) {
							diskMediaCount++
							continue
						}
					}
				}
			}
		}

		if !isDir {
			if media.IsSupportedExtension(name) {
				diskMediaCount++
			}
			continue
		}

		// At storage root, skip internal directories
		if canonicalRelPath == "" && (name == "thumbnails" || name == "tmp") {
			continue
		}

		subRelPath := name
		if canonicalRelPath != "" {
			subRelPath = canonicalRelPath + "/" + name
		}

		recCount, _ := repo.CountByFolder(ctx, subRelPath, true)
		dirCount, _ := repo.CountByFolder(ctx, subRelPath, false)

		childSubfolderCount := 0
		subHasUnindexed := false
		subDirAbs := filepath.Join(resolvedAbsPath, name)
		if subEntries, readErr := os.ReadDir(subDirAbs); readErr == nil {
			subDiskMedia := 0
			for _, se := range subEntries {
				seName := se.Name()
				if strings.HasPrefix(seName, ".") {
					continue
				}
				if se.IsDir() {
					childSubfolderCount++
				} else if media.IsSupportedExtension(seName) {
					subDiskMedia++
				}
			}
			if subDiskMedia > dirCount {
				subHasUnindexed = true
			}
		}

		var coverID *string
		var coverURL *string
		if recCount > 0 {
			coverPhoto, _ := repo.GetFolderCoverPhoto(ctx, subRelPath)
			if coverPhoto != nil {
				cID := coverPhoto.ID
				coverID = &cID
				u := fmt.Sprintf("/api/photos/%s/thumbnail", cID)
				coverURL = &u
			}
		}

		folders = append(folders, PhysicalFolderItem{
			Name:              name,
			Path:              subRelPath,
			PhotoCount:        recCount,
			DirectPhotoCount:  dirCount,
			SubfolderCount:    childSubfolderCount,
			CoverPhotoID:      coverID,
			CoverThumbnailURL: coverURL,
			HasUnindexedFiles: subHasUnindexed,
		})
	}

	return folders, diskMediaCount, nil
}

func buildBreadcrumbs(canonicalRelPath string) []BreadcrumbItem {
	crumbs := []BreadcrumbItem{
		{Name: "Folders", Path: ""},
	}
	if canonicalRelPath == "" {
		return crumbs
	}

	parts := strings.Split(canonicalRelPath, "/")
	accum := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		if accum == "" {
			accum = part
		} else {
			accum = accum + "/" + part
		}
		crumbs = append(crumbs, BreadcrumbItem{
			Name: part,
			Path: accum,
		})
	}
	return crumbs
}

func getParentPath(canonicalRelPath string) *string {
	if canonicalRelPath == "" {
		return nil
	}
	idx := strings.LastIndex(canonicalRelPath, "/")
	if idx == -1 {
		empty := ""
		return &empty
	}
	parent := canonicalRelPath[:idx]
	return &parent
}

// ListFoldersHandler handles GET /api/folders with optional ?path=... and sorting.
func ListFoldersHandler(store *storage.Storage, repo repository.PhotoRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if store == nil || repo == nil {
			return SendError(c, fiber.StatusServiceUnavailable, "service_unavailable", "storage or repository unavailable")
		}

		userPath := c.Query("path")
		resolvedAbs, canonicalRel, err := store.ResolveFolderPath(userPath)
		if err != nil {
			return handleFolderResolutionError(c, err, userPath)
		}

		folders, _, err := discoverImmediateFolders(c.Context(), store, repo, resolvedAbs, canonicalRel)
		if err != nil {
			return SendError(c, fiber.StatusInternalServerError, "filesystem_error", "failed to read folder contents")
		}
		if folders == nil {
			folders = make([]PhysicalFolderItem, 0)
		}

		sortBy := c.Query("sort", "name")
		order := strings.ToLower(c.Query("order", "asc"))

		sort.Slice(folders, func(i, j int) bool {
			if sortBy == "photos" {
				if order == "desc" {
					return folders[i].PhotoCount > folders[j].PhotoCount
				}
				return folders[i].PhotoCount < folders[j].PhotoCount
			}
			if sortBy == "subfolders" {
				if order == "desc" {
					return folders[i].SubfolderCount > folders[j].SubfolderCount
				}
				return folders[i].SubfolderCount < folders[j].SubfolderCount
			}
			if order == "desc" {
				return strings.ToLower(folders[i].Name) > strings.ToLower(folders[j].Name)
			}
			return strings.ToLower(folders[i].Name) < strings.ToLower(folders[j].Name)
		})

		return c.Status(fiber.StatusOK).JSON(FolderListResponse{
			Path:    canonicalRel,
			Folders: folders,
			Total:   len(folders),
		})
	}
}

// GetFolderContentsHandler handles GET /api/folders/contents?path=...
func GetFolderContentsHandler(store *storage.Storage, repo repository.PhotoRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if store == nil || repo == nil {
			return SendError(c, fiber.StatusServiceUnavailable, "service_unavailable", "storage or repository unavailable")
		}

		userPath := c.Query("path")
		resolvedAbs, canonicalRel, err := store.ResolveFolderPath(userPath)
		if err != nil {
			return handleFolderResolutionError(c, err, userPath)
		}

		folders, diskMediaCount, err := discoverImmediateFolders(c.Context(), store, repo, resolvedAbs, canonicalRel)
		if err != nil {
			return SendError(c, fiber.StatusInternalServerError, "filesystem_error", "failed to read folder contents")
		}
		if folders == nil {
			folders = make([]PhysicalFolderItem, 0)
		}

		// Sort child folders alphabetically by default
		sort.Slice(folders, func(i, j int) bool {
			return strings.ToLower(folders[i].Name) < strings.ToLower(folders[j].Name)
		})

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

		scope := ComputeFolderScope(canonicalRel, sortBy, order)
		var cursor *repository.CursorData
		if cursorStr := c.Query("cursor"); cursorStr != "" {
			cData, err := DecodeCursor(cursorStr, scope)
			if err != nil {
				return SendError(c, fiber.StatusBadRequest, "validation_error", fmt.Sprintf("invalid cursor: %v", err))
			}
			cursor = cData
		}

		var photos []models.Photo
		var totalPhotos int
		var hasMore bool
		var nextCursorData *repository.CursorData

		if cursor != nil {
			photos, totalPhotos, hasMore, nextCursorData, err = repo.ListByFolderCursor(c.Context(), canonicalRel, limit, sortBy, order, cursor)
		} else {
			// Offset or initial page
			filter := repository.PhotoFilter{
				Folder:    canonicalRel,
				Recursive: false,
				SortBy:    sortBy,
				SortOrder: order,
				Limit:     limit,
				Offset:    offset,
			}
			photos, totalPhotos, hasMore, nextCursorData, err = repo.SearchAndFilterCursor(c.Context(), filter)
		}

		if err != nil {
			return SendError(c, fiber.StatusInternalServerError, "database_error", "failed to list folder photos")
		}

		photoResponses := make([]PhotoResponse, 0, len(photos))
		for _, p := range photos {
			photoResponses = append(photoResponses, ToPhotoResponse(p))
		}

		unindexed := diskMediaCount - totalPhotos
		if unindexed < 0 {
			unindexed = 0
		}

		folderName := "Folders"
		if canonicalRel != "" {
			folderName = filepath.Base(canonicalRel)
		}

		var nextCursor *string
		if hasMore && nextCursorData != nil {
			token, err := EncodeCursor(nextCursorData, scope)
			if err == nil {
				nextCursor = &token
			}
		}

		return c.Status(fiber.StatusOK).JSON(FolderContentsResponse{
			Name:              folderName,
			Path:              canonicalRel,
			ParentPath:        getParentPath(canonicalRel),
			Breadcrumbs:       buildBreadcrumbs(canonicalRel),
			Folders:           folders,
			Photos:            photoResponses,
			Items:             photoResponses,
			TotalPhotos:       totalPhotos,
			Limit:             limit,
			Offset:            offset,
			NextCursor:        nextCursor,
			HasMore:           hasMore,
			DiskMediaCount:    diskMediaCount,
			UnindexedCount:    unindexed,
			HasUnindexedFiles: unindexed > 0,
		})
	}
}

// GetFolderPhotosHandler handles GET /api/folders/photos?path=...
func GetFolderPhotosHandler(store *storage.Storage, repo repository.PhotoRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if store == nil || repo == nil {
			return SendError(c, fiber.StatusServiceUnavailable, "service_unavailable", "storage or repository unavailable")
		}

		userPath := c.Query("path")
		_, canonicalRel, err := store.ResolveFolderPath(userPath)
		if err != nil {
			return handleFolderResolutionError(c, err, userPath)
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

		scope := ComputeFolderScope(canonicalRel, sortBy, order)
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

		if cursor != nil {
			photos, total, hasMore, nextCursorData, err = repo.ListByFolderCursor(c.Context(), canonicalRel, limit, sortBy, order, cursor)
		} else {
			filter := repository.PhotoFilter{
				Folder:    canonicalRel,
				Recursive: false,
				SortBy:    sortBy,
				SortOrder: order,
				Limit:     limit,
				Offset:    offset,
			}
			photos, total, hasMore, nextCursorData, err = repo.SearchAndFilterCursor(c.Context(), filter)
		}

		if err != nil {
			return SendError(c, fiber.StatusInternalServerError, "database_error", "failed to list folder photos")
		}

		photoResponses := make([]PhotoResponse, 0, len(photos))
		for _, p := range photos {
			photoResponses = append(photoResponses, ToPhotoResponse(p))
		}

		var nextCursor *string
		if hasMore && nextCursorData != nil {
			token, err := EncodeCursor(nextCursorData, scope)
			if err == nil {
				nextCursor = &token
			}
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"path":       canonicalRel,
			"photos":     photoResponses,
			"items":      photoResponses,
			"total":      total,
			"limit":      limit,
			"offset":     offset,
			"nextCursor": nextCursor,
			"hasMore":    hasMore,
		})
	}
}
