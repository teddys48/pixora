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

// PhotoFilter defines search, filter, and sorting criteria for querying photos.
type PhotoFilter struct {
	Query     string     // Substring match on original_filename or relative_path
	StartDate *time.Time // Inclusive start instant for verified capture date (taken_at)
	EndDate   *time.Time // Inclusive end instant for verified capture date (taken_at)
	Folder    string     // Relative physical folder path
	Recursive bool       // Whether to include photos in child subfolders (default true)
	AlbumID   string     // Virtual album UUID
	MediaType string     // MIME type (e.g. "image/jpeg") or prefix (e.g. "image/*")
	IsUndated *bool      // If true: taken_at IS NULL; if false: taken_at IS NOT NULL
	SortBy    string     // "date", "modified", "filename", "size", "created"
	SortOrder string     // "desc", "asc"
	Offset    int
	Limit     int
}

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
	ListTimeline(ctx context.Context, offset, limit int) ([]models.Photo, int, error)
	GetAdjacentPhotos(ctx context.Context, id string, collectionType string, collectionID string) (prevID *string, nextID *string, err error)
	SearchAndFilter(ctx context.Context, filter PhotoFilter) ([]models.Photo, int, error)
	ListTimelineWithFilter(ctx context.Context, filter PhotoFilter) ([]models.Photo, int, error)
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

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func buildPhotoFilterConditions(filter PhotoFilter) (string, []interface{}) {
	var whereClauses []string
	var args []interface{}

	// 1. Text search on original_filename or relative_path (case-insensitive substring)
	q := strings.TrimSpace(filter.Query)
	if q != "" {
		escaped := "%" + escapeLike(q) + "%"
		whereClauses = append(whereClauses, "(original_filename LIKE ? ESCAPE '\\' OR relative_path LIKE ? ESCAPE '\\')")
		args = append(args, escaped, escaped)
	}

	// 2. Date range filtering on verified capture date (taken_at)
	if filter.StartDate != nil {
		whereClauses = append(whereClauses, "taken_at IS NOT NULL AND taken_at >= ?")
		args = append(args, filter.StartDate.UTC())
	}
	if filter.EndDate != nil {
		whereClauses = append(whereClauses, "taken_at IS NOT NULL AND taken_at <= ?")
		args = append(args, filter.EndDate.UTC())
	}

	// 3. Undated photos filtering
	if filter.IsUndated != nil {
		if *filter.IsUndated {
			whereClauses = append(whereClauses, "taken_at IS NULL")
		} else {
			whereClauses = append(whereClauses, "taken_at IS NOT NULL")
		}
	}

	// 4. Physical folder filtering
	folder := cleanFolderPath(filter.Folder)
	if folder != "" {
		prefix := folder + "/"
		if filter.Recursive {
			whereClauses = append(whereClauses, "(relative_path = ? OR SUBSTR(relative_path, 1, ?) = ?)")
			args = append(args, folder, len(prefix), prefix)
		} else {
			whereClauses = append(whereClauses, "SUBSTR(relative_path, 1, ?) = ? AND INSTR(SUBSTR(relative_path, ?), '/') = 0")
			args = append(args, len(prefix), prefix, len(prefix)+1)
		}
	} else if strings.TrimSpace(filter.Folder) == "/" || strings.TrimSpace(filter.Folder) == "." {
		if !filter.Recursive {
			whereClauses = append(whereClauses, "INSTR(relative_path, '/') = 0")
		}
	}

	// 5. Virtual album filtering
	albumID := strings.TrimSpace(filter.AlbumID)
	if albumID != "" {
		whereClauses = append(whereClauses, "EXISTS (SELECT 1 FROM photo_albums pa WHERE pa.photo_id = photos.id AND pa.album_id = ?)")
		args = append(args, albumID)
	}

	// 6. Media type filtering
	mediaType := strings.TrimSpace(filter.MediaType)
	if mediaType != "" {
		if strings.HasSuffix(mediaType, "/*") {
			prefix := strings.TrimSuffix(mediaType, "/*") + "/"
			whereClauses = append(whereClauses, "SUBSTR(media_type, 1, ?) = ?")
			args = append(args, len(prefix), prefix)
		} else {
			whereClauses = append(whereClauses, "media_type = ?")
			args = append(args, mediaType)
		}
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	return whereSQL, args
}

func buildPhotoOrderBy(sortBy, sortOrder string) string {
	dir := "DESC"
	if strings.ToLower(sortOrder) == "asc" {
		dir = "ASC"
	}

	switch strings.ToLower(sortBy) {
	case "date", "taken_at":
		return fmt.Sprintf("(CASE WHEN taken_at IS NOT NULL THEN 0 ELSE 1 END) ASC, taken_at %s, file_modified_at %s, id ASC", dir, dir)
	case "modified", "file_modified_at":
		return fmt.Sprintf("file_modified_at %s, id ASC", dir)
	case "filename", "name", "title":
		return fmt.Sprintf("original_filename COLLATE NOCASE %s, id ASC", dir)
	case "size", "file_size":
		return fmt.Sprintf("file_size %s, id ASC", dir)
	case "created", "created_at":
		return fmt.Sprintf("created_at %s, id ASC", dir)
	default:
		return fmt.Sprintf("(CASE WHEN taken_at IS NOT NULL THEN 0 ELSE 1 END) ASC, taken_at %s, file_modified_at %s, id ASC", dir, dir)
	}
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
	return r.SearchAndFilter(ctx, PhotoFilter{
		Offset:    offset,
		Limit:     limit,
		SortBy:    sortBy,
		SortOrder: order,
	})
}

// SearchAndFilter returns a paginated slice of photos matching the given filter criteria, along with total count.
func (r *SQLitePhotoRepository) SearchAndFilter(ctx context.Context, filter PhotoFilter) ([]models.Photo, int, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	whereSQL, args := buildPhotoFilterConditions(filter)

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM photos %s", whereSQL)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count filtered photos: %w", err)
	}

	orderBy := buildPhotoOrderBy(filter.SortBy, filter.SortOrder)
	dataQuery := fmt.Sprintf(`
SELECT id, relative_path, original_filename, media_type, file_size,
       width, height, taken_at, file_modified_at, created_at, updated_at
FROM photos
%s
ORDER BY %s
LIMIT ? OFFSET ?`, whereSQL, orderBy)

	queryArgs := make([]interface{}, len(args), len(args)+2)
	copy(queryArgs, args)
	queryArgs = append(queryArgs, limit, offset)

	rows, err := r.db.QueryContext(ctx, dataQuery, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query filtered photos: %w", err)
	}
	defer rows.Close()

	photos := make([]models.Photo, 0, limit)
	for rows.Next() {
		var p models.Photo
		if err := rows.Scan(
			&p.ID, &p.RelativePath, &p.OriginalFilename, &p.MediaType, &p.FileSize,
			&p.Width, &p.Height, &p.TakenAt, &p.FileModifiedAt, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan filtered photo: %w", err)
		}
		photos = append(photos, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error during filtered photos iteration: %w", err)
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

	orderByClause := buildPhotoOrderBy(sortBy, order)

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
ORDER BY (CASE WHEN taken_at IS NOT NULL THEN 0 ELSE 1 END) ASC, taken_at DESC, file_modified_at DESC, id ASC
LIMIT 1`
	} else {
		prefix := normPath + "/"
		query = `
SELECT id, relative_path, original_filename, media_type, file_size,
       width, height, taken_at, file_modified_at, created_at, updated_at
FROM photos
WHERE SUBSTR(relative_path, 1, ?) = ?
ORDER BY (CASE WHEN taken_at IS NOT NULL THEN 0 ELSE 1 END) ASC, taken_at DESC, file_modified_at DESC, id ASC
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

// ListTimeline returns a paginated list of photos ordered chronologically:
// 1. Photos with a verified capture date (taken_at IS NOT NULL) sorted descending (newest capture date first)
// 2. Photos without capture dates (taken_at IS NULL) sorted by file_modified_at descending as fallback
// Consistent deterministic sorting is ensured via tie-breaker (id ASC).
func (r *SQLitePhotoRepository) ListTimeline(ctx context.Context, offset, limit int) ([]models.Photo, int, error) {
	return r.ListTimelineWithFilter(ctx, PhotoFilter{
		Offset: offset,
		Limit:  limit,
	})
}

// ListTimelineWithFilter returns chronologically ordered photos matching filter criteria.
func (r *SQLitePhotoRepository) ListTimelineWithFilter(ctx context.Context, filter PhotoFilter) ([]models.Photo, int, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	whereSQL, args := buildPhotoFilterConditions(filter)

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM photos %s", whereSQL)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count timeline photos: %w", err)
	}

	query := fmt.Sprintf(`
SELECT id, relative_path, original_filename, media_type, file_size,
       width, height, taken_at, file_modified_at, created_at, updated_at
FROM photos
%s
ORDER BY (CASE WHEN taken_at IS NOT NULL THEN 0 ELSE 1 END) ASC,
         taken_at DESC,
         file_modified_at DESC,
         id ASC
LIMIT ? OFFSET ?`, whereSQL)

	queryArgs := make([]interface{}, len(args), len(args)+2)
	copy(queryArgs, args)
	queryArgs = append(queryArgs, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query timeline photos: %w", err)
	}
	defer rows.Close()

	photos := make([]models.Photo, 0, limit)
	for rows.Next() {
		var p models.Photo
		if err := rows.Scan(
			&p.ID, &p.RelativePath, &p.OriginalFilename, &p.MediaType, &p.FileSize,
			&p.Width, &p.Height, &p.TakenAt, &p.FileModifiedAt, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan timeline photo: %w", err)
		}
		photos = append(photos, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error during timeline photos iteration: %w", err)
	}

	return photos, total, nil
}

// GetAdjacentPhotos returns previous and next photo IDs relative to photoID within the specified collection context.
// collectionType can be: "timeline", "folder", "album", or empty/default (all photos).
func (r *SQLitePhotoRepository) GetAdjacentPhotos(ctx context.Context, id string, collectionType string, collectionID string) (prevID *string, nextID *string, err error) {
	var query string
	var args []interface{}

	switch collectionType {
	case "album":
		query = `
WITH ordered AS (
    SELECT p.id,
           LAG(p.id) OVER (ORDER BY pa.added_at DESC, p.id ASC) AS prev_id,
           LEAD(p.id) OVER (ORDER BY pa.added_at DESC, p.id ASC) AS next_id
    FROM photos p
    INNER JOIN photo_albums pa ON pa.photo_id = p.id
    WHERE pa.album_id = ?
)
SELECT prev_id, next_id FROM ordered WHERE id = ?`
		args = []interface{}{collectionID, id}

	case "folder":
		normPath := cleanFolderPath(collectionID)
		if normPath == "" {
			query = `
WITH ordered AS (
    SELECT id,
           LAG(id) OVER (ORDER BY (CASE WHEN taken_at IS NOT NULL THEN 0 ELSE 1 END) ASC, taken_at DESC, file_modified_at DESC, id ASC) AS prev_id,
           LEAD(id) OVER (ORDER BY (CASE WHEN taken_at IS NOT NULL THEN 0 ELSE 1 END) ASC, taken_at DESC, file_modified_at DESC, id ASC) AS next_id
    FROM photos
    WHERE INSTR(relative_path, '/') = 0
)
SELECT prev_id, next_id FROM ordered WHERE id = ?`
			args = []interface{}{id}
		} else {
			prefix := normPath + "/"
			query = `
WITH ordered AS (
    SELECT id,
           LAG(id) OVER (ORDER BY (CASE WHEN taken_at IS NOT NULL THEN 0 ELSE 1 END) ASC, taken_at DESC, file_modified_at DESC, id ASC) AS prev_id,
           LEAD(id) OVER (ORDER BY (CASE WHEN taken_at IS NOT NULL THEN 0 ELSE 1 END) ASC, taken_at DESC, file_modified_at DESC, id ASC) AS next_id
    FROM photos
    WHERE SUBSTR(relative_path, 1, ?) = ? AND INSTR(SUBSTR(relative_path, ?), '/') = 0
)
SELECT prev_id, next_id FROM ordered WHERE id = ?`
			args = []interface{}{len(prefix), prefix, len(prefix) + 1, id}
		}

	case "timeline":
		fallthrough
	default:
		query = `
WITH ordered AS (
    SELECT id,
           LAG(id) OVER (
               ORDER BY (CASE WHEN taken_at IS NOT NULL THEN 0 ELSE 1 END) ASC,
                        taken_at DESC,
                        file_modified_at DESC,
                        id ASC
           ) AS prev_id,
           LEAD(id) OVER (
               ORDER BY (CASE WHEN taken_at IS NOT NULL THEN 0 ELSE 1 END) ASC,
                        taken_at DESC,
                        file_modified_at DESC,
                        id ASC
           ) AS next_id
    FROM photos
)
SELECT prev_id, next_id FROM ordered WHERE id = ?`
		args = []interface{}{id}
	}

	var pID, nID sql.NullString
	err = r.db.QueryRowContext(ctx, query, args...).Scan(&pID, &nID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, fmt.Errorf("failed to query adjacent photos: %w", err)
	}

	if pID.Valid && pID.String != "" {
		s := pID.String
		prevID = &s
	}
	if nID.Valid && nID.String != "" {
		s := nID.String
		nextID = &s
	}

	return prevID, nextID, nil
}
