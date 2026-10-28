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
	"math/big"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/tiff"
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

// ExifDetails contains extended photography metadata for photo viewer display.
type ExifDetails struct {
	Camera       string `json:"camera,omitempty"`
	Lens         string `json:"lens,omitempty"`
	FocalLength  string `json:"focalLength,omitempty"`
	Aperture     string `json:"aperture,omitempty"`
	ShutterSpeed string `json:"shutterSpeed,omitempty"`
	ISO          *int   `json:"iso,omitempty"`
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

// parseExifTimestamp attempts to parse a string representation of an EXIF timestamp.
func parseExifTimestamp(raw string) (*time.Time, error) {
	cleaned := strings.Trim(strings.TrimSpace(raw), "\"")
	if cleaned == "" || strings.HasPrefix(cleaned, "0000:00:00") {
		return nil, errors.New("empty or zero EXIF date")
	}

	// Standard EXIF format: "2006:01:02 15:04:05"
	layouts := []string{
		"2006:01:02 15:04:05",
		"2006-01-02 15:04:05",
		"2006:01:02 15:04",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		time.RFC3339,
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, cleaned); err == nil && !t.IsZero() && t.Year() > 1970 {
			utc := t.UTC()
			return &utc, nil
		}
	}

	return nil, fmt.Errorf("unrecognized EXIF timestamp format %q", cleaned)
}

// ExtractCaptureDateFromExif extracts the best capture date from EXIF adhering to strict precedence:
// 1. EXIF DateTimeOriginal (primary photo capture instant)
// 2. EXIF DateTimeDigitized (alternative valid capture timestamp)
// 3. EXIF DateTime (EXIF file timestamp)
// 4. Returns nil if no valid EXIF date is found
func ExtractCaptureDateFromExif(x *exif.Exif) *time.Time {
	if x == nil {
		return nil
	}

	// 1. Try DateTimeOriginal
	if tag, err := x.Get(exif.DateTimeOriginal); err == nil && tag != nil {
		if val, err := tag.StringVal(); err == nil {
			if parsed, err := parseExifTimestamp(val); err == nil {
				return parsed
			}
		}
	}

	// 2. Try DateTimeDigitized
	if tag, err := x.Get(exif.DateTimeDigitized); err == nil && tag != nil {
		if val, err := tag.StringVal(); err == nil {
			if parsed, err := parseExifTimestamp(val); err == nil {
				return parsed
			}
		}
	}

	// 3. Try DateTime
	if tag, err := x.Get(exif.DateTime); err == nil && tag != nil {
		if val, err := tag.StringVal(); err == nil {
			if parsed, err := parseExifTimestamp(val); err == nil {
				return parsed
			}
		}
	}

	// Fallback to goexif's built-in parser if tags were structured differently
	if dt, err := x.DateTime(); err == nil && !dt.IsZero() && dt.Year() > 1970 {
		utc := dt.UTC()
		return &utc
	}

	return nil
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
				// Extract capture time using documented precedence
				info.TakenAt = ExtractCaptureDateFromExif(x)

				// Extract orientation tag (1-8)
				if tag, err := x.Get(exif.Orientation); err == nil && tag != nil {
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

	takenAt := ExtractCaptureDateFromExif(x)

	orientation := 1
	if tag, err := x.Get(exif.Orientation); err == nil && tag != nil {
		if val, err := tag.Int(0); err == nil && val >= 1 && val <= 8 {
			orientation = val
		}
	}

	return takenAt, orientation
}

// ExtractExifDetails parses extended camera and exposure metadata safely from an image reader.
func ExtractExifDetails(r io.ReadSeeker) (*ExifDetails, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	x, err := exif.Decode(r)
	if err != nil || x == nil {
		return nil, nil // No EXIF or not decodable; return nil without error
	}

	details := &ExifDetails{}
	var makeVal, modelVal string

	if tag, err := x.Get(exif.Make); err == nil && tag != nil {
		if val, err := tag.StringVal(); err == nil {
			makeVal = strings.Trim(strings.TrimSpace(val), "\"")
		}
	}

	if tag, err := x.Get(exif.Model); err == nil && tag != nil {
		if val, err := tag.StringVal(); err == nil {
			modelVal = strings.Trim(strings.TrimSpace(val), "\"")
		}
	}

	if makeVal != "" || modelVal != "" {
		if strings.Contains(strings.ToLower(modelVal), strings.ToLower(makeVal)) {
			details.Camera = modelVal
		} else if makeVal != "" && modelVal != "" {
			details.Camera = fmt.Sprintf("%s %s", makeVal, modelVal)
		} else if modelVal != "" {
			details.Camera = modelVal
		} else {
			details.Camera = makeVal
		}
	}

	// LensModel (Tag 0xA434)
	if tag, err := x.Get(exif.FieldName("LensModel")); err == nil && tag != nil {
		if val, err := tag.StringVal(); err == nil {
			details.Lens = strings.Trim(strings.TrimSpace(val), "\"")
		}
	}

	// FocalLength
	if tag, err := x.Get(exif.FocalLength); err == nil && tag != nil {
		if num, den, err := tag.Rat2(0); err == nil && den > 0 {
			val := float64(num) / float64(den)
			details.FocalLength = fmt.Sprintf("%.0fmm", val)
		} else if rat := getRatVal(tag); rat != nil {
			f, _ := rat.Float64()
			details.FocalLength = fmt.Sprintf("%.0fmm", f)
		}
	}

	// Aperture (FNumber)
	if tag, err := x.Get(exif.FNumber); err == nil && tag != nil {
		if num, den, err := tag.Rat2(0); err == nil && den > 0 {
			f := float64(num) / float64(den)
			details.Aperture = fmt.Sprintf("f/%.1f", f)
		} else if rat := getRatVal(tag); rat != nil {
			f, _ := rat.Float64()
			details.Aperture = fmt.Sprintf("f/%.1f", f)
		}
	}

	// ExposureTime (Shutter Speed)
	if tag, err := x.Get(exif.ExposureTime); err == nil && tag != nil {
		if num, den, err := tag.Rat2(0); err == nil && den > 0 {
			if num == 1 || (den%num == 0 && num > 0) {
				details.ShutterSpeed = fmt.Sprintf("1/%ds", den/num)
			} else {
				val := float64(num) / float64(den)
				if val < 1.0 {
					details.ShutterSpeed = fmt.Sprintf("1/%.0fs", 1.0/val)
				} else {
					details.ShutterSpeed = fmt.Sprintf("%.1fs", val)
				}
			}
		}
	}

	// ISO (ISOSpeedRatings)
	if tag, err := x.Get(exif.ISOSpeedRatings); err == nil && tag != nil {
		if iso, err := tag.Int(0); err == nil && iso > 0 {
			details.ISO = &iso
		}
	}

	return details, nil
}

func getRatVal(tag *tiff.Tag) *big.Rat {
	if tag == nil {
		return nil
	}
	if rat, err := tag.Rat(0); err == nil {
		return rat
	}
	return nil
}
