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
	"os/exec"
	"path/filepath"
	"testing"

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

func createTestVideoFile(t *testing.T, targetPath string, durationSec int, width, height int) bool {
	t.Helper()
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return false
	}

	cmd := exec.Command(ffmpegPath,
		"-y",
		"-f", "lavfi",
		"-i", "testsrc=duration=1:size=320x240:rate=10",
		"-pix_fmt", "yuv420p",
		targetPath,
	)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func TestExtractMetadata(t *testing.T) {
	// 1. Valid JPEG
	jpgData := createTestJPEG(120, 80)
	info, err := media.ExtractMetadata(bytes.NewReader(jpgData))
	if err != nil {
		t.Fatalf("ExtractMetadata failed for JPEG: %v", err)
	}
	if info.MediaType != "image/jpeg" || info.Width != 120 || info.Height != 80 {
		t.Errorf("Unexpected JPEG info: %+v", info)
	}

	// 2. Valid PNG
	pngData := createTestPNG(64, 48)
	info, err = media.ExtractMetadata(bytes.NewReader(pngData))
	if err != nil {
		t.Fatalf("ExtractMetadata failed for PNG: %v", err)
	}
	if info.MediaType != "image/png" || info.Width != 64 || info.Height != 48 {
		t.Errorf("Unexpected PNG info: %+v", info)
	}

	// 3. Valid GIF
	gifData := createTestGIF(32, 32)
	info, err = media.ExtractMetadata(bytes.NewReader(gifData))
	if err != nil {
		t.Fatalf("ExtractMetadata failed for GIF: %v", err)
	}
	if info.MediaType != "image/gif" || info.Width != 32 || info.Height != 32 {
		t.Errorf("Unexpected GIF info: %+v", info)
	}

	// 4. Unsupported format (plain text)
	plainText := []byte("hello world this is not an image")
	_, err = media.ExtractMetadata(bytes.NewReader(plainText))
	if err == nil {
		t.Error("Expected error for plain text file, got nil")
	}

	// 5. Corrupted header
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
		{"video.mp4", true},
		{"clip.WEBM", true},
		{"movie.mov", true},
		{"show.m4v", true},
		{"feature.mkv", true},
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

func TestIsSupportedVideoExtension(t *testing.T) {
	videoTests := []struct {
		filename string
		expected bool
	}{
		{"video.mp4", true},
		{"video.MP4", true},
		{"clip.webm", true},
		{"movie.mov", true},
		{"movie.MOV", true},
		{"show.m4v", true},
		{"feature.mkv", true},
		{"photo.jpg", false},
		{"image.png", false},
		{"doc.pdf", false},
	}

	for _, tc := range videoTests {
		got := media.IsSupportedVideoExtension(tc.filename)
		if got != tc.expected {
			t.Errorf("IsSupportedVideoExtension(%q) = %v, expected %v", tc.filename, got, tc.expected)
		}
	}
}

func TestVideoSniffingAndDetection(t *testing.T) {
	// MP4 ftyp header
	mp4Header := []byte{
		0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p',
		'i', 's', 'o', 'm', 0x00, 0x00, 0x02, 0x00,
	}
	mime, ext, err := media.SniffVideoMediaType(mp4Header)
	if err != nil || mime != "video/mp4" || ext != ".mp4" {
		t.Errorf("SniffVideoMediaType failed for MP4: mime=%s, ext=%s, err=%v", mime, ext, err)
	}

	// QuickTime ftyp header
	movHeader := []byte{
		0x00, 0x00, 0x00, 0x14, 'f', 't', 'y', 'p',
		'q', 't', ' ', ' ',
	}
	mime, ext, err = media.SniffVideoMediaType(movHeader)
	if err != nil || mime != "video/quicktime" || ext != ".mov" {
		t.Errorf("SniffVideoMediaType failed for QuickTime: mime=%s, ext=%s, err=%v", mime, ext, err)
	}

	// WebM EBML header
	webmHeader := append([]byte{0x1A, 0x45, 0xDF, 0xA3}, []byte("...webm...")...)
	mime, ext, err = media.SniffVideoMediaType(webmHeader)
	if err != nil || mime != "video/webm" || ext != ".webm" {
		t.Errorf("SniffVideoMediaType failed for WebM: mime=%s, ext=%s, err=%v", mime, ext, err)
	}

	// Matroska MKV EBML header
	mkvHeader := append([]byte{0x1A, 0x45, 0xDF, 0xA3}, []byte("...matroska...")...)
	mime, ext, err = media.SniffVideoMediaType(mkvHeader)
	if err != nil || mime != "video/x-matroska" || ext != ".mkv" {
		t.Errorf("SniffVideoMediaType failed for MKV: mime=%s, ext=%s, err=%v", mime, ext, err)
	}

	// Invalid header
	junk := []byte("This is not a video file at all!")
	_, _, err = media.SniffVideoMediaType(junk)
	if err == nil {
		t.Error("Expected error sniffing non-video data, got nil")
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
		t.Errorf("Expected rotated bounds 10x20, got %dx%d", r180.Bounds().Dx(), r180.Bounds().Dy())
	}

	// Test rotate 270 (orientation 8)
	r270 := media.ApplyOrientation(img, 8)
	if r270.Bounds().Dx() != 20 || r270.Bounds().Dy() != 10 {
		t.Errorf("Expected rotated bounds 20x10, got %dx%d", r270.Bounds().Dx(), r270.Bounds().Dy())
	}

	// Test flip horizontal (orientation 2)
	fh := media.ApplyOrientation(img, 2)
	if fh.Bounds().Dx() != 10 || fh.Bounds().Dy() != 20 {
		t.Errorf("Expected flipped bounds 10x20, got %dx%d", fh.Bounds().Dx(), fh.Bounds().Dy())
	}

	// Test orientation 1 (normal, no-op)
	orig := media.ApplyOrientation(img, 1)
	if orig != img {
		t.Error("Expected orientation 1 to return original image pointer")
	}
}

func TestExtractExifDetailsSafe(t *testing.T) {
	// Valid image without EXIF metadata should return (nil, nil) without error
	jpgData := createTestJPEG(100, 100)
	details, err := media.ExtractExifDetails(bytes.NewReader(jpgData))
	if err != nil {
		t.Fatalf("Expected nil error for image without EXIF, got: %v", err)
	}
	if details != nil {
		t.Errorf("Expected nil details for image without EXIF, got: %+v", details)
	}

	// Corrupted data should return (nil, nil) safely without crashing
	corrupted := []byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x10, 'E', 'x', 'i', 'f', 0x00, 0x00}
	details, err = media.ExtractExifDetails(bytes.NewReader(corrupted))
	if err != nil {
		t.Fatalf("Expected safe error handling for malformed EXIF, got: %v", err)
	}
	if details != nil {
		t.Errorf("Expected nil details for malformed EXIF, got: %+v", details)
	}
}

func TestVideoMetadataExtraction(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pixora_video_meta_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	videoPath := filepath.Join(tempDir, "clip.mp4")
	if !createTestVideoFile(t, videoPath, 1, 320, 240) {
		t.Skip("ffmpeg not available or failed to generate test video")
	}

	ctx := context.Background()
	info, err := media.ExtractVideoMetadata(ctx, videoPath)
	if err != nil {
		t.Fatalf("ExtractVideoMetadata failed: %v", err)
	}

	if info.MediaType != "video/mp4" {
		t.Errorf("Expected mediaType video/mp4, got %s", info.MediaType)
	}
	if info.Width != 320 || info.Height != 240 {
		t.Errorf("Expected dimensions 320x240, got %dx%d", info.Width, info.Height)
	}
	if info.Duration <= 0 {
		t.Errorf("Expected duration > 0, got %f", info.Duration)
	}
	if info.VideoCodec != "h264" {
		t.Errorf("Expected codec h264, got %s", info.VideoCodec)
	}

	// Test missing file error
	_, err = media.ExtractVideoMetadata(ctx, filepath.Join(tempDir, "nonexistent.mp4"))
	if err == nil {
		t.Error("Expected error for non-existent video file, got nil")
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

	// 1. Create original photo
	origRel := "originals/photo1.jpg"
	origAbs, err := store.ResolvePath(origRel)
	if err != nil {
		t.Fatalf("Failed to resolve path: %v", err)
	}
	_ = os.MkdirAll(filepath.Dir(origAbs), 0755)
	_ = os.WriteFile(origAbs, createTestJPEG(800, 600), 0644)

	thumbSvc := media.NewThumbnailService(store)
	p := &models.Photo{
		ID:           "test-photo-1",
		RelativePath: origRel,
		MediaType:    "image/jpeg",
	}

	ctx := context.Background()

	// 2. Generate thumbnail
	thumbPath, err := thumbSvc.GetOrGenerate(ctx, p)
	if err != nil {
		t.Fatalf("Failed to generate thumbnail: %v", err)
	}

	// 3. Verify thumbnail file exists on disk and is non-empty
	info, err := os.Stat(thumbPath)
	if err != nil {
		t.Fatalf("Thumbnail file does not exist: %v", err)
	}
	if info.Size() == 0 {
		t.Error("Generated thumbnail is 0 bytes")
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

func TestVideoThumbnailService(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pixora_video_thumb_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := storage.New(tempDir)
	if err != nil {
		t.Fatalf("Failed to init storage: %v", err)
	}

	videoRel := "videos/sample.mp4"
	videoAbs, err := store.ResolvePath(videoRel)
	if err != nil {
		t.Fatalf("Failed to resolve path: %v", err)
	}
	_ = os.MkdirAll(filepath.Dir(videoAbs), 0755)

	if !createTestVideoFile(t, videoAbs, 1, 320, 240) {
		t.Skip("ffmpeg not available or failed to generate test video")
	}

	dur := 1.0
	thumbSvc := media.NewThumbnailService(store)
	p := &models.Photo{
		ID:               "test-video-1",
		RelativePath:     videoRel,
		OriginalFilename: "sample.mp4",
		MediaType:        "video/mp4",
		Duration:         &dur,
	}

	ctx := context.Background()

	// 1. Generate video thumbnail
	thumbPath, err := thumbSvc.GetOrGenerate(ctx, p)
	if err != nil {
		t.Fatalf("Failed to generate video thumbnail: %v", err)
	}

	// 2. Verify thumbnail file exists, is non-empty, and is a valid JPEG image
	info, err := os.Stat(thumbPath)
	if err != nil {
		t.Fatalf("Thumbnail file does not exist: %v", err)
	}
	if info.Size() == 0 {
		t.Error("Video thumbnail is 0 bytes")
	}

	tf, err := os.Open(thumbPath)
	if err != nil {
		t.Fatalf("Failed to open thumbnail: %v", err)
	}
	defer tf.Close()
	cfg, format, err := image.DecodeConfig(tf)
	if err != nil || format != "jpeg" {
		t.Errorf("Expected valid JPEG thumbnail, got format=%s, err=%v", format, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		t.Errorf("Invalid thumbnail dimensions: %dx%d", cfg.Width, cfg.Height)
	}

	// 3. Repeated call should return cached path without error
	secondPath, err := thumbSvc.GetOrGenerate(ctx, p)
	if err != nil {
		t.Fatalf("Failed on repeated video thumbnail call: %v", err)
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
		MediaType:    "image/jpeg",
	}

	_, err = thumbSvc.GetOrGenerate(context.Background(), p)
	if err == nil {
		t.Error("Expected error when generating thumbnail in read-only storage, got nil")
	}
}
