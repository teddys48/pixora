package scanner

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"pixora/backend/internal/media"
	"pixora/backend/internal/models"
	"pixora/backend/internal/repository"
	"pixora/backend/internal/storage"
)

var (
	// ErrScanInProgress is returned when a scan is requested while another is already running.
	ErrScanInProgress = errors.New("media scan is already in progress")
)

// ScanStatus represents the progress and result of a media scan operation.
type ScanStatus struct {
	Status     string     `json:"status"` // "idle", "running", "completed", "failed", "canceled"
	Scanned    int        `json:"scanned"`
	Indexed    int        `json:"indexed"`
	Updated    int        `json:"updated"`
	Skipped    int        `json:"skipped"`
	Failed     int        `json:"failed"`
	Errors     []string   `json:"errors,omitempty"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	DurationMs int64      `json:"durationMs"`
}

// Scanner scans media files in storage root and reconciles them with the database.
type Scanner struct {
	storage    *storage.Storage
	photoRepo  repository.PhotoRepository
	statusMu   sync.RWMutex
	status     ScanStatus
	isScanning atomic.Bool
	cancelMu   sync.Mutex
	cancelFunc context.CancelFunc
}

// New creates a new Scanner instance.
func New(store *storage.Storage, photoRepo repository.PhotoRepository) *Scanner {
	return &Scanner{
		storage:   store,
		photoRepo: photoRepo,
		status: ScanStatus{
			Status: "idle",
		},
	}
}

// Status returns a copy of the current scan status.
func (s *Scanner) Status() ScanStatus {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	return s.status
}

// Cancel interrupts any active scan operation safely.
func (s *Scanner) Cancel() {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
}

// Scan performs media discovery and indexing across the entire storage root.
func (s *Scanner) Scan(ctx context.Context) (ScanStatus, error) {
	return s.ScanPath(ctx, "")
}

// ScanPath performs media discovery and indexing for a specific relative folder under storage root.
func (s *Scanner) ScanPath(ctx context.Context, targetRelPath string, forceRefresh ...bool) (ScanStatus, error) {
	if s.storage == nil {
		return ScanStatus{Status: "failed"}, errors.New("storage unavailable")
	}
	if s.photoRepo == nil {
		return ScanStatus{Status: "failed"}, errors.New("photo repository unavailable")
	}

	if !s.isScanning.CompareAndSwap(false, true) {
		return s.Status(), ErrScanInProgress
	}
	defer s.isScanning.Store(false)

	scanCtx, cancel := context.WithCancel(ctx)
	s.cancelMu.Lock()
	s.cancelFunc = cancel
	s.cancelMu.Unlock()
	defer func() {
		s.cancelMu.Lock()
		s.cancelFunc = nil
		s.cancelMu.Unlock()
		cancel()
	}()

	isRefresh := len(forceRefresh) > 0 && forceRefresh[0]

	startTime := time.Now().UTC()
	s.statusMu.Lock()
	s.status = ScanStatus{
		Status:    "running",
		StartedAt: &startTime,
	}
	s.statusMu.Unlock()

	// Check read access to storage root
	if err := s.storage.CheckReadAccess(); err != nil {
		now := time.Now().UTC()
		s.statusMu.Lock()
		s.status.Status = "failed"
		s.status.FinishedAt = &now
		s.status.DurationMs = time.Since(startTime).Milliseconds()
		s.status.Errors = []string{fmt.Sprintf("storage read access check failed: %v", err)}
		statusCopy := s.status
		s.statusMu.Unlock()
		return statusCopy, err
	}

	rootDir := s.storage.Root()
	realRoot, err := filepath.EvalSymlinks(rootDir)
	if err != nil {
		realRoot = rootDir
	}

	walkStartDir := rootDir
	cleanRel := strings.TrimSpace(targetRelPath)
	if cleanRel != "" && cleanRel != "." {
		resolvedDir, _, err := s.storage.ResolveFolderPath(cleanRel)
		if err != nil {
			now := time.Now().UTC()
			s.statusMu.Lock()
			s.status.Status = "failed"
			s.status.FinishedAt = &now
			s.status.DurationMs = time.Since(startTime).Milliseconds()
			s.status.Errors = []string{fmt.Sprintf("invalid scan folder %q: %v", targetRelPath, err)}
			statusCopy := s.status
			s.statusMu.Unlock()
			return statusCopy, err
		}
		walkStartDir = resolvedDir
	}

	var (
		scanned int
		indexed int
		updated int
		skipped int
		failed  int
		errList []string
	)

	walkErr := filepath.WalkDir(walkStartDir, func(path string, d fs.DirEntry, err error) error {
		if scanCtx.Err() != nil {
			return scanCtx.Err()
		}

		if err != nil {
			failed++
			if len(errList) < 50 {
				errList = append(errList, fmt.Sprintf("cannot access path %q: %v", path, err))
			}
			return nil
		}

		name := d.Name()

		// Skip hidden directories and internal storage directories
		if d.IsDir() {
			if path != rootDir {
				if strings.HasPrefix(name, ".") || name == "thumbnails" || name == "tmp" {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Skip hidden files
		if strings.HasPrefix(name, ".") {
			return nil
		}

		// Check file extension (photos and videos)
		if !media.IsSupportedExtension(name) {
			return nil
		}

		// Compute path relative to STORAGE_ROOT
		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			failed++
			return nil
		}
		// Canonicalize to forward slashes for cross-platform stability
		relPath = filepath.ToSlash(filepath.Clean(relPath))

		// Check symlink escape: ensure file resolves within storage root
		if d.Type()&os.ModeSymlink != 0 {
			resolvedTarget, err := filepath.EvalSymlinks(path)
			if err != nil {
				failed++
				return nil
			}
			relToRoot, err := filepath.Rel(realRoot, resolvedTarget)
			if err != nil || strings.HasPrefix(relToRoot, "..") {
				// Symlink escapes root: skip for security
				return nil
			}
		}

		scanned++

		info, err := d.Info()
		if err != nil {
			failed++
			if len(errList) < 50 {
				errList = append(errList, fmt.Sprintf("failed to stat %s: %v", relPath, err))
			}
			return nil
		}

		fileSize := info.Size()
		fileModTime := info.ModTime().UTC()
		isVideo := media.IsSupportedVideoExtension(name)

		// Check if media is already in database
		existing, err := s.photoRepo.GetByRelativePath(scanCtx, relPath)
		if err == nil && existing != nil {
			// Record exists: check if file was modified (or force refresh requested)
			if !isRefresh && existing.FileSize == fileSize && existing.FileModifiedAt.Equal(fileModTime) {
				skipped++
				return nil
			}

			// File changed or refresh requested: re-extract metadata and update
			if isVideo {
				vMeta, vErr := s.inspectVideo(scanCtx, relPath)
				if vErr != nil {
					failed++
					if len(errList) < 50 {
						errList = append(errList, fmt.Sprintf("failed to extract video metadata for %s: %v", relPath, vErr))
					}
					return nil
				}
				existing.FileSize = fileSize
				existing.FileModifiedAt = fileModTime
				existing.MediaType = vMeta.MediaType
				if vMeta.Width > 0 && vMeta.Height > 0 {
					existing.Width = &vMeta.Width
					existing.Height = &vMeta.Height
				}
				if vMeta.Duration > 0 {
					existing.Duration = &vMeta.Duration
				}
				if vMeta.VideoCodec != "" {
					existing.VideoCodec = &vMeta.VideoCodec
				}
				if vMeta.AudioCodec != "" {
					existing.AudioCodec = &vMeta.AudioCodec
				}
				if vMeta.ContainerFormat != "" {
					existing.ContainerFormat = &vMeta.ContainerFormat
				}
				if vMeta.CreationTime != nil {
					existing.TakenAt = vMeta.CreationTime
				}
				existing.UpdatedAt = time.Now().UTC()

				if err := s.photoRepo.Update(scanCtx, existing); err != nil {
					failed++
				} else {
					updated++
				}
				return nil
			}

			// Photo update
			meta, extractErr := s.inspectFile(relPath)
			if extractErr != nil {
				failed++
				if len(errList) < 50 {
					errList = append(errList, fmt.Sprintf("failed to extract metadata for updated %s: %v", relPath, extractErr))
				}
				return nil
			}

			existing.FileSize = fileSize
			existing.FileModifiedAt = fileModTime
			existing.MediaType = meta.MediaType
			if meta.Width > 0 && meta.Height > 0 {
				existing.Width = &meta.Width
				existing.Height = &meta.Height
			}
			existing.TakenAt = meta.TakenAt
			existing.UpdatedAt = time.Now().UTC()

			if err := s.photoRepo.Update(scanCtx, existing); err != nil {
				failed++
			} else {
				updated++
			}
			return nil
		}

		// New media record: extract metadata
		if isVideo {
			vMeta, vErr := s.inspectVideo(scanCtx, relPath)
			if vErr != nil {
				failed++
				if len(errList) < 50 {
					errList = append(errList, fmt.Sprintf("failed to read video %s: %v", relPath, vErr))
				}
				return nil
			}

			photoID := uuid.New().String()
			var wPtr, hPtr *int
			if vMeta.Width > 0 && vMeta.Height > 0 {
				wPtr = &vMeta.Width
				hPtr = &vMeta.Height
			}
			var durPtr *float64
			if vMeta.Duration > 0 {
				durPtr = &vMeta.Duration
			}
			var vCodecPtr *string
			if vMeta.VideoCodec != "" {
				vCodecPtr = &vMeta.VideoCodec
			}
			var aCodecPtr *string
			if vMeta.AudioCodec != "" {
				aCodecPtr = &vMeta.AudioCodec
			}
			var cFormatPtr *string
			if vMeta.ContainerFormat != "" {
				cFormatPtr = &vMeta.ContainerFormat
			}
			thumbStatus := "ready"

			newMedia := &models.Photo{
				ID:               photoID,
				RelativePath:     relPath,
				OriginalFilename: name,
				MediaType:        vMeta.MediaType,
				FileSize:         fileSize,
				Width:            wPtr,
				Height:           hPtr,
				Duration:         durPtr,
				VideoCodec:       vCodecPtr,
				AudioCodec:       aCodecPtr,
				ContainerFormat:  cFormatPtr,
				ThumbnailStatus:  &thumbStatus,
				TakenAt:          vMeta.CreationTime,
				FileModifiedAt:   fileModTime,
				CreatedAt:        time.Now().UTC(),
				UpdatedAt:        time.Now().UTC(),
			}

			if err := s.photoRepo.Create(scanCtx, newMedia); err != nil {
				failed++
				if len(errList) < 50 {
					errList = append(errList, fmt.Sprintf("failed to index video %s: %v", relPath, err))
				}
			} else {
				indexed++
			}
			return nil
		}

		// New Photo
		meta, extractErr := s.inspectFile(relPath)
		if extractErr != nil {
			failed++
			if len(errList) < 50 {
				errList = append(errList, fmt.Sprintf("failed to read %s: %v", relPath, extractErr))
			}
			return nil
		}

		photoID := uuid.New().String()
		var wPtr, hPtr *int
		if meta.Width > 0 && meta.Height > 0 {
			wPtr = &meta.Width
			hPtr = &meta.Height
		}

		newPhoto := &models.Photo{
			ID:               photoID,
			RelativePath:     relPath,
			OriginalFilename: name,
			MediaType:        meta.MediaType,
			FileSize:         fileSize,
			Width:            wPtr,
			Height:           hPtr,
			TakenAt:          meta.TakenAt,
			FileModifiedAt:   fileModTime,
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		}

		if err := s.photoRepo.Create(scanCtx, newPhoto); err != nil {
			failed++
			if len(errList) < 50 {
				errList = append(errList, fmt.Sprintf("failed to index %s: %v", relPath, err))
			}
		} else {
			indexed++
		}

		return nil
	})

	now := time.Now().UTC()
	s.statusMu.Lock()
	s.status.Scanned = scanned
	s.status.Indexed = indexed
	s.status.Updated = updated
	s.status.Skipped = skipped
	s.status.Failed = failed
	s.status.Errors = errList
	s.status.FinishedAt = &now
	s.status.DurationMs = time.Since(startTime).Milliseconds()

	if walkErr != nil {
		if errors.Is(walkErr, context.Canceled) {
			s.status.Status = "canceled"
		} else {
			s.status.Status = "failed"
			s.status.Errors = append(s.status.Errors, walkErr.Error())
		}
	} else {
		s.status.Status = "completed"
	}

	result := s.status
	s.statusMu.Unlock()

	return result, walkErr
}

// inspectFile opens a file under storage root and extracts its image metadata.
func (s *Scanner) inspectFile(relPath string) (media.ImageInfo, error) {
	resolved, err := s.storage.ResolvePath(relPath)
	if err != nil {
		return media.ImageInfo{}, err
	}

	f, err := os.Open(resolved)
	if err != nil {
		return media.ImageInfo{}, err
	}
	defer f.Close()

	return media.ExtractMetadata(f)
}

// inspectVideo extracts metadata from a video file under storage root.
func (s *Scanner) inspectVideo(ctx context.Context, relPath string) (media.VideoInfo, error) {
	resolved, err := s.storage.ResolvePath(relPath)
	if err != nil {
		return media.VideoInfo{}, err
	}
	return media.ExtractVideoMetadata(ctx, resolved)
}
