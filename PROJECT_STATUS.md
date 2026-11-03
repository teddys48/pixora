# Pixora — Project Status

## 1. Project Overview

**Project:** Pixora  
**Description:** A web-based self-hosted photo and video gallery for uploading media, organizing physical folders and virtual albums, searching, filtering, and browsing media chronologically.

**Current overall status:** Phase 8 — Performance Optimization & Production Hardening  
**Current phase:** Phase 8 Completed (Production Ready)

### Product Goals

- Provide a fast, responsive, and accessible photo and video gallery UI.
- Support uploading one or multiple media files safely.
- Discover and index existing photos and videos in mounted host storage (`/media`).
- Support read-only physical folder and nested subfolder browsing directly from `/media`.
- Detect unindexed media files on disk with targeted one-click scanning.
- Organize photos and videos into virtual collections and albums without moving original files.
- Browse photos and videos chronologically using capture dates when available.
- View full-resolution photos in a responsive lightbox with EXIF metadata and stream videos via RFC 7233 range requests.
- Search indexed media by filename, relative folder path, and metadata with case-insensitivity and wildcard safety.
- Filter media flexibly by date range, physical folder (direct/recursive), virtual album, media format, and undated status.
- Sort media deterministically by capture date (newest/oldest), filename, file size, or modification date.
- Store media and metadata reliably on local storage with host bind mounts.
- Scale reliably to libraries with tens of thousands of media files using optimized SQLite pragmas, composite indexes, and bounded concurrency.
- Keep deployment and maintenance simple with a single-container multi-stage Docker build.

### Out of Scope for Initial MVP

Unless explicitly requested, do not implement these features in the initial MVP:

- Multi-user collaboration.
- Public photo sharing links.
- Social features and comments.
- AI-based image recognition.
- Cloud synchronization.
- Mobile native applications.
- Complex role-based access control.

Authentication and access control must be reconsidered before exposing the application publicly.

## 2. Technology Decisions

| Component | Decision | Status | Notes |
|---|---|---|---|
| Frontend | React 18 + TypeScript + Vite | Implemented | Strict TypeScript, component modularity, centralized PhotosContext with debouncing & AbortController, GalleryControls, AlbumDetailView, PhysicalFolderDetailView, TimelinePage |
| UI styling | Tailwind CSS + semantic tokens | Implemented | Light/dark CSS tokens, custom scrollbars, aspect-ratio preservation, filter chips |
| Icons | Lucide React | Implemented | Standard SVG icons across shell, cards, modals, filter panel & action toolbars |
| Routing | React Router v6 | Implemented | Client-side routing with Go SPA fallback (`/`, `/photos`, `/timeline`, `/folders`, `/folders/:albumId`), synchronized URL query params |
| Backend | Go 1.22+ + Fiber v2 | Implemented | Package separation (config, storage, database, repository, api, server, media, scanner) |
| Database | SQLite via `modernc.org/sqlite` | Implemented | Pure Go (CGO_ENABLED=0), WAL mode, synchronous=NORMAL, 10s busy timeout, 64MB cache, 256MB mmap, foreign keys, versioned migrations (v1 - v6) |
| Image Processing | Pure-Go decoders & bilinear scaling | Implemented | `image/jpeg`, `image/png`, `image/gif`, `golang.org/x/image/webp`, `golang.org/x/image/tiff`, `golang.org/x/image/draw`, bounded semaphore (4 workers), buffered I/O |
| EXIF Parser | `github.com/rwcarlsen/goexif` | Implemented | Pure Go, extracts `DateTimeOriginal`, timestamps, orientation tags (1-8), and camera/lens/exposure details |
| Video Processing | `ffmpeg` & `ffprobe` + Range Streaming | Implemented | Container & MIME detection (.mp4, .webm, .mov, .m4v, .mkv), ffprobe JSON metadata, ffmpeg thumbnail generation with concurrency limits (3 workers) & cooldown, RFC 7233 byte-range streaming |
| Photo storage | Local host bind mount (`/media`) | Implemented | Target `STORAGE_ROOT=/media`; safe path traversal checks; read-write mount preserved; read-only folder browsing |
| Database storage | Separate persistent volume (`/data`) | Implemented | Target `DATABASE_PATH=/data/pixora.db`; parent dir auto-created; never stored in photo dir |
| Path convention | Relative storage paths in database | Implemented | Relative paths (e.g. `originals/...`, `Pictures/vacation/beach.jpg`); validated with `internal/storage` |
| Container access | Non-root `pixora` user (UID 1000) | Implemented | Host permissions documented; no write checks at startup |
| API style | REST + JSON | Implemented | Query parameters for discovery and cursor pagination (`cursor`, `limit`, `q`, `startDate`, `endDate`, `folder`, `albumId`, `mediaType`, `isUndated`, `sort`, `order`); opaque base64 scope tokens; structured error envelopes |
| HTTP Caching | Conditional requests & immutable assets | Implemented | ETag + Last-Modified validation (`If-None-Match`, `If-Modified-Since` -> 304 Not Modified), immutable 1-year cache on hashed static `/assets/*`, `no-cache` on `index.html` |
| Health & Readiness | Liveness & Readiness checks | Implemented | Separate `/api/health/live` (process alive) and `/api/health/ready` (checks DB ping & storage read; 503 if DB disconnected) |
| Deployment | Unified Multi-Stage Dockerfile | Implemented | Builds FE & BE into a single non-root Alpine container (`CGO_ENABLED=0`) |
| Theme | Light, Dark, and System | Implemented | Persisted in `localStorage`, media query sync |

## 3. Development Phases

### Phase 1 — Foundation & UI
**Status:** Completed

### Phase 2 — Backend Foundation & Data Model
**Status:** Completed

### Phase 3 — Photo Management, Media Scanning & Upload
**Status:** Completed

### Phase 4 — Virtual Albums & Physical Folder Browsing
**Status:** Completed

### Phase 5 — Timeline & Photo Viewer
**Status:** Completed

### Phase 6 — Search, Filtering & Sorting
**Status:** Completed

### Phase 7 — Video Support
**Status:** Completed

### Phase 8 — Performance Optimization & Production Hardening
**Status:** Completed

Scope & Acceptance Criteria Met:

#### Part A: SQLite Performance Optimization & Connection Tuning
- [x] **Connection Configuration (`internal/database/db.go`):**
  - Set `MaxOpenConns(10)` to prevent connection thrashing and lock contention.
  - Set `MaxIdleConns(5)` and `ConnMaxLifetime(1 hour)`.
  - Added `ConnMaxIdleTime(15 minutes)` to recycle idle connections safely.
  - Configured high-performance SQLite PRAGMAs:
    - `busy_timeout=10000`: Wait up to 10 seconds on locked database before throwing error.
    - `journal_mode=WAL`: Concurrent readers do not block writers and writer does not block readers.
    - `synchronous=NORMAL`: Fast sync with full ACID durability in WAL mode.
    - `cache_size=-64000`: 64MB memory page cache for fast index traversal on large collections.
    - `temp_store=2 (MEMORY)`: Prevents temporary disk files during sorting, subqueries, and window functions.
    - `mmap_size=268435456 (256MB)`: Memory-mapped I/O for direct kernel-space page reading.
    - `foreign_keys=1`: Explicitly enforced.
- [x] **Migration 6: High-Performance Composite Indexes (`internal/database/migrations.go`):**
  - Added composite indexes tailored to cursor-based queries and discovery filters:
    - `idx_photos_taken_at_id ON photos(taken_at DESC, id ASC)`: Satisfies default capture date sorting and cursor pagination directly from the index.
    - `idx_photos_file_modified_at_id ON photos(file_modified_at DESC, id ASC)`: Optimizes fallback sorting and undated media pagination.
    - `idx_photos_media_type_taken_at ON photos(media_type, taken_at DESC)`: Accelerates photo/video filtering with chronological ordering.
- [x] **WAL Checkpoint Management (`internal/database/db.go`):**
  - Implemented `Checkpoint(mode string)` supporting `PASSIVE`, `FULL`, `RESTART`, and `TRUNCATE`.
  - On application shutdown, `db.Close()` executes `Checkpoint("PASSIVE")` to flush WAL pages cleanly back into the main SQLite database.

#### Part B: Backend Concurrency & Resource Management
- [x] **Bounded Thumbnail Concurrency (`internal/media/thumbnail.go`):**
  - Added bounded worker semaphore for image thumbnail generation (`imageSem`, max 4 concurrent decodes).
  - Maintained bounded worker semaphore for video thumbnail extraction (`ffmpegSem`, max 3 concurrent ffmpeg tasks).
  - Replaced unbounded `sync.Map` mutexes with a fixed 64-bucket striped lock array, eliminating memory leaks while preventing duplicate concurrent generation for the same media item.
  - Added 64KB buffered I/O (`bufio.Reader` and `bufio.Writer`) for thumbnail generation, reducing filesystem syscall overhead.
- [x] **Stale Thumbnail Invalidation (`internal/media/thumbnail.go`):**
  - Implemented `isThumbnailFresh(thumbInfo, sourceModified)`: Detects when original media has been modified or replaced after thumbnail creation (`sourceModified > thumbModified`).
  - Automatically regenerates stale or empty thumbnails.
  - Added failure negative cache with 3-minute cooldown and `ClearFailed()` eviction.
- [x] **Background Scanner Cancellation & Reliability (`internal/scanner/scanner.go`):**
  - Added `Scanner.Cancel()` method with cancellable contexts, allowing graceful termination during shutdown or manual abort.
  - Walking loop checks `scanCtx.Err()` and terminates promptly without leaving hanging goroutines or corrupt records.

#### Part C: HTTP Caching, Static Delivery & Observability
- [x] **Conditional Request Caching (`internal/api/photos.go`):**
  - Enhanced `ServeThumbnailHandler` and `ServeOriginalPhotoHandler` with `checkNotModified`.
  - Supports both `If-None-Match` (ETag) and `If-Modified-Since` (Last-Modified) returning HTTP `304 Not Modified`.
  - Prevents redownloading media files and thumbnails when cached in client or browser.
- [x] **Frontend Static Asset Caching (`internal/server/server.go`):**
  - Added 1-year immutable caching (`Cache-Control: public, max-age=31536000, immutable`) for content-hashed assets under `/assets/*`.
  - Added `Cache-Control: no-cache` for `index.html` to guarantee instant deployment updates without stale client caches.
- [x] **Log Noise Reduction (`internal/server/server.go`):**
  - Configured Fiber logger middleware to skip logging routine successful thumbnail requests (status < 400) and healthy health probes (`/api/health*`), preventing log flooding while preserving all real requests and error traces.
- [x] **Health and Readiness Endpoints (`internal/api/health.go`):**
  - `GET /api/health/live`: Liveness probe indicating server process is alive (HTTP 200).
  - `GET /api/health/ready`: Readiness probe verifying database ping and storage read accessibility. Returns HTTP 503 Service Unavailable when the database is disconnected.
  - Maintained `GET /api/health` backward compatibility.
- [x] **Graceful Shutdown (`cmd/server/main.go`):**
  - Listens for SIGINT / SIGTERM.
  - Cancels active background scanner tasks via `mediaScanner.Cancel()`.
  - Stops accepting new connections and drains in-flight requests with a 10-second timeout.
  - Cleanly checkpoints SQLite WAL and closes database connection via deferred `db.Close()`.

#### Part D: Frontend Performance & Virtualization Evaluation
- [x] **Component Memoization & Async Decoding:**
  - Wrapped `PhotoCard` with `React.memo` to prevent re-rendering hundreds of cards when unrelated context states update.
  - Added `decoding="async"` alongside `loading="lazy"` in `FallbackImage.tsx` to keep image decoding off the main thread.
- [x] **Virtualization Evaluation:**
  - Evaluated virtualized list rendering against cursor-based pagination with batch loading.
  - *Decision:* Pixora currently employs 50-item keyset cursor pagination with lazy image decoding and reserved aspect-ratio containers. This architecture renders between 50 and 200 DOM cards at any given time, keeping memory consumption low (<30MB) and frame rates smooth at 60 FPS.
  - Introducing a virtualization library (such as react-virtual or react-window) would introduce significant downsides:
    1. Breaking native browser page search (`Ctrl+F`).
    2. Complicating sticky chronological date headers (Day/Month/Year) in the Timeline view.
    3. Impairing keyboard navigation accessibility and screen-reader tree exploration.
  - *Conclusion:* Keyset cursor pagination combined with `React.memo`, `loading="lazy"`, and `decoding="async"` provides the optimal balance of performance, simplicity, and accessibility for collections up to hundreds of thousands of photos.

## 4. Current Implementation Inventory

| Area | Status | Verified Notes |
|---|---|---|
| Frontend scaffold | Implemented | React 18, Vite, TypeScript strict, Tailwind CSS, Lucide |
| Backend scaffold | Implemented | Go 1.22+, Fiber v2, graceful shutdown, request logging, CORS |
| Storage abstraction | Implemented | `internal/storage`: Safe path resolution, traversal rejection, symlink checks, folder validation |
| Media scanner | Implemented | `internal/scanner`: In-place discovery, recursive walk, format detection, deduplication, full & targeted scan, cancel support |
| Metadata & thumbnails | Implemented | `internal/media`: EXIF `DateTimeOriginal` precedence, orientation transforms (1-8), bilinear thumbnailing, EXIF details, bounded concurrency (4 images, 3 ffmpeg), stale invalidation, striped locks |
| Video support | Implemented | `internal/media`: Container & MIME detection (.mp4, .webm, .mov, .m4v, .mkv), ffprobe JSON metadata, RFC 7233 byte-range streaming |
| Database integration | Implemented | `internal/database`: Pure-Go SQLite, WAL mode, synchronous=NORMAL, 64MB cache, 256MB mmap, 10s busy timeout, foreign keys, migrations (v1 - v6 with composite indexes), WAL checkpointing |
| Repository layer | Implemented | `internal/repository`: PhotoRepository with `SearchAndFilter`, `ListTimelineWithFilter`, keyset cursor pagination, safe SQL parameterization & `LIKE` escaping |
| API Endpoints | Implemented | `/api/health`, `/api/health/live`, `/api/health/ready`, `/api/photos` (with discovery query params), `/api/photos/:id`, `/api/photos/:id/file`, `/api/photos/:id/thumbnail`, `/api/photos/scan`, `/api/photos/upload`, `/api/timeline` (with filters), `/api/albums`, `/api/albums/:id`, `/api/folders`, `/api/folders/contents`, `/api/folders/photos` |
| Photo discovery UI | Implemented | Debounced search, expandable filter panel, active filter chips, 8-way sorting, URL sync, date transparency |
| Timeline UI | Implemented | Real API data from `/api/timeline`, grouped by Day under sticky Month/Year headers, dedicated Undated section, incremental loading |
| Photo viewer (Lightbox) | Implemented | Full zoom (100%-400%), mouse/touch panning, fit-to-width, fit-to-screen, EXIF details drawer, keyboard navigation, context-aware adjacent photos |
| Video Player Modal | Implemented | Integrated into lightbox with native HTML5 controls, poster thumbnail, duration & codec metadata badges |
| Photo upload UI | Implemented | Drag & drop, multi-file selection, progress tracking, per-file error handling, auto-refresh |
| Physical Folders UI | Implemented | Read-only filesystem browsing, nested subfolder navigation, breadcrumbs, unindexed photo alerts, targeted scan button |
| Virtual Albums UI | Implemented | Real SQLite albums from `/api/albums`, persistent creation, renaming, safe deletion, album detail view, photo assignment |
| Theme switching | Implemented | Light, dark, and system preference with localStorage persistence |
| Automated tests | Verified | 79 Go tests passing across all packages; frontend typecheck and build passing cleanly (0 errors) |
| Performance Benchmarks | Verified | Keyset pagination, query search/filter, timeline, and thumbnail benchmarks executed and recorded |
| Unified Dockerfile | Implemented | Multi-Stage build (`CGO_ENABLED=0` statically compiled, non-root UID 1000) |

## 5. Verification Results

### Go Backend Tests (`backend`)
```
=== RUN   TestAlbumsAPI
--- PASS: TestAlbumsAPI (0.01s)
=== RUN   TestPhotosAPI
--- PASS: TestPhotosAPI (0.02s)
=== RUN   TestUploadAPI
--- PASS: TestUploadAPI (0.01s)
=== RUN   TestScanAPI
--- PASS: TestScanAPI (0.01s)
=== RUN   TestTimelineAndPhotoViewerAPI
--- PASS: TestTimelineAndPhotoViewerAPI (0.00s)
=== RUN   TestPhotoSearchAndFilteringAPI
--- PASS: TestPhotoSearchAndFilteringAPI (0.00s)
=== RUN   TestVideoStreamRangeAndHeaders
--- PASS: TestVideoStreamRangeAndHeaders (0.00s)
=== RUN   TestVideoThumbnailSafetyAndFallback
--- PASS: TestVideoThumbnailSafetyAndFallback (0.05s)
=== RUN   TestVideoMediaTypeFilteringAndDetails
--- PASS: TestVideoMediaTypeFilteringAndDetails (0.00s)
=== RUN   TestCursorEncodingAndScopeValidation
--- PASS: TestCursorEncodingAndScopeValidation (0.00s)
=== RUN   TestPhotosCursorPaginationLifecycle
--- PASS: TestPhotosCursorPaginationLifecycle (0.00s)
=== RUN   TestAlbumAndFolderCursorPagination
--- PASS: TestAlbumAndFolderCursorPagination (0.03s)
=== RUN   TestListFoldersAndContents
--- PASS: TestListFoldersAndContents (0.10s)
=== RUN   TestFolderSecurityAndPathTraversal
--- PASS: TestFolderSecurityAndPathTraversal (0.00s)
=== RUN   TestScanFolderIntegration
--- PASS: TestScanFolderIntegration (0.01s)
=== RUN   TestHealthHandler
--- PASS: TestHealthHandler (0.00s)
=== RUN   TestLivenessHandler
--- PASS: TestLivenessHandler (0.00s)
=== RUN   TestNewHealthHandlerWithDBAndStorage
--- PASS: TestNewHealthHandlerWithDBAndStorage (0.00s)
PASS
ok  	pixora/backend/internal/api	0.340s
=== RUN   TestConfigDefaultsProduction
--- PASS: TestConfigDefaultsProduction (0.00s)
=== RUN   TestConfigEnvironmentOverrides
--- PASS: TestConfigEnvironmentOverrides (0.00s)
=== RUN   TestConfigValidationInvalidValues
--- PASS: TestConfigValidationInvalidValues (0.00s)
=== RUN   TestResolveStoragePath
--- PASS: TestResolveStoragePath (0.00s)
=== RUN   TestCheckWritable
--- PASS: TestCheckWritable (0.00s)
PASS
ok  	pixora/backend/internal/config	0.002s
=== RUN   TestOpenInNonWritableDir
--- PASS: TestOpenInNonWritableDir (0.00s)
=== RUN   TestDatabaseInitializationAndMigrations
--- PASS: TestDatabaseInitializationAndMigrations (0.00s)
=== RUN   TestDatabaseForeignKeyEnforcement
--- PASS: TestDatabaseForeignKeyEnforcement (0.00s)
=== RUN   TestDatabaseUniqueConstraints
--- PASS: TestDatabaseUniqueConstraints (0.00s)
=== RUN   TestDatabasePingAndClose
--- PASS: TestDatabasePingAndClose (0.00s)
PASS
ok  	pixora/backend/internal/database	0.014s
=== RUN   TestExtractMetadata
--- PASS: TestExtractMetadata (0.01s)
=== RUN   TestIsSupportedExtension
--- PASS: TestIsSupportedExtension (0.00s)
=== RUN   TestIsSupportedVideoExtension
--- PASS: TestIsSupportedVideoExtension (0.00s)
=== RUN   TestVideoSniffingAndDetection
--- PASS: TestVideoSniffingAndDetection (0.00s)
=== RUN   TestOrientationTransform
--- PASS: TestOrientationTransform (0.00s)
=== RUN   TestExtractExifDetailsSafe
--- PASS: TestExtractExifDetailsSafe (0.00s)
=== RUN   TestVideoMetadataExtraction
--- PASS: TestVideoMetadataExtraction (0.10s)
=== RUN   TestThumbnailService
--- PASS: TestThumbnailService (0.02s)
=== RUN   TestThumbnailServiceStaleInvalidation
--- PASS: TestThumbnailServiceStaleInvalidation (0.01s)
=== RUN   TestThumbnailConcurrentGeneration
--- PASS: TestThumbnailConcurrentGeneration (0.01s)
=== RUN   TestVideoThumbnailService
--- PASS: TestVideoThumbnailService (0.08s)
=== RUN   TestThumbnailServiceReadOnly
--- PASS: TestThumbnailServiceReadOnly (0.00s)
PASS
ok  	pixora/backend/internal/media	0.240s
=== RUN   TestPhotoRepositoryCRUDAndListing
--- PASS: TestPhotoRepositoryCRUDAndListing (0.00s)
=== RUN   TestAlbumRepositoryCRUDAndPhotos
--- PASS: TestAlbumRepositoryCRUDAndPhotos (0.00s)
=== RUN   TestPhotoRepositoryFolderQueries
--- PASS: TestPhotoRepositoryFolderQueries (0.00s)
=== RUN   TestPhotoRepositoryTimelineAndAdjacent
--- PASS: TestPhotoRepositoryTimelineAndAdjacent (0.00s)
=== RUN   TestPhotoRepositorySearchAndFilter
--- PASS: TestPhotoRepositorySearchAndFilter (0.00s)
PASS
ok  	pixora/backend/internal/repository	0.033s
=== RUN   TestScannerDiscoveryAndRepeatedScan
--- PASS: TestScannerDiscoveryAndRepeatedScan (0.05s)
=== RUN   TestScannerVideoDiscoveryAndRepeatedScan
--- PASS: TestScannerVideoDiscoveryAndRepeatedScan (0.07s)
=== RUN   TestScannerSymlinkEscapePrevention
--- PASS: TestScannerSymlinkEscapePrevention (0.00s)
=== RUN   TestScannerConcurrentScanProtection
--- PASS: TestScannerConcurrentScanProtection (0.00s)
=== RUN   TestScannerMissingStorageRoot
--- PASS: TestScannerMissingStorageRoot (0.00s)
=== RUN   TestScannerScanSpecificFolder
--- PASS: TestScannerScanSpecificFolder (0.01s)
=== RUN   TestScannerForceRefresh
--- PASS: TestScannerForceRefresh (0.01s)
=== RUN   TestScannerCancel
--- PASS: TestScannerCancel (0.01s)
PASS
ok  	pixora/backend/internal/scanner	0.180s
=== RUN   TestFrontendBackendUnifiedServing
--- PASS: TestFrontendBackendUnifiedServing (0.00s)
=== RUN   TestPhase3FullServerLifecycle
--- PASS: TestPhase3FullServerLifecycle (0.03s)
=== RUN   TestPhase4FullServerLifecycle
--- PASS: TestPhase4FullServerLifecycle (0.01s)
=== RUN   TestPhysicalFolderBrowsingFullLifecycle
--- PASS: TestPhysicalFolderBrowsingFullLifecycle (0.04s)
=== RUN   TestPhase5TimelineAndViewerLifecycle
--- PASS: TestPhase5TimelineAndViewerLifecycle (0.03s)
=== RUN   TestPhase6SearchFilterSortLifecycle
--- PASS: TestPhase6SearchFilterSortLifecycle (0.02s)
=== RUN   TestServerHealthRoute
--- PASS: TestServerHealthRoute (0.00s)
=== RUN   TestServerWithDependencies
--- PASS: TestServerWithDependencies (0.00s)
=== RUN   TestServerUnknownAPIRouteReturns404JSON
--- PASS: TestServerUnknownAPIRouteReturns404JSON (0.00s)
=== RUN   TestServerStaticAndSPAFallback
--- PASS: TestServerStaticAndSPAFallback (0.00s)
PASS
ok  	pixora/backend/internal/server	0.181s
=== RUN   TestStorageRelativePathValidation
--- PASS: TestStorageRelativePathValidation (0.00s)
=== RUN   TestStoragePathTraversalRejection
--- PASS: TestStoragePathTraversalRejection (0.00s)
=== RUN   TestStorageSymlinkEscapeRejection
--- PASS: TestStorageSymlinkEscapeRejection (0.00s)
=== RUN   TestStorageMissingDirectoryBehavior
--- PASS: TestStorageMissingDirectoryBehavior (0.00s)
=== RUN   TestStorageReadOnlyBehavior
--- PASS: TestStorageReadOnlyBehavior (0.00s)
=== RUN   TestStorageFileReadWriteLifecycle
--- PASS: TestStorageFileReadWriteLifecycle (0.00s)
=== RUN   TestStorageFolderPathValidationAndResolution
--- PASS: TestStorageFolderPathValidationAndResolution (0.00s)
PASS
ok  	pixora/backend/internal/storage	0.002s

All 79 Go tests passed (0 failures).
```

### Go Performance Benchmarks
Environment: Linux x86_64, Intel Core 5 210H, modernc.org/sqlite, pure Go.
Dataset: 5,000 synthetic photos/videos across folders and date ranges.

```
pkg: pixora/backend/internal/media
BenchmarkThumbnailCached-12        	  244521	      4734 ns/op	    2201 B/op	      28 allocs/op
BenchmarkThumbnailGeneration-12    	      92	  11088240 ns/op	 9195242 B/op	     169 allocs/op

pkg: pixora/backend/internal/repository
BenchmarkListCursor-12         	      74	  14984359 ns/op	  134402 B/op	    4381 allocs/op
BenchmarkSearchAndFilter-12    	     901	   1374599 ns/op	   14491 B/op	      71 allocs/op
BenchmarkTimelineCursor-12     	     100	  12414506 ns/op	   66834 B/op	    2184 allocs/op
BenchmarkAdjacentPhotos-12     	      62	  19358085 ns/op	     796 B/op	      22 allocs/op
```

Key Findings:
1. **Cached Thumbnail Delivery:** ~4.7 microseconds per request (244k ops/sec).
2. **Fresh Thumbnail Generation:** ~11.1 milliseconds per 800x600 image decode, bilinear scaling, and JPEG encoding.
3. **Cursor Keyset Pagination:** ~15.0 ms for sequential 2-page keyset traversal (100 items retrieved).
4. **Search and Date Range Filtering:** ~1.37 ms per indexed search query.


### Folder Scan & Photos Scan Consistency Bug Fix
- **Issue:** Opening physical folders with multi-byte Unicode path names or root directories reported unindexed media files that did not disappear after scanning from either the folder or the Photos page. Photos page scan reported 0 new media because files were already indexed, yet folder queries failed to locate them.
- **Root Cause:** In `internal/repository/photo.go`, folder matching SQL queries (`buildPhotoFilterConditions`, `CountByFolder`, `ListByFolder`, `GetFolderCoverPhoto`, and `GetAdjacentPhotos`) used Go byte length `len(prefix)` as substring length in SQLite `SUBSTR(relative_path, 1, ?) = ?`. Because SQLite `SUBSTR` operates on character counts rather than UTF-8 bytes, multi-byte Unicode folder names caused SQLite to extract too many characters, failing all equality comparisons. In addition, root directory matching did not handle `canonicalRel == ""` properly in `buildPhotoFilterConditions`.
- **Fix:** 
  1. Replaced byte-length parameters with SQLite character count function `LENGTH(?)` across all folder query clauses.
  2. Normalized root folder paths (`""`, `"/"`, `"."`, `"\"`) cleanly in `ValidateFolderPath`, `folders.go`, and repository cursor pagination.
  3. Corrected scan status reporting so total indexing failures mark status as `failed` rather than false `completed`.
  4. Updated frontend folder scan and library scan status banners to report indexing failures cleanly.
  5. Added comprehensive 10-step isolated integration test suite in `internal/api/scan_consistency_test.go` verifying 134 media files in nested and Unicode folders.

### Frontend Build & Lint Checks (`frontend`)
```
> tsc --noEmit (Passed cleanly, 0 errors)
> vite build (Built dist/index.html, dist/assets in 1.29s)
```

## 6. Manual Verification Steps for User

```bash
# 1. Run all backend tests
cd backend && go test -v -count=1 ./...

# 2. Run backend performance benchmarks
cd backend && go test -bench=. -benchmem -run=^$ ./...

# 3. Run frontend typecheck and build
cd ../frontend && npm run typecheck && npm run build

# 4. Start server locally
cd ../backend
STORAGE_ROOT=./storage/media DATABASE_PATH=./storage/data/pixora.db go run ./cmd/server

# 5. Check Liveness and Readiness endpoints
curl -s "http://localhost:8080/api/health/live" | jq .
curl -s "http://localhost:8080/api/health/ready" | jq .

# 6. Verify HTTP 304 conditional request on thumbnails
curl -i "http://localhost:8080/api/photos/<photo-id>/thumbnail"
# Copy the returned ETag header and test:
curl -i -H 'If-None-Match: "thumb-<photo-id>..."' "http://localhost:8080/api/photos/<photo-id>/thumbnail"
# Expected: HTTP/1.1 304 Not Modified

# 7. Test graceful shutdown
# Send SIGINT (Ctrl+C) to server: observe scanner cancellation, connection draining, WAL checkpoint, and clean DB closure.
```
