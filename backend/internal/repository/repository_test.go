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

func TestPhotoRepositoryFolderQueries(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := repository.NewSQLitePhotoRepository(db)
	ctx := context.Background()

	t1 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC)
	t4 := time.Date(2026, 1, 4, 10, 0, 0, 0, time.UTC)
	t5 := time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC)
	t6 := time.Date(2026, 1, 6, 10, 0, 0, 0, time.UTC)
	t7 := time.Date(2026, 1, 7, 10, 0, 0, 0, time.UTC)

	photos := []*models.Photo{
		{ID: "p-root-1", RelativePath: "root1.jpg", OriginalFilename: "root1.jpg", MediaType: "image/jpeg", FileSize: 100, TakenAt: &t1},
		{ID: "p-root-2", RelativePath: "root2.jpg", OriginalFilename: "root2.jpg", MediaType: "image/jpeg", FileSize: 200, TakenAt: &t2},
		{ID: "p-pic-1", RelativePath: "Pictures/pic1.jpg", OriginalFilename: "pic1.jpg", MediaType: "image/jpeg", FileSize: 300, TakenAt: &t3},
		{ID: "p-anime-1", RelativePath: "Pictures/Anime/anime1.jpg", OriginalFilename: "anime1.jpg", MediaType: "image/jpeg", FileSize: 400, TakenAt: &t4},
		{ID: "p-anime-2", RelativePath: "Pictures/Anime/anime2.jpg", OriginalFilename: "anime2.jpg", MediaType: "image/jpeg", FileSize: 500, TakenAt: &t5},
		{ID: "p-movie-1", RelativePath: "Pictures/Anime/Movies/movie1.jpg", OriginalFilename: "movie1.jpg", MediaType: "image/jpeg", FileSize: 600, TakenAt: &t6},
		{ID: "p-other-1", RelativePath: "Other/other.jpg", OriginalFilename: "other.jpg", MediaType: "image/jpeg", FileSize: 700, TakenAt: &t7},
	}

	for _, p := range photos {
		if err := repo.Create(ctx, p); err != nil {
			t.Fatalf("Failed to create photo %s: %v", p.ID, err)
		}
	}

	// 1. Root folder photos (direct)
	rootList, rootTotal, err := repo.ListByFolder(ctx, "", 0, 10, "date", "desc")
	if err != nil {
		t.Fatalf("ListByFolder(root) failed: %v", err)
	}
	if rootTotal != 2 || len(rootList) != 2 {
		t.Errorf("Expected 2 direct photos in root, got total=%d, len=%d", rootTotal, len(rootList))
	}
	if rootList[0].ID != "p-root-2" || rootList[1].ID != "p-root-1" {
		t.Errorf("Root list ordering mismatch: got %+v", rootList)
	}

	// 2. Direct photos in "Pictures" (should only contain pic1.jpg, not anime1 or movie1)
	picList, picTotal, err := repo.ListByFolder(ctx, "Pictures", 0, 10, "date", "desc")
	if err != nil {
		t.Fatalf("ListByFolder('Pictures') failed: %v", err)
	}
	if picTotal != 1 || len(picList) != 1 || picList[0].ID != "p-pic-1" {
		t.Errorf("Expected 1 direct photo in Pictures ('p-pic-1'), got total=%d, photos=%+v", picTotal, picList)
	}

	// 3. Direct photos in "Pictures/Anime" (anime1, anime2, but NOT movie1)
	animeList, animeTotal, err := repo.ListByFolder(ctx, "Pictures/Anime", 0, 10, "date", "desc")
	if err != nil {
		t.Fatalf("ListByFolder('Pictures/Anime') failed: %v", err)
	}
	if animeTotal != 2 || len(animeList) != 2 {
		t.Errorf("Expected 2 direct photos in Anime, got total=%d, len=%d", animeTotal, len(animeList))
	}

	// 4. Direct photos in "Pictures/Anime/Movies"
	movieList, movieTotal, err := repo.ListByFolder(ctx, "Pictures/Anime/Movies", 0, 10, "date", "desc")
	if err != nil {
		t.Fatalf("ListByFolder('Pictures/Anime/Movies') failed: %v", err)
	}
	if movieTotal != 1 || len(movieList) != 1 || movieList[0].ID != "p-movie-1" {
		t.Errorf("Expected 1 photo in Movies, got total=%d, photos=%+v", movieTotal, movieList)
	}

	// 5. CountByFolder direct vs recursive
	directPicCount, err := repo.CountByFolder(ctx, "Pictures", false)
	if err != nil || directPicCount != 1 {
		t.Errorf("CountByFolder('Pictures', direct) = %d, err: %v; want 1", directPicCount, err)
	}
	recPicCount, err := repo.CountByFolder(ctx, "Pictures", true)
	if err != nil || recPicCount != 4 {
		t.Errorf("CountByFolder('Pictures', recursive) = %d, err: %v; want 4", recPicCount, err)
	}

	directRootCount, err := repo.CountByFolder(ctx, "", false)
	if err != nil || directRootCount != 2 {
		t.Errorf("CountByFolder('', direct) = %d, err: %v; want 2", directRootCount, err)
	}
	recRootCount, err := repo.CountByFolder(ctx, "", true)
	if err != nil || recRootCount != 7 {
		t.Errorf("CountByFolder('', recursive) = %d, err: %v; want 7", recRootCount, err)
	}

	// 6. Cover photo retrieval
	coverPic, err := repo.GetFolderCoverPhoto(ctx, "Pictures")
	if err != nil {
		t.Fatalf("GetFolderCoverPhoto('Pictures') failed: %v", err)
	}
	if coverPic == nil || coverPic.ID != "p-movie-1" {
		t.Errorf("Expected cover photo 'p-movie-1' for Pictures, got: %+v", coverPic)
	}

	coverEmpty, err := repo.GetFolderCoverPhoto(ctx, "NonExistent")
	if err != nil || coverEmpty != nil {
		t.Errorf("Expected nil cover for nonexistent folder, got %+v, err: %v", coverEmpty, err)
	}
}

func TestPhotoRepositoryTimelineAndAdjacent(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := repository.NewSQLitePhotoRepository(db)
	ctx := context.Background()

	t1 := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	mod1 := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	mod2 := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	mod3 := time.Date(2026, 10, 26, 15, 0, 0, 0, time.UTC)
	mod4 := time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)

	_ = repo.Create(ctx, &models.Photo{
		ID:               "p1",
		RelativePath:     "originals/p1.jpg",
		OriginalFilename: "p1.jpg",
		MediaType:        "image/jpeg",
		FileSize:         1000,
		TakenAt:          &t1,
		FileModifiedAt:   mod1,
	})
	_ = repo.Create(ctx, &models.Photo{
		ID:               "p2",
		RelativePath:     "originals/p2.jpg",
		OriginalFilename: "p2.jpg",
		MediaType:        "image/jpeg",
		FileSize:         1000,
		TakenAt:          &t2,
		FileModifiedAt:   mod2,
	})
	_ = repo.Create(ctx, &models.Photo{
		ID:               "p3",
		RelativePath:     "originals/p3.jpg",
		OriginalFilename: "p3.jpg",
		MediaType:        "image/jpeg",
		FileSize:         1000,
		TakenAt:          nil,
		FileModifiedAt:   mod3,
	})
	_ = repo.Create(ctx, &models.Photo{
		ID:               "p4",
		RelativePath:     "originals/p4.jpg",
		OriginalFilename: "p4.jpg",
		MediaType:        "image/jpeg",
		FileSize:         1000,
		TakenAt:          nil,
		FileModifiedAt:   mod4,
	})

	// 1. Verify timeline ordering:
	// Expected: p1 (dated, newest), p2 (dated, older), p3 (undated, mod 10-26), p4 (undated, mod 10-15)
	timelinePhotos, total, err := repo.ListTimeline(ctx, 0, 10)
	if err != nil {
		t.Fatalf("ListTimeline failed: %v", err)
	}
	if total != 4 || len(timelinePhotos) != 4 {
		t.Fatalf("Expected 4 photos in timeline, got total=%d len=%d", total, len(timelinePhotos))
	}
	expectedOrder := []string{"p1", "p2", "p3", "p4"}
	for i, expected := range expectedOrder {
		if timelinePhotos[i].ID != expected {
			t.Errorf("Timeline index %d: expected %s, got %s", i, expected, timelinePhotos[i].ID)
		}
	}

	// 2. Pagination test
	page1, _, _ := repo.ListTimeline(ctx, 0, 2)
	if len(page1) != 2 || page1[0].ID != "p1" || page1[1].ID != "p2" {
		t.Errorf("Pagination page 1 unexpected: %+v", page1)
	}
	page2, _, _ := repo.ListTimeline(ctx, 2, 2)
	if len(page2) != 2 || page2[0].ID != "p3" || page2[1].ID != "p4" {
		t.Errorf("Pagination page 2 unexpected: %+v", page2)
	}

	// 3. Adjacent navigation in timeline
	prev, next, err := repo.GetAdjacentPhotos(ctx, "p1", "timeline", "")
	if err != nil {
		t.Fatalf("GetAdjacentPhotos p1 failed: %v", err)
	}
	if prev != nil {
		t.Errorf("Expected nil prev for first photo p1, got %v", *prev)
	}
	if next == nil || *next != "p2" {
		t.Errorf("Expected next 'p2' for p1, got %v", next)
	}

	prev, next, err = repo.GetAdjacentPhotos(ctx, "p2", "timeline", "")
	if err != nil {
		t.Fatalf("GetAdjacentPhotos p2 failed: %v", err)
	}
	if prev == nil || *prev != "p1" {
		t.Errorf("Expected prev 'p1' for p2, got %v", prev)
	}
	if next == nil || *next != "p3" {
		t.Errorf("Expected next 'p3' for p2, got %v", next)
	}

	prev, next, err = repo.GetAdjacentPhotos(ctx, "p4", "timeline", "")
	if err != nil {
		t.Fatalf("GetAdjacentPhotos p4 failed: %v", err)
	}
	if prev == nil || *prev != "p3" {
		t.Errorf("Expected prev 'p3' for p4, got %v", prev)
	}
	if next != nil {
		t.Errorf("Expected nil next for last photo p4, got %v", *next)
	}

	// For non-existent photo: ErrNotFound
	_, _, err = repo.GetAdjacentPhotos(ctx, "nonexistent", "timeline", "")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Expected ErrNotFound for nonexistent adjacent query, got %v", err)
	}
}

func TestPhotoRepositorySearchAndFilter(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := repository.NewSQLitePhotoRepository(db)
	albumRepo := repository.NewSQLiteAlbumRepository(db)
	ctx := context.Background()

	// Create test album
	album := &models.Album{
		ID:          "album-vacation",
		Name:        "Vacation 2026",
		Description: "Trip to mountains and beach",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := albumRepo.Create(ctx, album); err != nil {
		t.Fatalf("Failed to create test album: %v", err)
	}

	d1 := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 6, 15, 15, 30, 0, 0, time.UTC)
	d3 := time.Date(2026, 7, 20, 8, 45, 0, 0, time.UTC)
	m1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	m2 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	// Photos fixture
	photos := []*models.Photo{
		{
			ID:               "p-mountain-1",
			RelativePath:     "Travel/Mountains/Everest_Summit.jpg",
			OriginalFilename: "Everest_Summit.jpg",
			MediaType:        "image/jpeg",
			FileSize:         500000,
			TakenAt:          &d1,
			FileModifiedAt:   m1,
		},
		{
			ID:               "p-mountain-2",
			RelativePath:     "Travel/Mountains/Fuji_Sunset.png",
			OriginalFilename: "Fuji_Sunset.png",
			MediaType:        "image/png",
			FileSize:         800000,
			TakenAt:          &d2,
			FileModifiedAt:   m2,
		},
		{
			ID:               "p-beach-1",
			RelativePath:     "Travel/Beach/Hawaii_Coast.jpg",
			OriginalFilename: "Hawaii_Coast.jpg",
			MediaType:        "image/jpeg",
			FileSize:         300000,
			TakenAt:          &d3,
			FileModifiedAt:   m1,
		},
		{
			ID:               "p-special-wildcard",
			RelativePath:     "Special/100%_pure_gold_final.jpg",
			OriginalFilename: "100%_pure_gold_final.jpg",
			MediaType:        "image/jpeg",
			FileSize:         150000,
			TakenAt:          nil, // Undated
			FileModifiedAt:   m2,
		},
		{
			ID:               "p-undated-doc",
			RelativePath:     "Documents/Scan_001.webp",
			OriginalFilename: "Scan_001.webp",
			MediaType:        "image/webp",
			FileSize:         100000,
			TakenAt:          nil, // Undated
			FileModifiedAt:   m1,
		},
	}

	for _, p := range photos {
		if err := repo.Create(ctx, p); err != nil {
			t.Fatalf("Failed to create fixture photo %s: %v", p.ID, err)
		}
	}

	// Add p-mountain-1 and p-beach-1 to album
	_, err := albumRepo.AddPhotos(ctx, album.ID, []string{"p-mountain-1", "p-beach-1"})
	if err != nil {
		t.Fatalf("Failed to add photos to album: %v", err)
	}

	// 1. Filename search: exact & substring case-insensitive
	res, total, err := repo.SearchAndFilter(ctx, repository.PhotoFilter{Query: "everest"})
	if err != nil || total != 1 || len(res) != 1 || res[0].ID != "p-mountain-1" {
		t.Errorf("Filename search 'everest' failed: total=%d, photos=%+v, err=%v", total, res, err)
	}

	// Case-insensitivity: "FUJI" matching "Fuji_Sunset.png"
	res, total, err = repo.SearchAndFilter(ctx, repository.PhotoFilter{Query: "FUJI"})
	if err != nil || total != 1 || len(res) != 1 || res[0].ID != "p-mountain-2" {
		t.Errorf("Case-insensitive search 'FUJI' failed: total=%d, photos=%+v, err=%v", total, res, err)
	}

	// 2. Folder path search in query
	res, total, err = repo.SearchAndFilter(ctx, repository.PhotoFilter{Query: "Travel/Beach"})
	if err != nil || total != 1 || len(res) != 1 || res[0].ID != "p-beach-1" {
		t.Errorf("Folder path search failed: total=%d, photos=%+v, err=%v", total, res, err)
	}

	// 3. Literal wildcard characters (% and _)
	res, total, err = repo.SearchAndFilter(ctx, repository.PhotoFilter{Query: "100%"})
	if err != nil || total != 1 || len(res) != 1 || res[0].ID != "p-special-wildcard" {
		t.Errorf("Wildcard search '100%%' failed: total=%d, photos=%+v, err=%v", total, res, err)
	}

	res, total, err = repo.SearchAndFilter(ctx, repository.PhotoFilter{Query: "_pure_"})
	if err != nil || total != 1 || len(res) != 1 || res[0].ID != "p-special-wildcard" {
		t.Errorf("Wildcard search '_pure_' failed: total=%d, photos=%+v, err=%v", total, res, err)
	}

	// 4. Empty and whitespace-only query returns all photos
	res, total, err = repo.SearchAndFilter(ctx, repository.PhotoFilter{Query: "   "})
	if err != nil || total != 5 || len(res) != 5 {
		t.Errorf("Whitespace query should return all 5 photos: total=%d, err=%v", total, err)
	}

	// 5. SQL Injection string resistance
	res, total, err = repo.SearchAndFilter(ctx, repository.PhotoFilter{Query: "' OR '1'='1"})
	if err != nil || total != 0 {
		t.Errorf("SQL injection attempt should safely return 0 results: total=%d, err=%v", total, err)
	}

	// 6. Date range filtering on verified capture date (taken_at)
	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 30, 23, 59, 59, 0, time.UTC)
	res, total, err = repo.SearchAndFilter(ctx, repository.PhotoFilter{StartDate: &start, EndDate: &end, SortBy: "date", SortOrder: "asc"})
	if err != nil || total != 2 || len(res) != 2 {
		t.Fatalf("Date range filter [May-Jun 2026] failed: total=%d, len=%d, err=%v", total, len(res), err)
	}
	if res[0].ID != "p-mountain-1" || res[1].ID != "p-mountain-2" {
		t.Errorf("Expected p-mountain-1 (May) then p-mountain-2 (June), got: %s, %s", res[0].ID, res[1].ID)
	}

	// 7. Undated filter
	isUndatedTrue := true
	res, total, err = repo.SearchAndFilter(ctx, repository.PhotoFilter{IsUndated: &isUndatedTrue})
	if err != nil || total != 2 || len(res) != 2 {
		t.Errorf("IsUndated=true filter failed: total=%d, len=%d, err=%v", total, len(res), err)
	}

	isUndatedFalse := false
	res, total, err = repo.SearchAndFilter(ctx, repository.PhotoFilter{IsUndated: &isUndatedFalse})
	if err != nil || total != 3 || len(res) != 3 {
		t.Errorf("IsUndated=false filter failed: total=%d, len=%d, err=%v", total, len(res), err)
	}

	// 8. Physical folder filtering (recursive vs direct)
	res, total, err = repo.SearchAndFilter(ctx, repository.PhotoFilter{Folder: "Travel", Recursive: true})
	if err != nil || total != 3 || len(res) != 3 {
		t.Errorf("Folder 'Travel' recursive failed: total=%d, len=%d, err=%v", total, len(res), err)
	}

	res, total, err = repo.SearchAndFilter(ctx, repository.PhotoFilter{Folder: "Travel", Recursive: false})
	if err != nil || total != 0 {
		t.Errorf("Folder 'Travel' non-recursive should have 0 direct photos: total=%d, err=%v", total, err)
	}

	res, total, err = repo.SearchAndFilter(ctx, repository.PhotoFilter{Folder: "Travel/Mountains", Recursive: false})
	if err != nil || total != 2 {
		t.Errorf("Folder 'Travel/Mountains' non-recursive failed: total=%d, err=%v", total, err)
	}

	// 9. Virtual album filtering
	res, total, err = repo.SearchAndFilter(ctx, repository.PhotoFilter{AlbumID: album.ID})
	if err != nil || total != 2 || len(res) != 2 {
		t.Errorf("Album filter failed: total=%d, len=%d, err=%v", total, len(res), err)
	}

	// 10. Media type filtering
	res, total, err = repo.SearchAndFilter(ctx, repository.PhotoFilter{MediaType: "image/png"})
	if err != nil || total != 1 || res[0].ID != "p-mountain-2" {
		t.Errorf("MediaType 'image/png' filter failed: total=%d, err=%v", total, err)
	}

	res, total, err = repo.SearchAndFilter(ctx, repository.PhotoFilter{MediaType: "image/*"})
	if err != nil || total != 5 {
		t.Errorf("MediaType 'image/*' prefix filter failed: total=%d, err=%v", total, err)
	}

	// 11. Sorting tests
	// Date desc: dated newest first (d3 Hawaii, d2 Fuji, d1 Everest), then undated
	res, _, _ = repo.SearchAndFilter(ctx, repository.PhotoFilter{SortBy: "date", SortOrder: "desc"})
	if res[0].ID != "p-beach-1" || res[1].ID != "p-mountain-2" || res[2].ID != "p-mountain-1" {
		t.Errorf("Unexpected date desc order: %s, %s, %s", res[0].ID, res[1].ID, res[2].ID)
	}

	// Date asc: dated oldest first (d1 Everest, d2 Fuji, d3 Hawaii), then undated
	res, _, _ = repo.SearchAndFilter(ctx, repository.PhotoFilter{SortBy: "date", SortOrder: "asc"})
	if res[0].ID != "p-mountain-1" || res[1].ID != "p-mountain-2" || res[2].ID != "p-beach-1" {
		t.Errorf("Unexpected date asc order: %s, %s, %s", res[0].ID, res[1].ID, res[2].ID)
	}

	// Filename asc: 100%_pure..., Everest_Summit..., Fuji_Sunset..., Hawaii_Coast..., Scan_001...
	res, _, _ = repo.SearchAndFilter(ctx, repository.PhotoFilter{SortBy: "filename", SortOrder: "asc"})
	if res[0].ID != "p-special-wildcard" || res[1].ID != "p-mountain-1" {
		t.Errorf("Unexpected filename asc order: %s, %s", res[0].ID, res[1].ID)
	}

	// FileSize desc: Fuji (800k), Everest (500k), Hawaii (300k), Special (150k), Scan (100k)
	res, _, _ = repo.SearchAndFilter(ctx, repository.PhotoFilter{SortBy: "size", SortOrder: "desc"})
	if res[0].ID != "p-mountain-2" || res[1].ID != "p-mountain-1" || res[2].ID != "p-beach-1" {
		t.Errorf("Unexpected size desc order: %s, %s, %s", res[0].ID, res[1].ID, res[2].ID)
	}

	// 12. Combined search + filter + sort + pagination
	res, total, err = repo.SearchAndFilter(ctx, repository.PhotoFilter{
		Query:     "Travel",
		MediaType: "image/jpeg",
		AlbumID:   album.ID,
		SortBy:    "date",
		SortOrder: "desc",
		Limit:     1,
		Offset:    0,
	})
	if err != nil || total != 2 || len(res) != 1 || res[0].ID != "p-beach-1" {
		t.Errorf("Combined filter page 1 failed: total=%d, len=%d, res[0]=%+v, err=%v", total, len(res), res, err)
	}

	// Page 2
	res, total, err = repo.SearchAndFilter(ctx, repository.PhotoFilter{
		Query:     "Travel",
		MediaType: "image/jpeg",
		AlbumID:   album.ID,
		SortBy:    "date",
		SortOrder: "desc",
		Limit:     1,
		Offset:    1,
	})
	if err != nil || total != 2 || len(res) != 1 || res[0].ID != "p-mountain-1" {
		t.Errorf("Combined filter page 2 failed: total=%d, len=%d, res[0]=%+v, err=%v", total, len(res), res, err)
	}

	// 13. Timeline with filter
	tlRes, tlTotal, tlErr := repo.ListTimelineWithFilter(ctx, repository.PhotoFilter{
		Folder: "Travel/Mountains",
	})
	if tlErr != nil || tlTotal != 2 || len(tlRes) != 2 {
		t.Errorf("ListTimelineWithFilter failed: total=%d, len=%d, err=%v", tlTotal, len(tlRes), tlErr)
	}
	if tlRes[0].ID != "p-mountain-2" || tlRes[1].ID != "p-mountain-1" {
		t.Errorf("Timeline chronological order mismatch: %s, %s", tlRes[0].ID, tlRes[1].ID)
	}
}
