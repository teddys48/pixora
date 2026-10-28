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

func TestExtractExifDetailsSafe(t *testing.T) {
	// 1. Plain image without EXIF returns nil details without error
	jpgData := createTestJPEG(100, 100)
	details, err := media.ExtractExifDetails(bytes.NewReader(jpgData))
	if err != nil {
		t.Fatalf("ExtractExifDetails on non-EXIF jpeg failed: %v", err)
	}
	if details != nil && (details.Camera != "" || details.ISO != nil) {
		t.Errorf("Expected empty details on non-EXIF image, got %+v", details)
	}

	// 2. Corrupt data returns nil or error safely without panic
	corrupt := []byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x10, 'E', 'x', 'i', 'f', 0x00, 0x00}
	_, _ = media.ExtractExifDetails(bytes.NewReader(corrupt))

	// 3. ParseEXIFSafe on empty slice
	takenAt, orient := media.ParseEXIFSafe(nil)
	if takenAt != nil || orient != 1 {
		t.Errorf("Expected nil and 1 for empty slice, got %v, %d", takenAt, orient)
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

	thumbSvc := media.NewThumbnailService(store)
	ctx := context.Background()

	// 1. Landscape 800x600 (4:3) -> should scale to 400x300
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

	p := &models.Photo{
		ID:               "thumb-test-1",
		RelativePath:     origRel,
		OriginalFilename: "test_orig.jpg",
		MediaType:        "image/jpeg",
		FileSize:         int64(len(jpgBytes)),
		FileModifiedAt:   time.Now().UTC(),
	}

	thumbPath, err := thumbSvc.GetOrGenerate(ctx, p)
	if err != nil {
		t.Fatalf("Failed to generate thumbnail: %v", err)
	}

	info, err := os.Stat(thumbPath)
	if err != nil || info.Size() == 0 {
		t.Fatalf("Thumbnail not created or empty: %v", err)
	}

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
	if thumbCfg.Width != 400 || thumbCfg.Height != 300 {
		t.Errorf("Expected landscape thumbnail dimensions 400x300, got %dx%d", thumbCfg.Width, thumbCfg.Height)
	}

	// 2. Portrait 600x800 (3:4) -> should scale to 300x400
	portRel := "originals/test_portrait.jpg"
	portAbs, _ := store.ResolvePath(portRel)
	portBytes := createTestJPEG(600, 800)
	_ = os.WriteFile(portAbs, portBytes, 0644)

	pPort := &models.Photo{
		ID:               "thumb-test-portrait",
		RelativePath:     portRel,
		OriginalFilename: "test_portrait.jpg",
		MediaType:        "image/jpeg",
		FileSize:         int64(len(portBytes)),
		FileModifiedAt:   time.Now().UTC(),
	}
	portThumbPath, err := thumbSvc.GetOrGenerate(ctx, pPort)
	if err != nil {
		t.Fatalf("Failed to generate portrait thumbnail: %v", err)
	}
	portThumbData, _ := os.ReadFile(portThumbPath)
	portCfg, _, _ := image.DecodeConfig(bytes.NewReader(portThumbData))
	if portCfg.Width != 300 || portCfg.Height != 400 {
		t.Errorf("Expected portrait thumbnail dimensions 300x400, got %dx%d", portCfg.Width, portCfg.Height)
	}

	// 3. Panoramic 1200x400 (3:1) -> should scale to 400x133
	panoRel := "originals/test_pano.jpg"
	panoAbs, _ := store.ResolvePath(panoRel)
	panoBytes := createTestJPEG(1200, 400)
	_ = os.WriteFile(panoAbs, panoBytes, 0644)

	pPano := &models.Photo{
		ID:               "thumb-test-pano",
		RelativePath:     panoRel,
		OriginalFilename: "test_pano.jpg",
		MediaType:        "image/jpeg",
		FileSize:         int64(len(panoBytes)),
		FileModifiedAt:   time.Now().UTC(),
	}
	panoThumbPath, err := thumbSvc.GetOrGenerate(ctx, pPano)
	if err != nil {
		t.Fatalf("Failed to generate pano thumbnail: %v", err)
	}
	panoThumbData, _ := os.ReadFile(panoThumbPath)
	panoCfg, _, _ := image.DecodeConfig(bytes.NewReader(panoThumbData))
	if panoCfg.Width != 400 || panoCfg.Height != 133 {
		t.Errorf("Expected panoramic thumbnail dimensions 400x133, got %dx%d", panoCfg.Width, panoCfg.Height)
	}

	// 4. Repeated call should return cached path without error
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
