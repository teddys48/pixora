package media

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

var (
	// ErrUnsupportedFormat is returned when an image format is not supported.
	ErrUnsupportedFormat = errors.New("unsupported image format")

	// ErrDecompressionBomb is returned when an image exceeds safe dimension limits.
	ErrDecompressionBomb = errors.New("image dimensions exceed safe limits")

	// ErrInvalidImage is returned when an image cannot be parsed or decoded.
	ErrInvalidImage = errors.New("invalid or corrupt image content")
)

const (
	// MaxDimension is the maximum allowed width or height in pixels to prevent decompression bombs.
	MaxDimension = 15000

	// MaxTotalPixels is the maximum allowed total pixel count (100 megapixels).
	MaxTotalPixels = 100_000_000
)

// ImageInfo contains extracted metadata from an image file.
type ImageInfo struct {
	MediaType   string
	Extension   string
	Width       int
	Height      int
	TakenAt     *time.Time
	Orientation int
}

// SniffMediaType detects the MIME type and canonical extension based on magic header bytes.
func SniffMediaType(header []byte) (mediaType string, ext string, err error) {
	if len(header) >= 3 && header[0] == 0xFF && header[1] == 0xD8 && header[2] == 0xFF {
		return "image/jpeg", ".jpg", nil
	}
	if len(header) >= 8 &&
		header[0] == 0x89 && header[1] == 'P' && header[2] == 'N' && header[3] == 'G' &&
		header[4] == '\r' && header[5] == '\n' && header[6] == 0x1A && header[7] == '\n' {
		return "image/png", ".png", nil
	}
	if len(header) >= 6 &&
		(string(header[:6]) == "GIF87a" || string(header[:6]) == "GIF89a") {
		return "image/gif", ".gif", nil
	}
	if len(header) >= 12 &&
		string(header[:4]) == "RIFF" && string(header[8:12]) == "WEBP" {
		return "image/webp", ".webp", nil
	}
	if len(header) >= 4 &&
		((header[0] == 'I' && header[1] == 'I' && header[2] == 0x2A && header[3] == 0x00) ||
			(header[0] == 'M' && header[1] == 'M' && header[2] == 0x00 && header[3] == 0x2A)) {
		return "image/tiff", ".tiff", nil
	}

	return "", "", ErrUnsupportedFormat
}

// IsSupportedExtension returns whether a file extension matches a supported image format.
func IsSupportedExtension(filename string) bool {
	lower := strings.ToLower(filename)
	ext := ""
	for i := len(lower) - 1; i >= 0; i-- {
		if lower[i] == '.' {
			ext = lower[i:]
			break
		}
	}
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".tif", ".tiff":
		return true
	default:
		return false
	}
}

// ExtractMetadata reads image dimensions, format, and EXIF metadata without loading the entire image into memory.
func ExtractMetadata(r io.ReadSeeker) (ImageInfo, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return ImageInfo{}, fmt.Errorf("failed to seek image: %w", err)
	}

	header := make([]byte, 512)
	n, err := io.ReadFull(r, header)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return ImageInfo{}, fmt.Errorf("failed to read image header: %w", err)
	}

	mediaType, ext, err := SniffMediaType(header[:n])
	if err != nil {
		return ImageInfo{}, err
	}

	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return ImageInfo{}, fmt.Errorf("failed to reset image reader: %w", err)
	}

	// DecodeConfig reads only header metadata without decompressing full image bytes
	cfg, _, err := image.DecodeConfig(r)
	if err != nil {
		return ImageInfo{}, fmt.Errorf("%w: %v", ErrInvalidImage, err)
	}

	if cfg.Width <= 0 || cfg.Height <= 0 {
		return ImageInfo{}, fmt.Errorf("%w: non-positive dimensions %dx%d", ErrInvalidImage, cfg.Width, cfg.Height)
	}

	if cfg.Width > MaxDimension || cfg.Height > MaxDimension {
		return ImageInfo{}, fmt.Errorf("%w: dimensions %dx%d exceed limit of %d", ErrDecompressionBomb, cfg.Width, cfg.Height, MaxDimension)
	}

	if int64(cfg.Width)*int64(cfg.Height) > MaxTotalPixels {
		return ImageInfo{}, fmt.Errorf("%w: total pixels exceed limit", ErrDecompressionBomb)
	}

	info := ImageInfo{
		MediaType:   mediaType,
		Extension:   ext,
		Width:       cfg.Width,
		Height:      cfg.Height,
		Orientation: 1, // Default normal orientation
	}

	// Attempt EXIF extraction (applicable mainly to JPEG and TIFF)
	if mediaType == "image/jpeg" || mediaType == "image/tiff" {
		if _, err := r.Seek(0, io.SeekStart); err == nil {
			x, err := exif.Decode(r)
			if err == nil && x != nil {
				// Extract capture time
				if dt, err := x.DateTime(); err == nil && !dt.IsZero() {
					utc := dt.UTC()
					info.TakenAt = &utc
				}

				// Extract orientation tag (1-8)
				if tag, err := x.Get(exif.Orientation); err == nil {
					if orientVal, err := tag.Int(0); err == nil && orientVal >= 1 && orientVal <= 8 {
						info.Orientation = orientVal
					}
				}
			}
		}
	}

	// For orientations 5, 6, 7, 8 (90 or 270 degree rotation), swap display width and height
	if info.Orientation >= 5 && info.Orientation <= 8 {
		info.Width, info.Height = info.Height, info.Width
	}

	return info, nil
}

// ParseEXIFSafe parses EXIF metadata from an in-memory byte slice safely.
func ParseEXIFSafe(data []byte) (*time.Time, int) {
	if len(data) == 0 {
		return nil, 1
	}

	x, err := exif.Decode(bytes.NewReader(data))
	if err != nil || x == nil {
		return nil, 1
	}

	var takenAt *time.Time
	if dt, err := x.DateTime(); err == nil && !dt.IsZero() {
		utc := dt.UTC()
		takenAt = &utc
	}

	orientation := 1
	if tag, err := x.Get(exif.Orientation); err == nil {
		if val, err := tag.Int(0); err == nil && val >= 1 && val <= 8 {
			orientation = val
		}
	}

	return takenAt, orientation
}
