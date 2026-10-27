package repository_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pixora/backend/internal/database"
	"pixora/backend/internal/models"
	"pixora/backend/internal/repository"
)

func setupTestDB(t *testing.T) (*database.DB, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "pixora_repo_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "repo_test.db")
	db, err := database.Open(dbPath)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to open test database: %v", err)
	}

	cleanup := func() {
		_ = db.Close()
		_ = os.RemoveAll(tempDir)
	}

	return db, cleanup
}

func TestPhotoRepositoryCRUDAndListing(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := repository.NewSQLitePhotoRepository(db)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	width := 1920
	height := 1080
	p1 := &models.Photo{
		ID:               "photo-1",
		RelativePath:     "originals/2026/10/img_1.jpg",
		OriginalFilename: "img_1.jpg",
		MediaType:        "image/jpeg",
		FileSize:         102400,
		Width:            &width,
		Height:           &height,
		TakenAt:          &now,
		FileModifiedAt:   now,
	}

	// Create photo
	if err := repo.Create(ctx, p1); err != nil {
		t.Fatalf("Failed to create photo: %v", err)
	}

	// Duplicate create should return ErrConflict
	if err := repo.Create(ctx, p1); !errors.Is(err, repository.ErrConflict) {
		t.Errorf("Expected ErrConflict on duplicate photo create, got: %v", err)
	}

	// Get by ID
	fetched, err := repo.GetByID(ctx, "photo-1")
	if err != nil {
		t.Fatalf("Failed to get photo by ID: %v", err)
	}
	if fetched.ID != "photo-1" || fetched.RelativePath != p1.RelativePath {
		t.Errorf("Photo data mismatch: got %+v", fetched)
	}

	// Update photo
	newWidth := 2048
	fetched.Width = &newWidth
	fetched.FileSize = 105000
	if err := repo.Update(ctx, fetched); err != nil {
		t.Fatalf("Failed to update photo: %v", err)
	}
	updated, err := repo.GetByID(ctx, "photo-1")
	if err != nil {
		t.Fatalf("Failed to get updated photo: %v", err)
	}
	if updated.FileSize != 105000 || *updated.Width != 2048 {
		t.Errorf("Updated photo mismatch: got %+v", updated)
	}

	// Update non-existent photo
	nonExistent := &models.Photo{ID: "not-found"}
	if err := repo.Update(ctx, nonExistent); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Expected ErrNotFound on non-existent update, got: %v", err)
	}

	// Get non-existent
	_, err = repo.GetByID(ctx, "does-not-exist")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Expected ErrNotFound for non-existent photo, got: %v", err)
	}

	// Get by relative path
	byPath, err := repo.GetByRelativePath(ctx, "originals/2026/10/img_1.jpg")
	if err != nil {
		t.Fatalf("Failed to get photo by relative path: %v", err)
	}
	if byPath.ID != "photo-1" {
		t.Errorf("Expected ID 'photo-1', got %s", byPath.ID)
	}

	// Insert additional photos for pagination and sorting
	earlier := now.Add(-2 * time.Hour)
	later := now.Add(2 * time.Hour)

	p2 := &models.Photo{
		ID:               "photo-2",
		RelativePath:     "originals/2026/10/img_2.jpg",
		OriginalFilename: "a_img_2.jpg",
		MediaType:        "image/jpeg",
		FileSize:         204800,
		TakenAt:          &earlier,
		FileModifiedAt:   earlier,
	}
	p3 := &models.Photo{
		ID:               "photo-3",
		RelativePath:     "originals/2026/10/img_3.jpg",
		OriginalFilename: "z_img_3.jpg",
		MediaType:        "image/jpeg",
		FileSize:         51200,
		TakenAt:          &later,
		FileModifiedAt:   later,
	}
	if err := repo.Create(ctx, p2); err != nil {
		t.Fatalf("Failed to create p2: %v", err)
	}
	if err := repo.Create(ctx, p3); err != nil {
		t.Fatalf("Failed to create p3: %v", err)
	}

	// Count
	count, err := repo.Count(ctx)
	if err != nil || count != 3 {
		t.Errorf("Expected count 3, got %d, err: %v", count, err)
	}

	// List with date-desc: order should be p3 (later), p1 (now), p2 (earlier)
	photos, total, err := repo.List(ctx, 0, 10, "date", "desc")
	if err != nil {
		t.Fatalf("Failed to list photos: %v", err)
	}
	if total != 3 || len(photos) != 3 {
		t.Fatalf("Expected total 3, got total %d, returned %d", total, len(photos))
	}
	if photos[0].ID != "photo-3" || photos[1].ID != "photo-1" || photos[2].ID != "photo-2" {
		t.Errorf("Unexpected sort order: %s, %s, %s", photos[0].ID, photos[1].ID, photos[2].ID)
	}

	// List sorted by modified asc
	photosMod, _, err := repo.List(ctx, 0, 10, "modified", "asc")
	if err != nil {
		t.Fatalf("Failed to list photos by modified: %v", err)
	}
	if photosMod[0].ID != "photo-2" {
		t.Errorf("Expected p2 first for modified asc, got %s", photosMod[0].ID)
	}

	// List with pagination: limit 1, offset 1
	paged, total, err := repo.List(ctx, 1, 1, "date", "desc")
	if err != nil {
		t.Fatalf("Failed to list paged photos: %v", err)
	}
	if total != 3 || len(paged) != 1 || paged[0].ID != "photo-1" {
		t.Errorf("Unexpected paged result: %+v, total: %d", paged, total)
	}

	// Delete photo
	if err := repo.Delete(ctx, "photo-1"); err != nil {
		t.Fatalf("Failed to delete photo: %v", err)
	}
	_, err = repo.GetByID(ctx, "photo-1")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Expected ErrNotFound after deletion, got: %v", err)
	}

	// Delete non-existent photo
	if err := repo.Delete(ctx, "photo-1"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Expected ErrNotFound on deleting non-existent photo, got: %v", err)
	}
}

func TestAlbumRepositoryCRUDAndPhotos(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	albumRepo := repository.NewSQLiteAlbumRepository(db)
	photoRepo := repository.NewSQLitePhotoRepository(db)
	ctx := context.Background()

	// Create photo
	photo := &models.Photo{
		ID:               "photo-100",
		RelativePath:     "originals/album_photo.jpg",
		OriginalFilename: "album_photo.jpg",
		MediaType:        "image/jpeg",
		FileSize:         1000,
	}
	if err := photoRepo.Create(ctx, photo); err != nil {
		t.Fatalf("Failed to create photo: %v", err)
	}

	// Create album
	album := &models.Album{
		ID:          "album-100",
		Name:        "Summer Trip",
		Description: "Photos from summer vacation",
	}
	if err := albumRepo.Create(ctx, album); err != nil {
		t.Fatalf("Failed to create album: %v", err)
	}

	// Duplicate create should return ErrConflict
	if err := albumRepo.Create(ctx, album); !errors.Is(err, repository.ErrConflict) {
		t.Errorf("Expected ErrConflict on duplicate album create, got: %v", err)
	}

	// List albums (initial photo count should be 0)
	albums, err := albumRepo.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list albums: %v", err)
	}
	if len(albums) != 1 || albums[0].PhotoCount != 0 {
		t.Errorf("Expected 1 album with photoCount 0, got %+v", albums)
	}

	// Add photo to album
	if err := albumRepo.AddPhoto(ctx, photo.ID, album.ID); err != nil {
		t.Fatalf("Failed to add photo to album: %v", err)
	}

	// Duplicate add photo should return ErrConflict
	if err := albumRepo.AddPhoto(ctx, photo.ID, album.ID); !errors.Is(err, repository.ErrConflict) {
		t.Errorf("Expected ErrConflict on duplicate photo add, got: %v", err)
	}

	// Adding non-existent photo or album should return ErrNotFound
	if err := albumRepo.AddPhoto(ctx, "invalid-photo", album.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Expected ErrNotFound for invalid photo, got: %v", err)
	}

	// Verify photoCount updated in GetByID
	fetchedAlbum, err := albumRepo.GetByID(ctx, album.ID)
	if err != nil {
		t.Fatalf("Failed to get album: %v", err)
	}
	if fetchedAlbum.PhotoCount != 1 {
		t.Errorf("Expected photoCount 1, got %d", fetchedAlbum.PhotoCount)
	}

	// Get photos in album
	albumPhotos, total, err := albumRepo.GetPhotos(ctx, album.ID, 0, 10)
	if err != nil {
		t.Fatalf("Failed to get photos in album: %v", err)
	}
	if total != 1 || len(albumPhotos) != 1 || albumPhotos[0].ID != photo.ID {
		t.Errorf("Unexpected album photos: %+v, total: %d", albumPhotos, total)
	}

	// Remove photo from album
	if err := albumRepo.RemovePhoto(ctx, photo.ID, album.ID); err != nil {
		t.Fatalf("Failed to remove photo from album: %v", err)
	}

	// Remove again should return ErrNotFound
	if err := albumRepo.RemovePhoto(ctx, photo.ID, album.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Expected ErrNotFound on second remove, got: %v", err)
	}

	// Delete album
	if err := albumRepo.Delete(ctx, album.ID); err != nil {
		t.Fatalf("Failed to delete album: %v", err)
	}
	_, err = albumRepo.GetByID(ctx, album.ID)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Expected ErrNotFound for deleted album, got: %v", err)
	}
}
