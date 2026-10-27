package media

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"sync"

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
)

// ThumbnailService generates, caches, and serves thumbnails.
type ThumbnailService struct {
	storage *storage.Storage
	locks   sync.Map // photoID -> *sync.Mutex
}

// NewThumbnailService creates a new ThumbnailService.
func NewThumbnailService(storage *storage.Storage) *ThumbnailService {
	return &ThumbnailService{
		storage: storage,
	}
}

// ThumbnailRelativePath returns the deterministic relative path for a photo's thumbnail.
func (s *ThumbnailService) ThumbnailRelativePath(photoID string) string {
	return filepath.Join("thumbnails", fmt.Sprintf("%s.jpg", photoID))
}

// GetOrGenerate returns the absolute path to the thumbnail for the given photo,
// generating it if it does not yet exist.
func (s *ThumbnailService) GetOrGenerate(ctx context.Context, p *models.Photo) (string, error) {
	if s.storage == nil {
		return "", errors.New("storage unavailable")
	}

	relThumb := s.ThumbnailRelativePath(p.ID)
	resolvedThumb, err := s.storage.ResolvePath(relThumb)
	if err != nil {
		return "", fmt.Errorf("failed to resolve thumbnail path: %w", err)
	}

	// 1. Check if thumbnail already exists and is non-empty
	if info, err := os.Stat(resolvedThumb); err == nil && info.Size() > 0 {
		return resolvedThumb, nil
	}

	// 2. Obtain lock for this photo ID to avoid duplicate concurrent generation
	lockVal, _ := s.locks.LoadOrStore(p.ID, &sync.Mutex{})
	mu := lockVal.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	// Double-check after acquiring lock
	if info, err := os.Stat(resolvedThumb); err == nil && info.Size() > 0 {
		return resolvedThumb, nil
	}

	// 3. Verify write access
	if err := s.storage.CheckWriteAccess(); err != nil {
		return "", fmt.Errorf("%w: cannot generate thumbnail in read-only storage", storage.ErrReadOnly)
	}

	// 4. Open original file
	f, err := s.storage.OpenFile(p.RelativePath)
	if err != nil {
		return "", fmt.Errorf("failed to open original photo: %w", err)
	}
	defer f.Close()

	// 5. Generate thumbnail to disk
	if err := s.generateThumbnail(f, resolvedThumb); err != nil {
		return "", err
	}

	return resolvedThumb, nil
}

// generateThumbnail decodes the source image, applies orientation, scales it, and writes JPEG to targetPath.
func (s *ThumbnailService) generateThumbnail(r io.Reader, targetPath string) error {
	// Decode original image
	srcImg, _, err := image.Decode(r)
	if err != nil {
		return fmt.Errorf("%w: failed to decode source image: %v", ErrInvalidImage, err)
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

	encodeErr := jpeg.Encode(tmpFile, thumbImg, &jpeg.Options{Quality: ThumbnailQuality})
	closeErr := tmpFile.Close()
	if encodeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to encode thumbnail jpeg: %w", encodeErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to flush thumbnail file: %w", closeErr)
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
