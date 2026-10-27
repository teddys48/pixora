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
	Delete(ctx context.Context, id string) error
	AddPhoto(ctx context.Context, photoID, albumID string) error
	RemovePhoto(ctx context.Context, photoID, albumID string) error
	GetPhotos(ctx context.Context, albumID string, offset, limit int) ([]models.Photo, int, error)
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
			return fmt.Errorf("%w: album with id %q already exists", ErrConflict, a.ID)
		}
		return fmt.Errorf("failed to insert album: %w", err)
	}

	return nil
}

// GetByID retrieves an album by its ID along with its current photo count.
func (r *SQLiteAlbumRepository) GetByID(ctx context.Context, id string) (*models.Album, error) {
	query := `
SELECT a.id, a.name, a.description, a.created_at, a.updated_at,
       (SELECT COUNT(*) FROM photo_albums pa WHERE pa.album_id = a.id) AS photo_count
FROM albums a
WHERE a.id = ?`

	var a models.Album
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&a.ID, &a.Name, &a.Description, &a.CreatedAt, &a.UpdatedAt, &a.PhotoCount,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get album by id: %w", err)
	}

	return &a, nil
}

// List returns all albums with their respective photo counts.
func (r *SQLiteAlbumRepository) List(ctx context.Context) ([]models.Album, error) {
	query := `
SELECT a.id, a.name, a.description, a.created_at, a.updated_at,
       (SELECT COUNT(*) FROM photo_albums pa WHERE pa.album_id = a.id) AS photo_count
FROM albums a
ORDER BY a.name ASC, a.id ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list albums: %w", err)
	}
	defer rows.Close()

	albums := make([]models.Album, 0)
	for rows.Next() {
		var a models.Album
		if err := rows.Scan(
			&a.ID, &a.Name, &a.Description, &a.CreatedAt, &a.UpdatedAt, &a.PhotoCount,
		); err != nil {
			return nil, fmt.Errorf("failed to scan album: %w", err)
		}
		albums = append(albums, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during album iteration: %w", err)
	}

	return albums, nil
}

// Delete removes an album record. Associated photo_album mappings are removed via ON DELETE CASCADE.
func (r *SQLiteAlbumRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM albums WHERE id = ?", id)
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

// AddPhoto associates a photo with an album.
func (r *SQLiteAlbumRepository) AddPhoto(ctx context.Context, photoID, albumID string) error {
	query := `INSERT INTO photo_albums (photo_id, album_id, added_at) VALUES (?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, photoID, albumID, time.Now().UTC())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "PRIMARY KEY") {
			return fmt.Errorf("%w: photo already in album", ErrConflict)
		}
		if strings.Contains(err.Error(), "FOREIGN KEY") || strings.Contains(err.Error(), "foreign key") {
			return fmt.Errorf("%w: photo or album does not exist", ErrNotFound)
		}
		return fmt.Errorf("failed to add photo to album: %w", err)
	}
	return nil
}

// RemovePhoto disassociates a photo from an album.
func (r *SQLiteAlbumRepository) RemovePhoto(ctx context.Context, photoID, albumID string) error {
	query := `DELETE FROM photo_albums WHERE photo_id = ? AND album_id = ?`
	res, err := r.db.ExecContext(ctx, query, photoID, albumID)
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
	return nil
}

// GetPhotos returns paginated photos that belong to an album.
func (r *SQLiteAlbumRepository) GetPhotos(ctx context.Context, albumID string, offset, limit int) ([]models.Photo, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
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
