package media_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pixora/backend/internal/media"
	"pixora/backend/internal/models"
	"pixora/backend/internal/storage"
)

func createTestJPEG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 120, 255})
		}
	}
	buf := new(bytes.Buffer)
	_ = jpeg.Encode(buf, img, &jpeg.Options{Quality: 80})
	return buf.Bytes()
}

func createTestPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	buf := new(bytes.Buffer)
	_ = png.Encode(buf, img)
	return buf.Bytes()
}

func createTestGIF(w, h int) []byte {
	img := image.NewPaletted(image.Rect(0, 0, w, h), color.Palette{
		color.RGBA{0, 0, 0, 255},
		color.RGBA{255, 255, 255, 255},
	})
	buf := new(bytes.Buffer)
	_ = gif.Encode(buf, img, nil)
	return buf.Bytes()
}

func TestExtractMetadata(t *testing.T) {
	// Test valid JPEG
	jpgData := createTestJPEG(640, 480)
	info, err := media.ExtractMetadata(bytes.NewReader(jpgData))
	if err != nil {
		t.Fatalf("Failed to extract JPEG metadata: %v", err)
	}
	if info.MediaType != "image/jpeg" || info.Width != 640 || info.Height != 480 {
		t.Errorf("Unexpected JPEG info: %+v", info)
	}

	// Test valid PNG
	pngData := createTestPNG(320, 240)
	infoPng, err := media.ExtractMetadata(bytes.NewReader(pngData))
	if err != nil {
		t.Fatalf("Failed to extract PNG metadata: %v", err)
	}
	if infoPng.MediaType != "image/png" || infoPng.Width != 320 || infoPng.Height != 240 {
		t.Errorf("Unexpected PNG info: %+v", infoPng)
	}

	// Test valid GIF
	gifData := createTestGIF(100, 100)
	infoGif, err := media.ExtractMetadata(bytes.NewReader(gifData))
	if err != nil {
		t.Fatalf("Failed to extract GIF metadata: %v", err)
	}
	if infoGif.MediaType != "image/gif" || infoGif.Width != 100 || infoGif.Height != 100 {
		t.Errorf("Unexpected GIF info: %+v", infoGif)
	}

	// Test unsupported format
	fakePdf := []byte("%PDF-1.4 corrupt text document")
	_, err = media.ExtractMetadata(bytes.NewReader(fakePdf))
	if err == nil {
		t.Error("Expected error for non-image format, got nil")
	}

	// Test corrupt header
	corruptJpg := []byte{0xFF, 0xD8, 0xFF, 0x00, 0x00}
	_, err = media.ExtractMetadata(bytes.NewReader(corruptJpg))
	if err == nil {
		t.Error("Expected error for truncated/corrupt JPEG, got nil")
	}
}

func TestIsSupportedExtension(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"photo.jpg", true},
		{"photo.JPEG", true},
		{"IMAGE.png", true},
		{"graphic.webp", true},
		{"animation.gif", true},
		{"scan.tiff", true},
		{"scan.TIF", true},
		{"document.pdf", false},
		{"script.sh", false},
		{"image.exe", false},
		{"noext", false},
	}

	for _, tc := range tests {
		got := media.IsSupportedExtension(tc.filename)
		if got != tc.expected {
			t.Errorf("IsSupportedExtension(%q) = %v, expected %v", tc.filename, got, tc.expected)
		}
	}
}

func TestOrientationTransform(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 20))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})

	// Test rotate 90 (orientation 6)
	r90 := media.ApplyOrientation(img, 6)
	if r90.Bounds().Dx() != 20 || r90.Bounds().Dy() != 10 {
		t.Errorf("Expected rotated bounds 20x10, got %dx%d", r90.Bounds().Dx(), r90.Bounds().Dy())
	}

	// Test rotate 180 (orientation 3)
	r180 := media.ApplyOrientation(img, 3)
	if r180.Bounds().Dx() != 10 || r180.Bounds().Dy() != 20 {
		t.Errorf("Expected 10x20, got %dx%d", r180.Bounds().Dx(), r180.Bounds().Dy())
	}

	// Test normal (orientation 1)
	r1 := media.ApplyOrientation(img, 1)
	if r1.Bounds().Dx() != 10 || r1.Bounds().Dy() != 20 {
		t.Errorf("Expected 10x20, got %dx%d", r1.Bounds().Dx(), r1.Bounds().Dy())
	}
}

func TestThumbnailService(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pixora_thumb_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := storage.New(tempDir)
	if err != nil {
		t.Fatalf("Failed to init storage: %v", err)
	}

	// Write test original image
	origRel := "originals/test_orig.jpg"
	origAbs, err := store.ResolvePath(origRel)
	if err != nil {
		t.Fatalf("Failed to resolve orig path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(origAbs), 0755); err != nil {
		t.Fatalf("Failed to mkdir originals: %v", err)
	}

	jpgBytes := createTestJPEG(800, 600)
	if err := os.WriteFile(origAbs, jpgBytes, 0644); err != nil {
		t.Fatalf("Failed to write original image: %v", err)
	}

	thumbSvc := media.NewThumbnailService(store)
	ctx := context.Background()

	p := &models.Photo{
		ID:               "thumb-test-1",
		RelativePath:     origRel,
		OriginalFilename: "test_orig.jpg",
		MediaType:        "image/jpeg",
		FileSize:         int64(len(jpgBytes)),
		FileModifiedAt:   time.Now().UTC(),
	}

	// 1. Generate thumbnail
	thumbPath, err := thumbSvc.GetOrGenerate(ctx, p)
	if err != nil {
		t.Fatalf("Failed to generate thumbnail: %v", err)
	}

	info, err := os.Stat(thumbPath)
	if err != nil || info.Size() == 0 {
		t.Fatalf("Thumbnail not created or empty: %v", err)
	}

	// 2. Decode generated thumbnail and verify dimensions
	thumbData, err := os.ReadFile(thumbPath)
	if err != nil {
		t.Fatalf("Failed to read thumbnail file: %v", err)
	}
	thumbCfg, format, err := image.DecodeConfig(bytes.NewReader(thumbData))
	if err != nil {
		t.Fatalf("Failed to decode thumbnail config: %v", err)
	}
	if format != "jpeg" {
		t.Errorf("Expected thumbnail format jpeg, got %s", format)
	}
	// Aspect ratio of 800x600 is 4:3. In 400x400 box -> 400x300
	if thumbCfg.Width != 400 || thumbCfg.Height != 300 {
		t.Errorf("Expected thumbnail dimensions 400x300, got %dx%d", thumbCfg.Width, thumbCfg.Height)
	}

	// 3. Repeated call should return cached path without error
	secondPath, err := thumbSvc.GetOrGenerate(ctx, p)
	if err != nil {
		t.Fatalf("Failed on repeated thumbnail call: %v", err)
	}
	if secondPath != thumbPath {
		t.Errorf("Expected cached path %s, got %s", thumbPath, secondPath)
	}
}

func TestThumbnailServiceReadOnly(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pixora_thumb_ro_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := storage.New(tempDir)
	if err != nil {
		t.Fatalf("Failed to init storage: %v", err)
	}

	// Create original photo
	origRel := "originals/photo_ro.jpg"
	origAbs, err := store.ResolvePath(origRel)
	if err != nil {
		t.Fatalf("Failed to resolve path: %v", err)
	}
	_ = os.MkdirAll(filepath.Dir(origAbs), 0755)
	_ = os.WriteFile(origAbs, createTestJPEG(200, 200), 0644)

	// Make directory read-only
	_ = os.Chmod(tempDir, 0555)
	defer func() {
		_ = os.Chmod(tempDir, 0755)
	}()

	thumbSvc := media.NewThumbnailService(store)
	p := &models.Photo{
		ID:           "ro-photo-1",
		RelativePath: origRel,
	}

	_, err = thumbSvc.GetOrGenerate(context.Background(), p)
	if err == nil {
		t.Error("Expected error when generating thumbnail in read-only storage, got nil")
	}
}
