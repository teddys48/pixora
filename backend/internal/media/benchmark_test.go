package media_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"pixora/backend/internal/media"
	"pixora/backend/internal/models"
	"pixora/backend/internal/storage"
)

func BenchmarkThumbnailCached(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "pixora_bench_thumb_*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := storage.New(tempDir)
	if err != nil {
		b.Fatalf("Failed to init storage: %v", err)
	}

	origRel := "originals/bench.jpg"
	origAbs, _ := store.ResolvePath(origRel)
	_ = os.MkdirAll(filepath.Dir(origAbs), 0755)
	_ = os.WriteFile(origAbs, createTestJPEG(400, 300), 0644)

	thumbSvc := media.NewThumbnailService(store)
	p := &models.Photo{
		ID:           "bench-photo-1",
		RelativePath: origRel,
		MediaType:    "image/jpeg",
	}

	ctx := context.Background()
	// Pre-generate thumbnail so it is cached on disk
	_, err = thumbSvc.GetOrGenerate(ctx, p)
	if err != nil {
		b.Fatalf("Failed to pre-generate thumbnail: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path, err := thumbSvc.GetOrGenerate(ctx, p)
		if err != nil || path == "" {
			b.Fatalf("Failed to retrieve cached thumbnail: %v", err)
		}
	}
}

func BenchmarkThumbnailGeneration(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "pixora_bench_thumb_gen_*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := storage.New(tempDir)
	if err != nil {
		b.Fatalf("Failed to init storage: %v", err)
	}

	origRel := "originals/source.jpg"
	origAbs, _ := store.ResolvePath(origRel)
	_ = os.MkdirAll(filepath.Dir(origAbs), 0755)
	_ = os.WriteFile(origAbs, createTestJPEG(800, 600), 0644)

	thumbSvc := media.NewThumbnailService(store)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		p := &models.Photo{
			ID:           models.Photo{}.ID + string(rune('a'+(i%26))) + string(rune('0'+(i%10))),
			RelativePath: origRel,
			MediaType:    "image/jpeg",
		}
		// Ensure thumbnail doesn't exist
		tPath, _ := store.ResolvePath(thumbSvc.ThumbnailRelativePath(p.ID))
		_ = os.Remove(tPath)
		b.StartTimer()

		_, err := thumbSvc.GetOrGenerate(ctx, p)
		if err != nil {
			b.Fatalf("Thumbnail generation failed: %v", err)
		}
	}
}
