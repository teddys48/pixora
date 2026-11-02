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
	GetPhotoIDs(ctx context.Context, albumID string) ([]string, error)
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
			return fmt.Errorf("%w: album with name %q already exists", ErrConflict, a.Name)
		}
		return fmt.Errorf("failed to insert album: %w", err)
	}

	return nil
}

// GetByID retrieves an album by its ID, with accurate photoCount, coverPhotoId, and coverThumbnailUrl.
func (r *SQLiteAlbumRepository) GetByID(ctx context.Context, id string) (*models.Album, error) {
	query := `
SELECT a.id, a.name, a.description, a.created_at, a.updated_at,
       COUNT(pa.photo_id) AS photo_count,
       (
           SELECT p.id
           FROM photos p
           INNER JOIN photo_albums pa2 ON pa2.photo_id = p.id
           WHERE pa2.album_id = a.id
           ORDER BY pa2.added_at DESC, p.id ASC
           LIMIT 1
       ) AS cover_photo_id
FROM albums a
LEFT JOIN photo_albums pa ON pa.album_id = a.id
WHERE a.id = ?
GROUP BY a.id`

	var a models.Album
	var coverPhotoID sql.NullString
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&a.ID, &a.Name, &a.Description, &a.CreatedAt, &a.UpdatedAt,
		&a.PhotoCount, &coverPhotoID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get album by id: %w", err)
	}

	if coverPhotoID.Valid && coverPhotoID.String != "" {
		cID := coverPhotoID.String
		a.CoverPhotoID = &cID
		thumbURL := fmt.Sprintf("/api/photos/%s/thumbnail", cID)
		a.CoverThumbnailURL = &thumbURL
	}

	return &a, nil
}

// List returns all albums ordered alphabetically by name (case-insensitive) with accurate photo count and cover thumbnail.
func (r *SQLiteAlbumRepository) List(ctx context.Context) ([]models.Album, error) {
	query := `
SELECT a.id, a.name, a.description, a.created_at, a.updated_at,
       COUNT(pa.photo_id) AS photo_count,
       (
           SELECT p.id
           FROM photos p
           INNER JOIN photo_albums pa2 ON pa2.photo_id = p.id
           WHERE pa2.album_id = a.id
           ORDER BY pa2.added_at DESC, p.id ASC
           LIMIT 1
       ) AS cover_photo_id
FROM albums a
LEFT JOIN photo_albums pa ON pa.album_id = a.id
GROUP BY a.id
ORDER BY a.name COLLATE NOCASE ASC, a.created_at DESC`

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
			&a.ID, &a.Name, &a.Description, &a.CreatedAt, &a.UpdatedAt,
			&a.PhotoCount, &coverPhotoID,
		); err != nil {
			return nil, fmt.Errorf("failed to scan album: %w", err)
		}
		if coverPhotoID.Valid && coverPhotoID.String != "" {
			cID := coverPhotoID.String
			a.CoverPhotoID = &cID
			thumbURL := fmt.Sprintf("/api/photos/%s/thumbnail", cID)
			a.CoverThumbnailURL = &thumbURL
		}
		albums = append(albums, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during albums iteration: %w", err)
	}

	return albums, nil
}

// Update updates an album's name and description.
func (r *SQLiteAlbumRepository) Update(ctx context.Context, a *models.Album) error {
	a.UpdatedAt = time.Now().UTC()

	query := `UPDATE albums SET name = ?, description = ?, updated_at = ? WHERE id = ?`
	res, err := r.db.ExecContext(ctx, query, a.Name, a.Description, a.UpdatedAt, a.ID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			return fmt.Errorf("%w: album with name %q already exists", ErrConflict, a.Name)
		}
		return fmt.Errorf("failed to update album: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// Delete removes an album record by ID.
func (r *SQLiteAlbumRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM albums WHERE id = ?`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete album: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// AddPhoto associates a photo with an album.
func (r *SQLiteAlbumRepository) AddPhoto(ctx context.Context, photoID, albumID string) error {
	var dummy string
	if err := r.db.QueryRowContext(ctx, "SELECT id FROM albums WHERE id = ?", albumID).Scan(&dummy); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to verify album existence: %w", err)
	}

	if err := r.db.QueryRowContext(ctx, "SELECT id FROM photos WHERE id = ?", photoID).Scan(&dummy); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to verify photo existence: %w", err)
	}

	now := time.Now().UTC()
	query := `
INSERT INTO photo_albums (photo_id, album_id, added_at)
VALUES (?, ?, ?)
ON CONFLICT(photo_id, album_id) DO NOTHING`

	_, err := r.db.ExecContext(ctx, query, photoID, albumID, now)
	if err != nil {
		return fmt.Errorf("failed to add photo to album: %w", err)
	}

	return nil
}

// AddPhotos associates multiple photos with an album within a single atomic database transaction.
func (r *SQLiteAlbumRepository) AddPhotos(ctx context.Context, albumID string, photoIDs []string) (int, error) {
	if len(photoIDs) == 0 {
		return 0, nil
	}

	var dummy string
	if err := r.db.QueryRowContext(ctx, "SELECT id FROM albums WHERE id = ?", albumID).Scan(&dummy); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, fmt.Errorf("failed to verify album existence: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO photo_albums (photo_id, album_id, added_at)
VALUES (?, ?, ?)
ON CONFLICT(photo_id, album_id) DO NOTHING`)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC()
	addedCount := 0

	for _, pID := range photoIDs {
		var validID string
		if err := tx.QueryRowContext(ctx, "SELECT id FROM photos WHERE id = ?", pID).Scan(&validID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return 0, ErrNotFound
			}
			return 0, fmt.Errorf("failed to verify photo %s: %w", pID, err)
		}

		res, err := stmt.ExecContext(ctx, pID, albumID, now)
		if err != nil {
			return 0, fmt.Errorf("failed to add photo %s to album: %w", pID, err)
		}
		if rows, _ := res.RowsAffected(); rows > 0 {
			addedCount++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit add photos transaction: %w", err)
	}

	return addedCount, nil
}

// RemovePhoto removes a photo association from an album.
func (r *SQLiteAlbumRepository) RemovePhoto(ctx context.Context, photoID, albumID string) error {
	query := `DELETE FROM photo_albums WHERE photo_id = ? AND album_id = ?`
	res, err := r.db.ExecContext(ctx, query, photoID, albumID)
	if err != nil {
		return fmt.Errorf("failed to remove photo from album: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// GetPhotos returns paginated photos that belong to an album, ordered by added_at DESC.
func (r *SQLiteAlbumRepository) GetPhotos(ctx context.Context, albumID string, offset, limit int) ([]models.Photo, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

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
       p.width, p.height, p.duration, p.video_codec, p.audio_codec, p.container_format,
       p.thumbnail_status, p.taken_at, p.file_modified_at, p.created_at, p.updated_at
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
			&p.Width, &p.Height, &p.Duration, &p.VideoCodec, &p.AudioCodec, &p.ContainerFormat,
			&p.ThumbnailStatus, &p.TakenAt, &p.FileModifiedAt, &p.CreatedAt, &p.UpdatedAt,
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
       p.width, p.height, p.duration, p.video_codec, p.audio_codec, p.container_format,
       p.thumbnail_status, p.taken_at, p.file_modified_at, p.created_at, p.updated_at,
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
			&p.Width, &p.Height, &p.Duration, &p.VideoCodec, &p.AudioCodec, &p.ContainerFormat,
			&p.ThumbnailStatus, &p.TakenAt, &p.FileModifiedAt, &p.CreatedAt, &p.UpdatedAt,
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

// GetPhotoIDs returns all photo IDs associated with the specified album.
func (r *SQLiteAlbumRepository) GetPhotoIDs(ctx context.Context, albumID string) ([]string, error) {
	var dummy string
	if err := r.db.QueryRowContext(ctx, "SELECT id FROM albums WHERE id = ?", albumID).Scan(&dummy); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to verify album: %w", err)
	}

	query := `SELECT photo_id FROM photo_albums WHERE album_id = ? ORDER BY added_at DESC, photo_id ASC`
	rows, err := r.db.QueryContext(ctx, query, albumID)
	if err != nil {
		return nil, fmt.Errorf("failed to query album photo IDs: %w", err)
	}
	defer rows.Close()

	photoIDs := make([]string, 0)
	for rows.Next() {
		var pid string
		if err := rows.Scan(&pid); err != nil {
			return nil, fmt.Errorf("failed to scan photo ID: %w", err)
		}
		photoIDs = append(photoIDs, pid)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during album photo IDs iteration: %w", err)
	}
	return photoIDs, nil
}
