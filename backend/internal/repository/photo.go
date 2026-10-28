package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"pixora/backend/internal/database"
	"pixora/backend/internal/models"
)

// PhotoRepository defines data access methods for photos.
type PhotoRepository interface {
	Create(ctx context.Context, p *models.Photo) error
	Update(ctx context.Context, p *models.Photo) error
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (*models.Photo, error)
	GetByRelativePath(ctx context.Context, relPath string) (*models.Photo, error)
	List(ctx context.Context, offset, limit int, sortBy, order string) ([]models.Photo, int, error)
	Count(ctx context.Context) (int, error)
	ListByFolder(ctx context.Context, folderPath string, offset, limit int, sortBy, order string) ([]models.Photo, int, error)
	CountByFolder(ctx context.Context, folderPath string, recursive bool) (int, error)
	GetFolderCoverPhoto(ctx context.Context, folderPath string) (*models.Photo, error)
}

// SQLitePhotoRepository implements PhotoRepository using SQLite.
type SQLitePhotoRepository struct {
	db *database.DB
}

// NewSQLitePhotoRepository creates a new SQLitePhotoRepository.
func NewSQLitePhotoRepository(db *database.DB) *SQLitePhotoRepository {
	return &SQLitePhotoRepository{db: db}
}

func cleanFolderPath(p string) string {
	cleaned := strings.Trim(filepath.ToSlash(filepath.Clean(strings.TrimSpace(p))), "/")
	if cleaned == "." {
		return ""
	}
	return cleaned
}

// Create inserts a new photo record.
func (r *SQLitePhotoRepository) Create(ctx context.Context, p *models.Photo) error {
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = now
	}
	if p.FileModifiedAt.IsZero() {
		p.FileModifiedAt = now
	}

	query := `
INSERT INTO photos (
    id, relative_path, original_filename, media_type, file_size,
    width, height, taken_at, file_modified_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query,
		p.ID, p.RelativePath, p.OriginalFilename, p.MediaType, p.FileSize,
		p.Width, p.Height, p.TakenAt, p.FileModifiedAt, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			return fmt.Errorf("%w: photo with relative_path %q or id %q already exists", ErrConflict, p.RelativePath, p.ID)
		}
		return fmt.Errorf("failed to insert photo: %w", err)
	}

	return nil
}

// Update updates an existing photo record by ID.
func (r *SQLitePhotoRepository) Update(ctx context.Context, p *models.Photo) error {
	p.UpdatedAt = time.Now().UTC()
	query := `
UPDATE photos
SET original_filename = ?, media_type = ?, file_size = ?,
    width = ?, height = ?, taken_at = ?, file_modified_at = ?, updated_at = ?
WHERE id = ?`

	res, err := r.db.ExecContext(ctx, query,
		p.OriginalFilename, p.MediaType, p.FileSize,
		p.Width, p.Height, p.TakenAt, p.FileModifiedAt, p.UpdatedAt,
		p.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update photo: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a photo record by ID.
func (r *SQLitePhotoRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM photos WHERE id = ?`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete photo: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// GetByID retrieves a photo by its stable ID.
func (r *SQLitePhotoRepository) GetByID(ctx context.Context, id string) (*models.Photo, error) {
	query := `
SELECT id, relative_path, original_filename, media_type, file_size,
       width, height, taken_at, file_modified_at, created_at, updated_at
FROM photos
WHERE id = ?`

	var p models.Photo
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&p.ID, &p.RelativePath, &p.OriginalFilename, &p.MediaType, &p.FileSize,
		&p.Width, &p.Height, &p.TakenAt, &p.FileModifiedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get photo by id: %w", err)
	}

	return &p, nil
}

// GetByRelativePath retrieves a photo by its relative storage path.
func (r *SQLitePhotoRepository) GetByRelativePath(ctx context.Context, relPath string) (*models.Photo, error) {
	query := `
SELECT id, relative_path, original_filename, media_type, file_size,
       width, height, taken_at, file_modified_at, created_at, updated_at
FROM photos
WHERE relative_path = ?`

	var p models.Photo
	err := r.db.QueryRowContext(ctx, query, relPath).Scan(
		&p.ID, &p.RelativePath, &p.OriginalFilename, &p.MediaType, &p.FileSize,
		&p.Width, &p.Height, &p.TakenAt, &p.FileModifiedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get photo by relative path: %w", err)
	}

	return &p, nil
}

// List returns a paginated list of photos with deterministic ordering.
func (r *SQLitePhotoRepository) List(ctx context.Context, offset, limit int, sortBy, order string) ([]models.Photo, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	// Map sort options safely
	var orderByClause string
	dir := "DESC"
	if strings.ToLower(order) == "asc" {
		dir = "ASC"
	}

	switch sortBy {
	case "date":
		// Prefer taken_at, fallback to file_modified_at then created_at
		orderByClause = fmt.Sprintf("COALESCE(taken_at, file_modified_at, created_at) %s, id ASC", dir)
	case "modified":
		orderByClause = fmt.Sprintf("file_modified_at %s, id ASC", dir)
	case "filename":
		orderByClause = fmt.Sprintf("original_filename %s, id ASC", dir)
	case "size":
		orderByClause = fmt.Sprintf("file_size %s, id ASC", dir)
	case "created":
		orderByClause = fmt.Sprintf("created_at %s, id ASC", dir)
	default:
		orderByClause = fmt.Sprintf("COALESCE(taken_at, file_modified_at, created_at) %s, id ASC", dir)
	}

	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM photos").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count photos: %w", err)
	}

	query := fmt.Sprintf(`
SELECT id, relative_path, original_filename, media_type, file_size,
       width, height, taken_at, file_modified_at, created_at, updated_at
FROM photos
ORDER BY %s
LIMIT ? OFFSET ?`, orderByClause)

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query photos: %w", err)
	}
	defer rows.Close()

	photos := make([]models.Photo, 0, limit)
	for rows.Next() {
		var p models.Photo
		if err := rows.Scan(
			&p.ID, &p.RelativePath, &p.OriginalFilename, &p.MediaType, &p.FileSize,
			&p.Width, &p.Height, &p.TakenAt, &p.FileModifiedAt, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan photo: %w", err)
		}
		photos = append(photos, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error during photos iteration: %w", err)
	}

	return photos, total, nil
}

// Count returns the total number of indexed photos.
func (r *SQLitePhotoRepository) Count(ctx context.Context) (int, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM photos").Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count photos: %w", err)
	}
	return count, nil
}

// ListByFolder returns photos whose direct parent folder is folderPath, with deterministic sorting and pagination.
func (r *SQLitePhotoRepository) ListByFolder(ctx context.Context, folderPath string, offset, limit int, sortBy, order string) ([]models.Photo, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	normPath := cleanFolderPath(folderPath)

	var whereClause string
	var countArgs []interface{}

	if normPath == "" {
		whereClause = "INSTR(relative_path, '/') = 0"
	} else {
		prefix := normPath + "/"
		whereClause = "SUBSTR(relative_path, 1, ?) = ? AND INSTR(SUBSTR(relative_path, ?), '/') = 0"
		countArgs = append(countArgs, len(prefix), prefix, len(prefix)+1)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM photos WHERE %s", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count photos in folder %q: %w", normPath, err)
	}

	var orderByClause string
	dir := "DESC"
	if strings.ToLower(order) == "asc" {
		dir = "ASC"
	}

	switch sortBy {
	case "date":
		orderByClause = fmt.Sprintf("COALESCE(taken_at, file_modified_at, created_at) %s, id ASC", dir)
	case "modified":
		orderByClause = fmt.Sprintf("file_modified_at %s, id ASC", dir)
	case "filename":
		orderByClause = fmt.Sprintf("original_filename %s, id ASC", dir)
	case "size":
		orderByClause = fmt.Sprintf("file_size %s, id ASC", dir)
	case "created":
		orderByClause = fmt.Sprintf("created_at %s, id ASC", dir)
	default:
		orderByClause = fmt.Sprintf("COALESCE(taken_at, file_modified_at, created_at) %s, id ASC", dir)
	}

	query := fmt.Sprintf(`
SELECT id, relative_path, original_filename, media_type, file_size,
       width, height, taken_at, file_modified_at, created_at, updated_at
FROM photos
WHERE %s
ORDER BY %s
LIMIT ? OFFSET ?`, whereClause, orderByClause)

	queryArgs := make([]interface{}, len(countArgs), len(countArgs)+2)
	copy(queryArgs, countArgs)
	queryArgs = append(queryArgs, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query photos in folder %q: %w", normPath, err)
	}
	defer rows.Close()

	photos := make([]models.Photo, 0, limit)
	for rows.Next() {
		var p models.Photo
		if err := rows.Scan(
			&p.ID, &p.RelativePath, &p.OriginalFilename, &p.MediaType, &p.FileSize,
			&p.Width, &p.Height, &p.TakenAt, &p.FileModifiedAt, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan photo: %w", err)
		}
		photos = append(photos, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error during folder photos iteration: %w", err)
	}

	return photos, total, nil
}

// CountByFolder returns the count of photos in folderPath. If recursive is true, includes all descendant subfolders.
func (r *SQLitePhotoRepository) CountByFolder(ctx context.Context, folderPath string, recursive bool) (int, error) {
	normPath := cleanFolderPath(folderPath)

	var query string
	var args []interface{}

	if normPath == "" {
		if recursive {
			query = "SELECT COUNT(*) FROM photos"
		} else {
			query = "SELECT COUNT(*) FROM photos WHERE INSTR(relative_path, '/') = 0"
		}
	} else {
		prefix := normPath + "/"
		if recursive {
			query = "SELECT COUNT(*) FROM photos WHERE SUBSTR(relative_path, 1, ?) = ?"
			args = append(args, len(prefix), prefix)
		} else {
			query = "SELECT COUNT(*) FROM photos WHERE SUBSTR(relative_path, 1, ?) = ? AND INSTR(SUBSTR(relative_path, ?), '/') = 0"
			args = append(args, len(prefix), prefix, len(prefix)+1)
		}
	}

	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count photos for folder %q (recursive=%v): %w", normPath, recursive, err)
	}
	return count, nil
}

// GetFolderCoverPhoto returns the latest photo in folderPath (or its descendants) to use as cover thumbnail.
func (r *SQLitePhotoRepository) GetFolderCoverPhoto(ctx context.Context, folderPath string) (*models.Photo, error) {
	normPath := cleanFolderPath(folderPath)

	var query string
	var args []interface{}

	if normPath == "" {
		query = `
SELECT id, relative_path, original_filename, media_type, file_size,
       width, height, taken_at, file_modified_at, created_at, updated_at
FROM photos
ORDER BY COALESCE(taken_at, file_modified_at, created_at) DESC, id ASC
LIMIT 1`
	} else {
		prefix := normPath + "/"
		query = `
SELECT id, relative_path, original_filename, media_type, file_size,
       width, height, taken_at, file_modified_at, created_at, updated_at
FROM photos
WHERE SUBSTR(relative_path, 1, ?) = ?
ORDER BY COALESCE(taken_at, file_modified_at, created_at) DESC, id ASC
LIMIT 1`
		args = append(args, len(prefix), prefix)
	}

	var p models.Photo
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&p.ID, &p.RelativePath, &p.OriginalFilename, &p.MediaType, &p.FileSize,
		&p.Width, &p.Height, &p.TakenAt, &p.FileModifiedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get cover photo for folder %q: %w", normPath, err)
	}

	return &p, nil
}
