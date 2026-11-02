package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrUnsupportedVideoFormat is returned when a video format is not recognized.
	ErrUnsupportedVideoFormat = errors.New("unsupported video format")
)

// VideoInfo contains extracted metadata from a video file.
type VideoInfo struct {
	MediaType       string
	Extension       string
	Width           int
	Height          int
	Duration        float64
	VideoCodec      string
	AudioCodec      string
	ContainerFormat string
	CreationTime    *time.Time
}

// SniffVideoMediaType detects the MIME type and canonical extension from header magic bytes.
func SniffVideoMediaType(header []byte) (mediaType string, ext string, err error) {
	if len(header) >= 12 {
		// ISO Base Media File Format: bytes 4..7 == "ftyp"
		if string(header[4:8]) == "ftyp" {
			brand := string(header[8:12])
			switch brand {
			case "qt  ":
				return "video/quicktime", ".mov", nil
			case "M4V ":
				return "video/mp4", ".m4v", nil
			default:
				return "video/mp4", ".mp4", nil
			}
		}

		// QuickTime alternative atom signatures (e.g. moov, mdat, wide at offset 4)
		atom := string(header[4:8])
		if atom == "moov" || atom == "mdat" || atom == "wide" || atom == "free" || atom == "skip" {
			return "video/quicktime", ".mov", nil
		}
	}

	// Matroska / WebM EBML header: 0x1A 0x45 0xDF 0xA3
	if len(header) >= 4 && header[0] == 0x1A && header[1] == 0x45 && header[2] == 0xDF && header[3] == 0xA3 {
		limit := len(header)
		if limit > 256 {
			limit = 256
		}
		sample := strings.ToLower(string(header[:limit]))
		if strings.Contains(sample, "webm") {
			return "video/webm", ".webm", nil
		}
		if strings.Contains(sample, "matroska") {
			return "video/x-matroska", ".mkv", nil
		}
		return "video/webm", ".webm", nil
	}

	return "", "", ErrUnsupportedVideoFormat
}

// DetectVideoMediaType inspects a file's extension and initial header bytes to determine MIME type.
func DetectVideoMediaType(filename string, r io.Reader) (mediaType string, ext string, err error) {
	lowerExt := strings.ToLower(filepath.Ext(filename))
	if !IsSupportedVideoExtension(filename) {
		return "", "", ErrUnsupportedVideoFormat
	}

	header := make([]byte, 512)
	n, readErr := io.ReadFull(r, header)
	if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
		return "", "", fmt.Errorf("failed to read video header: %w", readErr)
	}
	header = header[:n]

	// 1. Try magic byte detection
	sniffedType, sniffedExt, sniffErr := SniffVideoMediaType(header)
	if sniffErr == nil {
		if lowerExt == ".mkv" && sniffedType == "video/webm" {
			return "video/x-matroska", ".mkv", nil
		}
		if lowerExt == ".m4v" && sniffedType == "video/mp4" {
			return "video/mp4", ".m4v", nil
		}
		if lowerExt == ".mov" && sniffedType == "video/mp4" {
			return "video/quicktime", ".mov", nil
		}
		return sniffedType, sniffedExt, nil
	}

	// 2. Extension-based mapping when magic bytes are partial or generic
	switch lowerExt {
	case ".mp4":
		return "video/mp4", ".mp4", nil
	case ".webm":
		return "video/webm", ".webm", nil
	case ".mov":
		return "video/quicktime", ".mov", nil
	case ".m4v":
		return "video/mp4", ".m4v", nil
	case ".mkv":
		return "video/x-matroska", ".mkv", nil
	default:
		return "", "", ErrUnsupportedVideoFormat
	}
}

type ffprobeStream struct {
	CodecType    string            `json:"codec_type"`
	CodecName    string            `json:"codec_name"`
	Width        int               `json:"width"`
	Height       int               `json:"height"`
	Duration     string            `json:"duration"`
	Tags         map[string]string `json:"tags"`
	SideDataList []struct {
		Rotation int `json:"rotation"`
	} `json:"side_data_list"`
}

type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
	Format  struct {
		FormatName string            `json:"format_name"`
		Duration   string            `json:"duration"`
		Tags       map[string]string `json:"tags"`
	} `json:"format"`
}

// ExtractVideoMetadata extracts metadata (duration, resolution, codecs, container, creation timestamp)
// using ffprobe without shell string interpolation and with timeout control.
func ExtractVideoMetadata(ctx context.Context, filePath string) (VideoInfo, error) {
	// Detect media type and extension
	f, err := os.Open(filePath)
	if err != nil {
		return VideoInfo{}, fmt.Errorf("failed to open video: %w", err)
	}
	mimeType, ext, detectErr := DetectVideoMediaType(filePath, f)
	_ = f.Close()
	if detectErr != nil {
		return VideoInfo{}, detectErr
	}

	info := VideoInfo{
		MediaType: mimeType,
		Extension: ext,
	}

	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		// ffprobe not installed: return basic VideoInfo without failing the file
		return info, nil
	}

	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(probeCtx, ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		// If probe failed on corrupt/partial file, still return basic info
		return info, nil
	}

	var data ffprobeOutput
	if err := json.Unmarshal(stdout.Bytes(), &data); err != nil {
		return info, nil
	}

	// Container format
	info.ContainerFormat = data.Format.FormatName

	// Duration (seconds)
	if data.Format.Duration != "" {
		if dur, err := strconv.ParseFloat(data.Format.Duration, 64); err == nil && dur > 0 {
			info.Duration = dur
		}
	}

	// Streams: extract video and audio metadata
	for _, stream := range data.Streams {
		if stream.CodecType == "video" && info.VideoCodec == "" {
			info.VideoCodec = stream.CodecName
			w := stream.Width
			h := stream.Height

			// Check rotation metadata
			rotation := 0
			for _, sd := range stream.SideDataList {
				if sd.Rotation != 0 {
					rotation = sd.Rotation
					break
				}
			}
			if rotation == 0 && stream.Tags != nil {
				if rotStr, ok := stream.Tags["rotate"]; ok {
					if rotVal, err := strconv.Atoi(rotStr); err == nil {
						rotation = rotVal
					}
				}
			}

			// Swap dimensions if rotated 90 or 270 degrees
			if rotation == 90 || rotation == 270 || rotation == -90 || rotation == -270 {
				w, h = h, w
			}

			info.Width = w
			info.Height = h

			if info.Duration <= 0 && stream.Duration != "" {
				if dur, err := strconv.ParseFloat(stream.Duration, 64); err == nil && dur > 0 {
					info.Duration = dur
				}
			}
		} else if stream.CodecType == "audio" && info.AudioCodec == "" {
			info.AudioCodec = stream.CodecName
		}
	}

	// Creation timestamp: check format tags first, then video stream tags
	var rawCreationTime string
	if data.Format.Tags != nil {
		if ct, ok := data.Format.Tags["creation_time"]; ok {
			rawCreationTime = ct
		}
	}
	if rawCreationTime == "" {
		for _, s := range data.Streams {
			if s.Tags != nil {
				if ct, ok := s.Tags["creation_time"]; ok {
					rawCreationTime = ct
					break
				}
			}
		}
	}

	if rawCreationTime != "" {
		info.CreationTime = parseVideoTimestamp(rawCreationTime)
	}

	return info, nil
}

func parseVideoTimestamp(raw string) *time.Time {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" || strings.HasPrefix(cleaned, "0000-00-00") {
		return nil
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006:01:02 15:04:05",
	}

	for _, l := range layouts {
		if t, err := time.Parse(l, cleaned); err == nil && !t.IsZero() && t.Year() > 1970 {
			utc := t.UTC()
			return &utc
		}
	}
	return nil
}
