# Pixora — Project Status

## 1. Project Overview

**Project:** Pixora  
**Description:** A web-based self-hosted photo gallery for uploading photos, organizing physical folders and virtual albums, searching, filtering, and browsing photos chronologically.

**Current overall status:** Phase 6 — Search, Filtering & Sorting  
**Current phase:** Phase 6 Completed (Ready for Hardening & Production Deployment)

### Product Goals

- Provide a fast, responsive, and accessible photo gallery UI.
- Support uploading one or multiple photos safely.
- Discover and index existing photos in mounted host storage (`/media`).
- Support read-only physical folder and nested subfolder browsing directly from `/media`.
- Detect unindexed media files on disk with targeted one-click scanning.
- Organize photos into virtual collections and albums without moving original files.
- Browse photos chronologically using capture dates when available.
- View full-resolution photos in a responsive lightbox with EXIF metadata.
- Search indexed photos by filename, relative folder path, and metadata with case-insensitivity and wildcard safety.
- Filter photos flexibly by date range, physical folder (direct/recursive), virtual album, media format, and undated status.
- Sort photos deterministically by capture date (newest/oldest), filename, file size, or modification date.
- Store photos and metadata reliably on local storage with host bind mounts.
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
| Database | SQLite via `modernc.org/sqlite` | Implemented | Pure Go (CGO_ENABLED=0), WAL mode, foreign keys, versioned migrations (v1, v2, v3) |
| Image Processing | Pure-Go decoders & bilinear scaling | Implemented | `image/jpeg`, `image/png`, `image/gif`, `golang.org/x/image/webp`, `golang.org/x/image/tiff`, `golang.org/x/image/draw` |
| EXIF Parser | `github.com/rwcarlsen/goexif` | Implemented | Pure Go, extracts `DateTimeOriginal`, timestamps, orientation tags (1-8), and camera/lens/exposure details |
| Photo storage | Local host bind mount (`/media`) | Implemented | Target `STORAGE_ROOT=/media`; safe path traversal checks; read-write mount preserved; read-only folder browsing |
| Database storage | Separate persistent volume (`/data`) | Implemented | Target `DATABASE_PATH=/data/pixora.db`; parent dir auto-created; never stored in photo dir |
| Path convention | Relative storage paths in database | Implemented | Relative paths (e.g. `originals/...`, `Pictures/vacation/beach.jpg`); validated with `internal/storage` |
| Container access | Non-root `pixora` user (UID 1000) | Implemented | Host permissions documented; no write checks at startup |
| API style | REST + JSON | Implemented | Query parameters for discovery (`q`, `startDate`, `endDate`, `folder`, `albumId`, `mediaType`, `isUndated`, `sort`, `order`); structured error envelopes |
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

Scope & Acceptance Criteria Met:

#### Part A: Database Schema & Migration 3
- [x] **Database Performance Indexes (`internal/database/migrations.go`):**
  - Migration 3 creates specialized SQLite indexes for high-performance indexed queries:
    - `idx_photos_original_filename_nocase ON photos(original_filename COLLATE NOCASE);`
    - `idx_photos_media_type ON photos(media_type);`
    - `idx_photos_file_modified_at ON photos(file_modified_at);`
    - `idx_photos_file_size ON photos(file_size);`
  - Reuses existing index `idx_photos_taken_at` for capture date sorting and filtering.
  - Safe, backward-compatible migration within atomic database transactions.

#### Part B: Repository Layer Discovery Engine (`internal/repository/photo.go`)
- [x] **Structured Query Criteria (`PhotoFilter`):**
  - Search query `Query`: searches both `original_filename` and relative directory paths safely.
  - Wildcard escaping: `escapeLike` escapes `%`, `_`, and `\` using `ESCAPE '\\'`, preventing user inputs like `100%` or `_v2` from acting as wildcards.
  - Case-insensitive substring matching using parameterized SQL (`?`).
  - Date range filtering (`StartDate`, `EndDate`): applies strictly to verified capture dates (`taken_at`), parsed safely as UTC boundaries.
  - Physical folder filtering (`Folder`, `Recursive`): supports direct folder queries (photos immediately inside folder) and recursive subfolder filtering without SQL string concatenation.
  - Virtual album filtering (`AlbumID`): filters photos by album membership via `photo_albums` join.
  - Media format filtering (`MediaType`): filters by indexed MIME types (e.g., `image/jpeg`, `image/png`, `image/webp`).
  - Undated filter (`IsUndated`): isolates photos lacking EXIF capture timestamps (`taken_at IS NULL`).
- [x] **Deterministic Sorting (`SearchAndFilter` & `ListTimelineWithFilter`):**
  - Supports sorting options: `date` (`taken_at`), `filename` (`original_filename COLLATE NOCASE`), `size` (`file_size`), and `modified` (`file_modified_at`).
  - Stable tie-breaking: every sort query appends `id ASC` as a secondary deterministic key.
  - Undated photo semantics: when sorting by capture date (`date DESC`), photos with verified capture dates are ordered newest first, while photos lacking EXIF timestamps (`taken_at IS NULL`) are deterministically ordered at the end by `file_modified_at DESC, id ASC`.

#### Part C: REST API Endpoints & Parameter Validation (`internal/api/photos.go`)
- [x] **Enhanced Listing & Timeline Endpoints:**
  - `GET /api/photos` — supports `q`, `startDate`, `endDate`, `folder`, `recursive`, `albumId`, `mediaType`, `isUndated`, `sort` (or `sortBy`), `order` (or `sortOrder`), `limit`, and `offset`.
  - `GET /api/photos/timeline` — supports the same filtering parameters while preserving strict chronological grouping.
- [x] **Input Validation & Security Guardrails:**
  - Date format validation: verifies `YYYY-MM-DD` or RFC3339 timestamps; returns 400 Bad Request for malformed dates.
  - Date boundary validation: verifies `startDate <= endDate`; returns 400 Bad Request with a clear message if `startDate > endDate`.
  - Path traversal protection: validates `folder` parameters through `storage.ValidateFolderPath`, rejecting `..`, `../`, null bytes, and absolute paths with 400 Bad Request.
  - Sort option whitelisting: rejects unrecognized sort keys or orders with 400 Bad Request.
  - Safe parameterization: zero raw SQL concatenation; full defense against SQL injection.

#### Part D: Frontend Integration & User Experience
- [x] **Context Discovery Layer (`PhotosContext.tsx`):**
  - Centralized discovery state: search query, date range, folder filter, album filter, media type filter, undated toggle, and sort option.
  - 300ms search input debouncing to prevent excessive server requests during fast typing.
  - Race condition prevention: uses `AbortController` to cancel in-flight HTTP requests when new inputs occur, ensuring slower older responses never overwrite newer results.
  - Reset helpers: `clearFilters` (resets filter panel inputs) and `resetAll` (resets search and all filters).
- [x] **Interactive Controls Toolbar & Filter Panel (`GalleryControls.tsx`):**
  - Expandable filter drawer with Sliders icon and active filter count badge.
  - Start Date & End Date range pickers with real-time range validation warning.
  - Physical Folder dropdown populated dynamically from available `/media` directories.
  - Virtual Album dropdown populated dynamically from user's custom albums.
  - Media Format selector (`JPEG`, `PNG`, `WebP`, `GIF`, `TIFF`).
  - Undated photos toggle switch.
  - Sort selector with 8 options: Newest Capture Date, Oldest Capture Date, Filename (A-Z / Z-A), File Size (Largest / Smallest), Modification Date (Newest / Oldest).
  - Active filter chips bar showing individual dismiss buttons (`x`) and a \"Clear all\" trigger.
  - Total matching photos count badge.
- [x] **URL Synchronization & Deeplinking (`PhotosPage.tsx`):**
  - Synchronizes active filters with URL query parameters (`?q=...&startDate=...&folder=...&sort=...`) using `useSearchParams`.
  - Bookmarkable search queries and state preservation across browser navigation.
  - Preserves search and filter state when returning from the Photo Viewer (Lightbox).
- [x] **Date Transparency & Fallback Semantics (`PhotoCard.tsx`):**
  - Displays verified EXIF capture dates with standard formatting.
  - Explicitly prefixes fallback filesystem modification dates with `Mod: <date>` and distinct styling so users are never misled.
  - Tooltips clearly describe the timestamp origin (`Verified EXIF capture date` vs `Filesystem modification time (fallback, no EXIF capture date)`).
- [x] **Empty & Error States (`EmptyState.tsx`):**
  - Distinct empty states for search queries vs active filter criteria with one-click reset action.

## 4. Current Implementation Inventory

| Area | Status | Verified Notes |
|---|---|---|
| Frontend scaffold | Implemented | React 18, Vite, TypeScript strict, Tailwind CSS, Lucide |
| Backend scaffold | Implemented | Go 1.22+, Fiber v2, graceful shutdown, request logging, CORS |
| Storage abstraction | Implemented | `internal/storage`: Safe path resolution, traversal rejection, symlink checks, folder validation |
| Media scanner | Implemented | `internal/scanner`: In-place discovery, recursive walk, format detection, deduplication, full & targeted scan |
| Metadata & thumbnails | Implemented | `internal/media`: EXIF `DateTimeOriginal` precedence, orientation transforms (1-8), bilinear thumbnailing, EXIF details |
| Database integration | Implemented | `internal/database`: Pure-Go SQLite, WAL mode, foreign keys, migrations (v1, v2, v3 with search indexes) |
| Repository layer | Implemented | `internal/repository`: PhotoRepository with `SearchAndFilter`, `ListTimelineWithFilter`, safe SQL parameterization & `LIKE` escaping |
| API Endpoints | Implemented | `/api/health`, `/api/photos` (with discovery query params), `/api/photos/:id`, `/api/photos/:id/file`, `/api/photos/:id/thumbnail`, `/api/photos/scan`, `/api/photos/upload`, `/api/timeline` (with filters), `/api/albums`, `/api/albums/:id`, `/api/folders`, `/api/folders/contents`, `/api/folders/photos` |
| Photo discovery UI | Implemented | Debounced search, expandable filter panel, active filter chips, 8-way sorting, URL sync, date transparency |
| Timeline UI | Implemented | Real API data from `/api/timeline`, grouped by Day under sticky Month/Year headers, dedicated Undated section, incremental loading |
| Photo viewer (Lightbox) | Implemented | Full zoom (100%-400%), mouse/touch panning, fit-to-width, fit-to-screen, EXIF details drawer, keyboard navigation, context-aware adjacent photos |
| Photo upload UI | Implemented | Drag & drop, multi-file selection, progress tracking, per-file error handling, auto-refresh |
| Physical Folders UI | Implemented | Read-only filesystem browsing, nested subfolder navigation, breadcrumbs, unindexed photo alerts, targeted scan button |
| Virtual Albums UI | Implemented | Real SQLite albums from `/api/albums`, persistent creation, renaming, safe deletion, album detail view, photo assignment |
| Theme switching | Implemented | Light, dark, and system preference with localStorage persistence |
| Automated tests | Verified | 55 Go tests passing across all packages; frontend typecheck and build passing cleanly (0 errors) |
| Unified Dockerfile | Implemented | Multi-Stage build (`CGO_ENABLED=0` statically compiled, non-root UID 1000) |

## 5. Verification Results

### Go Backend Tests (`backend`)
```
=== RUN   TestAlbumsAPI
--- PASS: TestAlbumsAPI (0.01s)
=== RUN   TestPhotosAPI
--- PASS: TestPhotosAPI (0.02s)
=== RUN   TestPhotoSearchAndFilteringAPI
--- PASS: TestPhotoSearchAndFilteringAPI (0.01s)
=== RUN   TestUploadAPI
--- PASS: TestUploadAPI (0.01s)
=== RUN   TestScanAPI
--- PASS: TestScanAPI (0.01s)
=== RUN   TestTimelineAndPhotoViewerAPI
--- PASS: TestTimelineAndPhotoViewerAPI (0.00s)
=== RUN   TestListFoldersAndContents
--- PASS: TestListFoldersAndContents (0.11s)
=== RUN   TestFolderSecurityAndPathTraversal
--- PASS: TestFolderSecurityAndPathTraversal (0.00s)
=== RUN   TestScanFolderIntegration
--- PASS: TestScanFolderIntegration (0.02s)
=== RUN   TestHealthHandler
--- PASS: TestHealthHandler (0.00s)
=== RUN   TestNewHealthHandlerWithDBAndStorage
--- PASS: TestNewHealthHandlerWithDBAndStorage (0.00s)
PASS
ok  	pixora/backend/internal/api	0.18s
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
ok  	pixora/backend/internal/database	0.011s
=== RUN   TestExtractMetadata
--- PASS: TestExtractMetadata (0.01s)
=== RUN   TestIsSupportedExtension
--- PASS: TestIsSupportedExtension (0.00s)
=== RUN   TestOrientationTransform
--- PASS: TestOrientationTransform (0.00s)
=== RUN   TestExtractExifDetailsSafe
--- PASS: TestExtractExifDetailsSafe (0.00s)
=== RUN   TestThumbnailService
--- PASS: TestThumbnailService (0.08s)
=== RUN   TestThumbnailServiceReadOnly
--- PASS: TestThumbnailServiceReadOnly (0.00s)
PASS
ok  	pixora/backend/internal/media	0.096s
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
ok  	pixora/backend/internal/repository	0.015s
=== RUN   TestScannerDiscoveryAndRepeatedScan
--- PASS: TestScannerDiscoveryAndRepeatedScan (0.10s)
=== RUN   TestScannerSymlinkEscapePrevention
--- PASS: TestScannerSymlinkEscapePrevention (0.01s)
=== RUN   TestScannerConcurrentScanProtection
--- PASS: TestScannerConcurrentScanProtection (0.00s)
=== RUN   TestScannerMissingStorageRoot
--- PASS: TestScannerMissingStorageRoot (0.00s)
=== RUN   TestScannerScanSpecificFolder
--- PASS: TestScannerScanSpecificFolder (0.02s)
=== RUN   TestScannerForceRefresh
--- PASS: TestScannerForceRefresh (0.01s)
PASS
ok  	pixora/backend/internal/scanner	0.141s
=== RUN   TestFrontendBackendUnifiedServing
--- PASS: TestFrontendBackendUnifiedServing (0.00s)
=== RUN   TestPhase3FullServerLifecycle
--- PASS: TestPhase3FullServerLifecycle (0.03s)
=== RUN   TestPhase4FullServerLifecycle
--- PASS: TestPhase4FullServerLifecycle (0.01s)
=== RUN   TestPhysicalFolderBrowsingFullLifecycle
--- PASS: TestPhysicalFolderBrowsingFullLifecycle (0.04s)
=== RUN   TestPhase5TimelineAndViewerLifecycle
--- PASS: TestPhase5TimelineAndViewerLifecycle (0.04s)
=== RUN   TestPhase6SearchFilterSortLifecycle
--- PASS: TestPhase6SearchFilterSortLifecycle (0.03s)
=== RUN   TestServerHealthRoute
--- PASS: TestServerHealthRoute (0.00s)
=== RUN   TestServerWithDependencies
--- PASS: TestServerWithDependencies (0.00s)
=== RUN   TestServerUnknownAPIRouteReturns404JSON
--- PASS: TestServerUnknownAPIRouteReturns404JSON (0.00s)
=== RUN   TestServerStaticAndSPAFallback
--- PASS: TestServerStaticAndSPAFallback (0.00s)
PASS
ok  	pixora/backend/internal/server	0.164s
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

All 55 Go tests passed (0 failures).
```

### Frontend Build & Lint Checks (`frontend`)
```
> tsc --noEmit (Passed cleanly, 0 errors)
> vite build (Built dist/index.html, dist/assets in 1.21s)
```

## 6. Manual Verification Steps for User

```bash
# 1. Run all backend tests
cd backend && go test -v -count=1 ./...

# 2. Run frontend typecheck and build
cd ../frontend && npm run typecheck && npm run build

# 3. Test running server locally
cd ../backend
STORAGE_ROOT=./storage/media DATABASE_PATH=./storage/data/pixora.db go run ./cmd/server

# 4. Search photos by filename query
curl -s "http://localhost:8080/api/photos?q=beach" | jq .

# 5. Filter photos by date range
curl -s "http://localhost:8080/api/photos?startDate=2026-01-01&endDate=2026-12-31" | jq .

# 6. Filter photos by physical folder
curl -s "http://localhost:8080/api/photos?folder=Pictures/Anime&recursive=true" | jq .

# 7. Filter photos by virtual album
curl -s "http://localhost:8080/api/photos?albumId=<album-id>" | jq .

# 8. Sort photos by file size descending
curl -s "http://localhost:8080/api/photos?sort=size&order=desc" | jq .

# 9. Verify date validation error (startDate > endDate returns 400)
curl -s "http://localhost:8080/api/photos?startDate=2026-12-31&endDate=2026-01-01" | jq .

# 10. Open web interface at http://localhost:8080
# - In Photos page, use the search bar to search across indexed filenames and paths.
# - Click the Sliders button to open the filter drawer.
# - Test combining Date Range, Physical Folder, Virtual Album, and Media Type filters.
# - Verify active filter chips appear and can be cleared individually or all at once.
# - Change sorting between Capture Date, Filename, Size, and Modification Date.
# - Note that URL search parameters dynamically update and preserve discovery state on refresh or viewer close.
```
