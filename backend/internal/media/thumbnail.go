package media

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/image/draw"

	"pixora/backend/internal/models"
	"pixora/backend/internal/storage"
)

const (
	// DefaultThumbnailSize is the max bounding box dimension for thumbnails.
	DefaultThumbnailSize = 400

	// ThumbnailQuality is the JPEG compression quality for generated thumbnails.
	ThumbnailQuality = 85

	// MaxConcurrentImages is the maximum number of simultaneous image decodes allowed.
	MaxConcurrentImages = 4

	// MaxConcurrentVideos is the maximum number of simultaneous ffmpeg operations allowed.
	MaxConcurrentVideos = 3
)

// ThumbnailService generates, caches, and serves thumbnails for photos and videos.
type ThumbnailService struct {
	storage   *storage.Storage
	locks     [64]sync.Mutex // Striped locks to prevent memory leaks from unbounded key storage
	ffmpegSem chan struct{}  // Concurrency limiter for ffmpeg
	imageSem  chan struct{}  // Concurrency limiter for image decoding
	failedIDs sync.Map       // photoID -> time.Time
}

// NewThumbnailService creates a new ThumbnailService.
func NewThumbnailService(storage *storage.Storage) *ThumbnailService {
	return &ThumbnailService{
		storage:   storage,
		ffmpegSem: make(chan struct{}, MaxConcurrentVideos),
		imageSem:  make(chan struct{}, MaxConcurrentImages),
	}
}

// getLock returns the striped mutex for a specific photo ID.
func (s *ThumbnailService) getLock(id string) *sync.Mutex {
	var h uint32 = 2166136261
	for i := 0; i < len(id); i++ {
		h = (h ^ uint32(id[i])) * 16777619
	}
	return &s.locks[h%64]
}

// ThumbnailRelativePath returns the deterministic relative path for a media thumbnail.
func (s *ThumbnailService) ThumbnailRelativePath(photoID string) string {
	return filepath.Join("thumbnails", fmt.Sprintf("%s.jpg", photoID))
}

// ClearFailed removes an ID from the negative failure cache (e.g. after file modification or rescan).
func (s *ThumbnailService) ClearFailed(photoID string) {
	s.failedIDs.Delete(photoID)
}

// isThumbnailFresh checks whether an existing thumbnail on disk is non-empty and newer than the source modification.
func isThumbnailFresh(thumbInfo os.FileInfo, sourceModified time.Time) bool {
	if thumbInfo.Size() == 0 {
		return false
	}
	if !sourceModified.IsZero() && sourceModified.After(thumbInfo.ModTime()) {
		// Source file was updated after thumbnail was created: thumbnail is stale
		return false
	}
	return true
}

// GetOrGenerate returns the absolute path to the thumbnail for the given photo or video,
// generating it if it does not yet exist or if the source file has been modified.
func (s *ThumbnailService) GetOrGenerate(ctx context.Context, p *models.Photo) (string, error) {
	if s.storage == nil {
		return "", errors.New("storage unavailable")
	}

	relThumb := s.ThumbnailRelativePath(p.ID)
	resolvedThumb, err := s.storage.ResolvePath(relThumb)
	if err != nil {
		return "", fmt.Errorf("failed to resolve thumbnail path: %w", err)
	}

	// 1. Fast path: check if thumbnail already exists, is non-empty, and is fresh
	if info, err := os.Stat(resolvedThumb); err == nil && isThumbnailFresh(info, p.FileModifiedAt) {
		return resolvedThumb, nil
	}

	// 2. Check if thumbnail generation previously failed recently (cooldown to avoid CPU thrashing)
	if failVal, ok := s.failedIDs.Load(p.ID); ok {
		if failTime, ok := failVal.(time.Time); ok && time.Since(failTime) < 3*time.Minute {
			return "", errors.New("thumbnail generation previously failed; cooling down")
		}
	}

	// 3. Obtain striped lock for this photo ID to avoid duplicate concurrent generation
	mu := s.getLock(p.ID)
	mu.Lock()
	defer mu.Unlock()

	// Double-check after acquiring lock
	if info, err := os.Stat(resolvedThumb); err == nil && isThumbnailFresh(info, p.FileModifiedAt) {
		return resolvedThumb, nil
	}

	// 4. Verify write access
	if err := s.storage.CheckWriteAccess(); err != nil {
		return "", fmt.Errorf("%w: cannot generate thumbnail in read-only storage", storage.ErrReadOnly)
	}

	resolvedOrig, err := s.storage.ResolvePath(p.RelativePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve original media: %w", err)
	}

	isVideo := strings.HasPrefix(p.MediaType, "video/") || IsSupportedVideoExtension(p.OriginalFilename)

	if isVideo {
		if err := s.generateVideoThumbnail(ctx, resolvedOrig, resolvedThumb, p.Duration); err != nil {
			s.failedIDs.Store(p.ID, time.Now().UTC())
			return "", err
		}
		s.failedIDs.Delete(p.ID)
		return resolvedThumb, nil
	}

	// 5. Open original image file
	f, err := s.storage.OpenFile(p.RelativePath)
	if err != nil {
		return "", fmt.Errorf("failed to open original photo: %w", err)
	}
	defer f.Close()

	// 6. Generate image thumbnail to disk with bounded concurrency
	select {
	case s.imageSem <- struct{}{}:
		defer func() { <-s.imageSem }()
	case <-ctx.Done():
		return "", ctx.Err()
	}

	if err := s.generateThumbnail(f, resolvedThumb); err != nil {
		s.failedIDs.Store(p.ID, time.Now().UTC())
		return "", err
	}
	s.failedIDs.Delete(p.ID)

	return resolvedThumb, nil
}

// generateVideoThumbnail extracts a representative video frame using ffmpeg and scales to thumbnail dimensions.
func (s *ThumbnailService) generateVideoThumbnail(ctx context.Context, inputPath, targetPath string, duration *float64) error {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found: %w", err)
	}

	// Limit concurrent ffmpeg processes
	select {
	case s.ffmpegSem <- struct{}{}:
		defer func() { <-s.ffmpegSem }()
	case <-ctx.Done():
		return ctx.Err()
	}

	destDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	tmpPath := filepath.Join(destDir, fmt.Sprintf(".thumb_%s.tmp.jpg", uuid.New().String()))
	defer os.Remove(tmpPath)

	seekTime := "00:00:01"
	if duration != nil && *duration < 1.5 {
		seekTime = "00:00:00"
	}

	runFFmpeg := func(st string) error {
		cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		cmd := exec.CommandContext(cmdCtx, ffmpegPath,
			"-y",
			"-ss", st,
			"-i", inputPath,
			"-vframes", "1",
			"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", DefaultThumbnailSize, DefaultThumbnailSize),
			"-q:v", "2",
			tmpPath,
		)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		return cmd.Run()
	}

	err = runFFmpeg(seekTime)
	if err != nil && seekTime != "00:00:00" {
		err = runFFmpeg("00:00:00")
	}
	if err != nil {
		return fmt.Errorf("ffmpeg thumbnail generation failed: %w", err)
	}

	info, err := os.Stat(tmpPath)
	if err != nil || info.Size() == 0 {
		return errors.New("ffmpeg generated empty thumbnail")
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("failed to commit video thumbnail file: %w", err)
	}

	return nil
}

// generateThumbnail decodes the source image, applies EXIF orientation, scales it preserving aspect ratio, and writes JPEG to targetPath.
func (s *ThumbnailService) generateThumbnail(r io.Reader, targetPath string) error {
	orientation := 1
	if rs, ok := r.(io.ReadSeeker); ok {
		if meta, err := ExtractMetadata(rs); err == nil && meta.Orientation >= 1 && meta.Orientation <= 8 {
			orientation = meta.Orientation
		}
		if _, err := rs.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("failed to reset reader: %w", err)
		}
	}

	// Use buffered reader for smooth decoding
	bufReader := bufio.NewReaderSize(r, 64*1024)

	// Decode original image
	srcImg, _, err := image.Decode(bufReader)
	if err != nil {
		return fmt.Errorf("%w: failed to decode source image: %v", ErrInvalidImage, err)
	}

	// Apply EXIF orientation transformation if necessary
	if orientation > 1 {
		srcImg = ApplyOrientation(srcImg, orientation)
	}

	// Calculate thumbnail dimensions
	srcBounds := srcImg.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()
	if srcW <= 0 || srcH <= 0 {
		return fmt.Errorf("%w: invalid dimensions %dx%d", ErrInvalidImage, srcW, srcH)
	}

	targetW, targetH := computeScaleDimensions(srcW, srcH, DefaultThumbnailSize, DefaultThumbnailSize)

	// Create scaled image canvas
	thumbImg := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	draw.BiLinear.Scale(thumbImg, thumbImg.Bounds(), srcImg, srcBounds, draw.Over, nil)

	// Ensure destination directory exists
	destDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	// Write to temporary file first for atomic creation
	tmpPath := filepath.Join(destDir, fmt.Sprintf(".thumb_%s.tmp", uuid.New().String()))
	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create temporary thumbnail file: %w", err)
	}

	// Buffered write for fast flush to disk
	bufWriter := bufio.NewWriterSize(tmpFile, 64*1024)
	encodeErr := jpeg.Encode(bufWriter, thumbImg, &jpeg.Options{Quality: ThumbnailQuality})
	flushErr := bufWriter.Flush()
	closeErr := tmpFile.Close()

	if encodeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to encode thumbnail jpeg: %w", encodeErr)
	}
	if flushErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to flush thumbnail buffer: %w", flushErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to close thumbnail file: %w", closeErr)
	}

	// Atomic rename to final path
	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to commit thumbnail file: %w", err)
	}

	return nil
}

// computeScaleDimensions calculates dimensions keeping aspect ratio within maxW x maxH.
func computeScaleDimensions(srcW, srcH, maxW, maxH int) (int, int) {
	if srcW <= maxW && srcH <= maxH {
		return srcW, srcH
	}

	aspect := float64(srcW) / float64(srcH)
	if float64(maxW)/float64(maxH) > aspect {
		// Height is the limiting factor
		newH := maxH
		newW := int(float64(maxH) * aspect)
		if newW < 1 {
			newW = 1
		}
		return newW, newH
	}

	// Width is the limiting factor
	newW := maxW
	newH := int(float64(maxW) / aspect)
	if newH < 1 {
		newH = 1
	}
	return newW, newH
}

// ApplyOrientation applies the EXIF orientation transformation to an image.
func ApplyOrientation(src image.Image, orientation int) image.Image {
	switch orientation {
	case 2:
		return flipH(src)
	case 3:
		return rotate180(src)
	case 4:
		return flipV(src)
	case 5:
		return flipH(rotate90(src))
	case 6:
		return rotate90(src)
	case 7:
		return flipH(rotate270(src))
	case 8:
		return rotate270(src)
	default:
		return src
	}
}

func rotate90(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(h-1-y, x, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func rotate180(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(w-1-x, h-1-y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func rotate270(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(y, w-1-x, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func flipH(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(w-1-x, y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func flipV(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, h-1-y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
