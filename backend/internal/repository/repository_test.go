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

	// Create 2 photos
	p1 := &models.Photo{
		ID:               "photo-100",
		RelativePath:     "originals/album_photo1.jpg",
		OriginalFilename: "album_photo1.jpg",
		MediaType:        "image/jpeg",
		FileSize:         1000,
	}
	p2 := &models.Photo{
		ID:               "photo-101",
		RelativePath:     "originals/album_photo2.jpg",
		OriginalFilename: "album_photo2.jpg",
		MediaType:        "image/jpeg",
		FileSize:         2000,
	}
	if err := photoRepo.Create(ctx, p1); err != nil {
		t.Fatalf("Failed to create p1: %v", err)
	}
	if err := photoRepo.Create(ctx, p2); err != nil {
		t.Fatalf("Failed to create p2: %v", err)
	}

	// 1. Create album
	album := &models.Album{
		ID:          "album-100",
		Name:        "Summer Trip",
		Description: "Photos from summer vacation",
	}
	if err := albumRepo.Create(ctx, album); err != nil {
		t.Fatalf("Failed to create album: %v", err)
	}

	// 2. Duplicate album name (case-insensitive) should return ErrConflict
	dupAlbum := &models.Album{
		ID:   "album-dup",
		Name: "summer trip",
	}
	if err := albumRepo.Create(ctx, dupAlbum); !errors.Is(err, repository.ErrConflict) {
		t.Errorf("Expected ErrConflict on duplicate case-insensitive album name, got: %v", err)
	}

	// 3. List albums (initial photo count should be 0, no cover thumbnail)
	albums, err := albumRepo.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list albums: %v", err)
	}
	if len(albums) != 1 || albums[0].PhotoCount != 0 || albums[0].CoverPhotoID != nil {
		t.Errorf("Expected 1 album with photoCount 0 and nil cover, got %+v", albums)
	}

	// 4. Batch add photos via AddPhotos
	added, err := albumRepo.AddPhotos(ctx, album.ID, []string{p1.ID, p2.ID})
	if err != nil {
		t.Fatalf("Failed to add photos to album: %v", err)
	}
	if added != 2 {
		t.Errorf("Expected 2 added photos, got %d", added)
	}

	// 5. Batch add duplicate photo associations: should not error and return 0 newly added
	addedDup, err := albumRepo.AddPhotos(ctx, album.ID, []string{p1.ID})
	if err != nil {
		t.Fatalf("Failed on duplicate AddPhotos: %v", err)
	}
	if addedDup != 0 {
		t.Errorf("Expected 0 added for duplicate, got %d", addedDup)
	}

	// 6. Non-existent photo ID in AddPhotos returns ErrNotFound
	_, err = albumRepo.AddPhotos(ctx, album.ID, []string{"nonexistent-photo"})
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Expected ErrNotFound for non-existent photo ID in AddPhotos, got: %v", err)
	}

	// 7. Verify photoCount and cover thumbnail updated in GetByID
	fetchedAlbum, err := albumRepo.GetByID(ctx, album.ID)
	if err != nil {
		t.Fatalf("Failed to get album: %v", err)
	}
	if fetchedAlbum.PhotoCount != 2 {
		t.Errorf("Expected photoCount 2, got %d", fetchedAlbum.PhotoCount)
	}
	if fetchedAlbum.CoverPhotoID == nil || fetchedAlbum.CoverThumbnailURL == nil {
		t.Fatalf("Expected non-nil cover photo and thumbnail URL, got nil")
	}

	// 8. Get photos in album with pagination
	albumPhotos, total, err := albumRepo.GetPhotos(ctx, album.ID, 0, 10)
	if err != nil {
		t.Fatalf("Failed to get photos in album: %v", err)
	}
	if total != 2 || len(albumPhotos) != 2 {
		t.Errorf("Unexpected album photos: %+v, total: %d", albumPhotos, total)
	}

	// 9. Get photos for nonexistent album returns ErrNotFound
	_, _, err = albumRepo.GetPhotos(ctx, "nonexistent-album", 0, 10)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Expected ErrNotFound when getting photos for nonexistent album, got: %v", err)
	}

	// 10. Update/Rename album
	fetchedAlbum.Name = "Winter Trip"
	fetchedAlbum.Description = "Updated winter photos"
	if err := albumRepo.Update(ctx, fetchedAlbum); err != nil {
		t.Fatalf("Failed to update album: %v", err)
	}
	renamedAlbum, err := albumRepo.GetByID(ctx, album.ID)
	if err != nil {
		t.Fatalf("Failed to get renamed album: %v", err)
	}
	if renamedAlbum.Name != "Winter Trip" || renamedAlbum.Description != "Updated winter photos" {
		t.Errorf("Rename mismatch: got %+v", renamedAlbum)
	}

	// 11. Rename collision should return ErrConflict
	secondAlbum := &models.Album{
		ID:   "album-200",
		Name: "Autumn Trip",
	}
	if err := albumRepo.Create(ctx, secondAlbum); err != nil {
		t.Fatalf("Failed to create second album: %v", err)
	}
	secondAlbum.Name = "winter trip" // Case-insensitive collision with first album
	if err := albumRepo.Update(ctx, secondAlbum); !errors.Is(err, repository.ErrConflict) {
		t.Errorf("Expected ErrConflict on renaming to duplicate name, got: %v", err)
	}

	// 12. Remove photo from album
	if err := albumRepo.RemovePhoto(ctx, p1.ID, album.ID); err != nil {
		t.Fatalf("Failed to remove photo from album: %v", err)
	}

	// Verify original photo in photos table is STILL present
	originalPhoto, err := photoRepo.GetByID(ctx, p1.ID)
	if err != nil || originalPhoto == nil {
		t.Fatalf("Removing photo from album must NOT delete the original photo, got err: %v", err)
	}

	// Remove again should return ErrNotFound
	if err := albumRepo.RemovePhoto(ctx, p1.ID, album.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Expected ErrNotFound on second remove, got: %v", err)
	}

	// 13. Delete album
	if err := albumRepo.Delete(ctx, album.ID); err != nil {
		t.Fatalf("Failed to delete album: %v", err)
	}
	_, err = albumRepo.GetByID(ctx, album.ID)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Expected ErrNotFound for deleted album, got: %v", err)
	}

	// Verify original photos are STILL present in photos repository after album deletion
	p2Fetched, err := photoRepo.GetByID(ctx, p2.ID)
	if err != nil || p2Fetched == nil {
		t.Errorf("Deleting album must NOT delete original photo records, got err: %v", err)
	}
}
