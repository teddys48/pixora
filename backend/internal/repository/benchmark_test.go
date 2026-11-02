package repository_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pixora/backend/internal/database"
	"pixora/backend/internal/repository"
)

func setupBenchmarkDB(b *testing.B, numPhotos int) (*database.DB, repository.PhotoRepository, func()) {
	b.Helper()

	tempDir, err := os.MkdirTemp("", "pixora_bench_db_*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "bench.db")
	db, err := database.Open(dbPath)
	if err != nil {
		b.Fatalf("Failed to open database: %v", err)
	}

	repo := repository.NewSQLitePhotoRepository(db)

	// Populate dataset in a single transaction for fast test setup
	tx, err := db.Begin()
	if err != nil {
		b.Fatalf("Failed to start transaction: %v", err)
	}

	stmt, err := tx.Prepare(`
INSERT INTO photos (
    id, relative_path, original_filename, media_type, file_size,
    width, height, duration, video_codec, audio_codec, container_format,
    thumbnail_status, taken_at, file_modified_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		b.Fatalf("Failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	folders := []string{"Vacation/Summer", "Vacation/Winter", "Family/Birthday", "Events/Concert", "Camera"}
	mediaTypes := []string{"image/jpeg", "image/png", "image/webp", "video/mp4"}

	w := 1920
	h := 1080
	dur := 15.5
	vCodec := "h264"
	aCodec := "aac"
	cFormat := "mov,mp4,m4a,3gp,3g2,mj2"
	thumbStatus := "ready"

	for i := 0; i < numPhotos; i++ {
		id := fmt.Sprintf("bench-photo-%06d", i)
		folder := folders[i%len(folders)]
		mType := mediaTypes[i%len(mediaTypes)]
		filename := fmt.Sprintf("IMG_%06d.%s", i, map[string]string{
			"image/jpeg": "jpg",
			"image/png":  "png",
			"image/webp": "webp",
			"video/mp4":  "mp4",
		}[mType])
		relPath := filepath.Join(folder, filename)

		var takenAt *time.Time
		if i%10 != 0 { // 90% dated, 10% undated
			t := baseTime.Add(time.Duration(i) * time.Minute)
			takenAt = &t
		}
		modTime := baseTime.Add(time.Duration(i) * time.Minute)

		var durPtr *float64
		var vCodecPtr, aCodecPtr, cFormatPtr *string
		if mType == "video/mp4" {
			durPtr = &dur
			vCodecPtr = &vCodec
			aCodecPtr = &aCodec
			cFormatPtr = &cFormat
		}

		_, err := stmt.Exec(
			id, relPath, filename, mType, 1024*1024+int64(i*100),
			&w, &h, durPtr, vCodecPtr, aCodecPtr, cFormatPtr,
			thumbStatus, takenAt, modTime, modTime, modTime,
		)
		if err != nil {
			b.Fatalf("Failed to insert synthetic record %d: %v", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		b.Fatalf("Failed to commit synthetic dataset: %v", err)
	}

	cleanup := func() {
		_ = db.Close()
		_ = os.RemoveAll(tempDir)
	}

	return db, repo, cleanup
}

func BenchmarkListCursor(b *testing.B) {
	_, repo, cleanup := setupBenchmarkDB(b, 5000)
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		photos, _, hasMore, nextCursor, err := repo.ListCursor(ctx, 50, "date", "desc", nil)
		if err != nil || len(photos) != 50 || !hasMore || nextCursor == nil {
			b.Fatalf("ListCursor page 1 failed: len=%d, err=%v", len(photos), err)
		}

		// Page 2 using cursor
		photos2, _, _, _, err := repo.ListCursor(ctx, 50, "date", "desc", nextCursor)
		if err != nil || len(photos2) != 50 {
			b.Fatalf("ListCursor page 2 failed: len=%d, err=%v", len(photos2), err)
		}
	}
}

func BenchmarkSearchAndFilter(b *testing.B) {
	_, repo, cleanup := setupBenchmarkDB(b, 5000)
	defer cleanup()

	ctx := context.Background()
	start := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filter := repository.PhotoFilter{
			Query:     "IMG_0001",
			StartDate: &start,
			EndDate:   &end,
			Limit:     50,
			SortBy:    "date",
			SortOrder: "desc",
		}
		photos, total, _, _, err := repo.SearchAndFilterCursor(ctx, filter)
		if err != nil {
			b.Fatalf("SearchAndFilterCursor failed: %v", err)
		}
		_ = photos
		_ = total
	}
}

func BenchmarkTimelineCursor(b *testing.B) {
	_, repo, cleanup := setupBenchmarkDB(b, 5000)
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filter := repository.PhotoFilter{
			Limit: 50,
		}
		photos, _, hasMore, nextCursor, err := repo.ListTimelineCursor(ctx, filter)
		if err != nil || len(photos) != 50 || !hasMore || nextCursor == nil {
			b.Fatalf("ListTimelineCursor page 1 failed: %v", err)
		}
	}
}

func BenchmarkAdjacentPhotos(b *testing.B) {
	_, repo, cleanup := setupBenchmarkDB(b, 5000)
	defer cleanup()

	ctx := context.Background()
	targetID := "bench-photo-002500"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		prevID, nextID, err := repo.GetAdjacentPhotos(ctx, targetID, "timeline", "")
		if err != nil || prevID == nil || nextID == nil {
			b.Fatalf("GetAdjacentPhotos failed: prev=%v, next=%v, err=%v", prevID, nextID, err)
		}
	}
}
