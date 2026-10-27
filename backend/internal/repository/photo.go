package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"pixora/backend/internal/database"
	"pixora/backend/internal/models"
)

// PhotoRepository defines data access methods for photos.
type PhotoRepository interface {
	Create(ctx context.Context, p *models.Photo) error
	GetByID(ctx context.Context, id string) (*models.Photo, error)
	GetByRelativePath(ctx context.Context, relPath string) (*models.Photo, error)
	List(ctx context.Context, offset, limit int, sortBy, order string) ([]models.Photo, int, error)
	Count(ctx context.Context) (int, error)
}

// SQLitePhotoRepository implements PhotoRepository using SQLite.
type SQLitePhotoRepository struct {
	db *database.DB
}

// NewSQLitePhotoRepository creates a new SQLitePhotoRepository.
func NewSQLitePhotoRepository(db *database.DB) *SQLitePhotoRepository {
	return &SQLitePhotoRepository{db: db}
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
