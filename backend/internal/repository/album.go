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

// AlbumRepository defines data access methods for albums.
type AlbumRepository interface {
	Create(ctx context.Context, a *models.Album) error
	GetByID(ctx context.Context, id string) (*models.Album, error)
	List(ctx context.Context) ([]models.Album, error)
	Update(ctx context.Context, a *models.Album) error
	Delete(ctx context.Context, id string) error
	AddPhoto(ctx context.Context, photoID, albumID string) error
	AddPhotos(ctx context.Context, albumID string, photoIDs []string) (int, error)
	RemovePhoto(ctx context.Context, photoID, albumID string) error
	GetPhotos(ctx context.Context, albumID string, offset, limit int) ([]models.Photo, int, error)
	GetPhotosCursor(ctx context.Context, albumID string, limit int, cursor *CursorData) (photos []models.Photo, total int, hasMore bool, nextCursor *CursorData, err error)
}

// SQLiteAlbumRepository implements AlbumRepository using SQLite.
type SQLiteAlbumRepository struct {
	db *database.DB
}

// NewSQLiteAlbumRepository creates a new SQLiteAlbumRepository.
func NewSQLiteAlbumRepository(db *database.DB) *SQLiteAlbumRepository {
	return &SQLiteAlbumRepository{db: db}
}

// Create inserts a new album record.
func (r *SQLiteAlbumRepository) Create(ctx context.Context, a *models.Album) error {
	now := time.Now().UTC()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	if a.UpdatedAt.IsZero() {
		a.UpdatedAt = now
	}

	query := `INSERT INTO albums (id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, a.ID, a.Name, a.Description, a.CreatedAt, a.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			return fmt.Errorf("%w: album with name %q or id %q already exists", ErrConflict, a.Name, a.ID)
		}
		return fmt.Errorf("failed to insert album: %w", err)
	}

	return nil
}

// GetByID retrieves an album by its ID along with its current photo count and cover thumbnail.
func (r *SQLiteAlbumRepository) GetByID(ctx context.Context, id string) (*models.Album, error) {
	query := `
SELECT a.id, a.name, a.description, a.created_at, a.updated_at,
       (SELECT COUNT(*) FROM photo_albums pa WHERE pa.album_id = a.id) AS photo_count,
       (SELECT pa.photo_id FROM photo_albums pa WHERE pa.album_id = a.id ORDER BY pa.added_at DESC, pa.photo_id ASC LIMIT 1) AS cover_photo_id
FROM albums a
WHERE a.id = ?`

	var a models.Album
	var coverPhotoID sql.NullString
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&a.ID, &a.Name, &a.Description, &a.CreatedAt, &a.UpdatedAt, &a.PhotoCount, &coverPhotoID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get album by id: %w", err)
	}

	if coverPhotoID.Valid && coverPhotoID.String != "" {
		photoID := coverPhotoID.String
		a.CoverPhotoID = &photoID
		thumbURL := fmt.Sprintf("/api/photos/%s/thumbnail", photoID)
		a.CoverThumbnailURL = &thumbURL
	}

	return &a, nil
}

// List returns all albums with their respective photo counts and cover thumbnails.
func (r *SQLiteAlbumRepository) List(ctx context.Context) ([]models.Album, error) {
	query := `
SELECT a.id, a.name, a.description, a.created_at, a.updated_at,
       (SELECT COUNT(*) FROM photo_albums pa WHERE pa.album_id = a.id) AS photo_count,
       (SELECT pa.photo_id FROM photo_albums pa WHERE pa.album_id = a.id ORDER BY pa.added_at DESC, pa.photo_id ASC LIMIT 1) AS cover_photo_id
FROM albums a
ORDER BY a.name COLLATE NOCASE ASC, a.id ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list albums: %w", err)
	}
	defer rows.Close()

	albums := make([]models.Album, 0)
	for rows.Next() {
		var a models.Album
		var coverPhotoID sql.NullString
		if err := rows.Scan(
			&a.ID, &a.Name, &a.Description, &a.CreatedAt, &a.UpdatedAt, &a.PhotoCount, &coverPhotoID,
		); err != nil {
			return nil, fmt.Errorf("failed to scan album: %w", err)
		}
		if coverPhotoID.Valid && coverPhotoID.String != "" {
			photoID := coverPhotoID.String
			a.CoverPhotoID = &photoID
			thumbURL := fmt.Sprintf("/api/photos/%s/thumbnail", photoID)
			a.CoverThumbnailURL = &thumbURL
		}
		albums = append(albums, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during album iteration: %w", err)
	}

	return albums, nil
}

// Update modifies an album's name, description, and updated_at timestamp.
func (r *SQLiteAlbumRepository) Update(ctx context.Context, a *models.Album) error {
	now := time.Now().UTC()
	a.UpdatedAt = now

	query := `UPDATE albums SET name = ?, description = ?, updated_at = ? WHERE id = ?`
	res, err := r.db.ExecContext(ctx, query, a.Name, a.Description, a.UpdatedAt, a.ID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			return fmt.Errorf("%w: album with name %q already exists", ErrConflict, a.Name)
		}
		return fmt.Errorf("failed to update album: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}

	return nil
}

// Delete removes an album by ID. Photo associations in photo_albums cascade-delete automatically.
func (r *SQLiteAlbumRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM albums WHERE id = ?`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete album: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}

	return nil
}

// AddPhoto associates a photo with an album and updates the album's updated_at timestamp.
func (r *SQLiteAlbumRepository) AddPhoto(ctx context.Context, photoID, albumID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	now := time.Now().UTC()
	query := `INSERT OR IGNORE INTO photo_albums (photo_id, album_id, added_at) VALUES (?, ?, ?)`
	_, err = tx.ExecContext(ctx, query, photoID, albumID, now)
	if err != nil {
		return fmt.Errorf("failed to associate photo with album: %w", err)
	}

	_, err = tx.ExecContext(ctx, "UPDATE albums SET updated_at = ? WHERE id = ?", now, albumID)
	if err != nil {
		return fmt.Errorf("failed to update album timestamp: %w", err)
	}

	return tx.Commit()
}

// AddPhotos associates multiple photos with an album inside a single database transaction.
// Duplicate associations are ignored, and nonexistent photos or albums return ErrNotFound.
func (r *SQLiteAlbumRepository) AddPhotos(ctx context.Context, albumID string, photoIDs []string) (int, error) {
	if len(photoIDs) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Verify album exists
	var dummy string
	err = tx.QueryRowContext(ctx, "SELECT id FROM albums WHERE id = ?", albumID).Scan(&dummy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, fmt.Errorf("failed to verify album: %w", err)
	}

	// Verify each photo exists
	for _, pid := range photoIDs {
		err = tx.QueryRowContext(ctx, "SELECT id FROM photos WHERE id = ?", pid).Scan(&dummy)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return 0, fmt.Errorf("%w: photo %q does not exist", ErrNotFound, pid)
			}
			return 0, fmt.Errorf("failed to verify photo %q: %w", pid, err)
		}
	}

	now := time.Now().UTC()
	stmt, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO photo_albums (photo_id, album_id, added_at) VALUES (?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	addedCount := 0
	for _, pid := range photoIDs {
		res, err := stmt.ExecContext(ctx, pid, albumID, now)
		if err != nil {
			return 0, fmt.Errorf("failed to associate photo %q: %w", pid, err)
		}
		affected, _ := res.RowsAffected()
		if affected > 0 {
			addedCount++
		}
	}

	_, err = tx.ExecContext(ctx, "UPDATE albums SET updated_at = ? WHERE id = ?", now, albumID)
	if err != nil {
		return 0, fmt.Errorf("failed to update album timestamp: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return addedCount, nil
}

// RemovePhoto disassociates a photo from an album and updates the album's updated_at timestamp.
func (r *SQLiteAlbumRepository) RemovePhoto(ctx context.Context, photoID, albumID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Verify album exists
	var dummy string
	if err := tx.QueryRowContext(ctx, "SELECT id FROM albums WHERE id = ?", albumID).Scan(&dummy); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to verify album: %w", err)
	}

	query := `DELETE FROM photo_albums WHERE photo_id = ? AND album_id = ?`
	res, err := tx.ExecContext(ctx, query, photoID, albumID)
	if err != nil {
		return fmt.Errorf("failed to remove photo from album: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}

	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, "UPDATE albums SET updated_at = ? WHERE id = ?", now, albumID)
	if err != nil {
		return fmt.Errorf("failed to update album timestamp: %w", err)
	}

	return tx.Commit()
}

// GetPhotos returns paginated photos that belong to an album using offset.
func (r *SQLiteAlbumRepository) GetPhotos(ctx context.Context, albumID string, offset, limit int) ([]models.Photo, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	// Verify album exists
	var dummy string
	if err := r.db.QueryRowContext(ctx, "SELECT id FROM albums WHERE id = ?", albumID).Scan(&dummy); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, 0, ErrNotFound
		}
		return nil, 0, fmt.Errorf("failed to verify album: %w", err)
	}

	var total int
	countQuery := `SELECT COUNT(*) FROM photo_albums WHERE album_id = ?`
	if err := r.db.QueryRowContext(ctx, countQuery, albumID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count album photos: %w", err)
	}

	query := `
SELECT p.id, p.relative_path, p.original_filename, p.media_type, p.file_size,
       p.width, p.height, p.taken_at, p.file_modified_at, p.created_at, p.updated_at
FROM photos p
INNER JOIN photo_albums pa ON pa.photo_id = p.id
WHERE pa.album_id = ?
ORDER BY pa.added_at DESC, p.id ASC
LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, albumID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query album photos: %w", err)
	}
	defer rows.Close()

	photos := make([]models.Photo, 0, limit)
	for rows.Next() {
		var p models.Photo
		if err := rows.Scan(
			&p.ID, &p.RelativePath, &p.OriginalFilename, &p.MediaType, &p.FileSize,
			&p.Width, &p.Height, &p.TakenAt, &p.FileModifiedAt, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan album photo: %w", err)
		}
		photos = append(photos, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error during album photos iteration: %w", err)
	}

	return photos, total, nil
}

// GetPhotosCursor returns paginated photos that belong to an album using keyset cursor pagination without using SQL OFFSET.
func (r *SQLiteAlbumRepository) GetPhotosCursor(ctx context.Context, albumID string, limit int, cursor *CursorData) ([]models.Photo, int, bool, *CursorData, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	// Verify album exists
	var dummy string
	if err := r.db.QueryRowContext(ctx, "SELECT id FROM albums WHERE id = ?", albumID).Scan(&dummy); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, 0, false, nil, ErrNotFound
		}
		return nil, 0, false, nil, fmt.Errorf("failed to verify album: %w", err)
	}

	var total int
	countQuery := `SELECT COUNT(*) FROM photo_albums WHERE album_id = ?`
	if err := r.db.QueryRowContext(ctx, countQuery, albumID).Scan(&total); err != nil {
		return nil, 0, false, nil, fmt.Errorf("failed to count album photos: %w", err)
	}

	whereClause := "pa.album_id = ?"
	args := []interface{}{albumID}

	if cursor != nil && cursor.ID != "" && cursor.TimeVal != nil {
		whereClause += " AND (pa.added_at < ? OR (pa.added_at = ? AND p.id > ?))"
		t := cursor.TimeVal.UTC()
		args = append(args, t, t, cursor.ID)
	}

	query := fmt.Sprintf(`
SELECT p.id, p.relative_path, p.original_filename, p.media_type, p.file_size,
       p.width, p.height, p.taken_at, p.file_modified_at, p.created_at, p.updated_at,
       pa.added_at
FROM photos p
INNER JOIN photo_albums pa ON pa.photo_id = p.id
WHERE %s
ORDER BY pa.added_at DESC, p.id ASC
LIMIT ?`, whereClause)

	args = append(args, limit+1)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, false, nil, fmt.Errorf("failed to query album photos: %w", err)
	}
	defer rows.Close()

	photos := make([]models.Photo, 0, limit+1)
	addedAts := make([]time.Time, 0, limit+1)
	for rows.Next() {
		var p models.Photo
		var addedAt time.Time
		if err := rows.Scan(
			&p.ID, &p.RelativePath, &p.OriginalFilename, &p.MediaType, &p.FileSize,
			&p.Width, &p.Height, &p.TakenAt, &p.FileModifiedAt, &p.CreatedAt, &p.UpdatedAt,
			&addedAt,
		); err != nil {
			return nil, 0, false, nil, fmt.Errorf("failed to scan album photo: %w", err)
		}
		photos = append(photos, p)
		addedAts = append(addedAts, addedAt)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, false, nil, fmt.Errorf("error during album photos iteration: %w", err)
	}

	hasMore := len(photos) > limit
	if hasMore {
		photos = photos[:limit]
		addedAts = addedAts[:limit]
	}

	var nextCursor *CursorData
	if hasMore && len(photos) > 0 {
		lastPhoto := photos[len(photos)-1]
		t := addedAts[len(addedAts)-1].UTC()
		nextCursor = &CursorData{
			ID:      lastPhoto.ID,
			TimeVal: &t,
		}
	}

	return photos, total, hasMore, nextCursor, nil
}
