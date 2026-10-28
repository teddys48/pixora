# Pixora — Project Status

## 1. Project Overview

**Project:** Pixora  
**Description:** A web-based self-hosted photo gallery for uploading photos, organizing physical folders and virtual albums, and browsing photos chronologically.

**Current overall status:** Phase 5 — Timeline & Photo Viewer  
**Current phase:** Phase 5 Completed (Transitioning to Phase 6 — Hardening & Deployment)

### Product Goals

- Provide a fast, responsive, and accessible photo gallery UI.
- Support uploading one or multiple photos safely.
- Discover and index existing photos in mounted host storage (`/media`).
- Support read-only physical folder and nested subfolder browsing directly from `/media`.
- Detect unindexed media files on disk with targeted one-click scanning.
- Organize photos into virtual collections and albums without moving original files.
- Browse photos chronologically using capture dates when available.
- View full-resolution photos in a responsive lightbox with EXIF metadata.
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
| Frontend | React 18 + TypeScript + Vite | Implemented | Strict TypeScript, component modularity, centralized PhotosContext, AlbumDetailView, PhysicalFolderDetailView, TimelinePage |
| UI styling | Tailwind CSS + semantic tokens | Implemented | Light/dark CSS tokens, custom scrollbars, aspect-ratio preservation |
| Icons | Lucide React | Implemented | Standard SVG icons across shell, cards, modals & actions |
| Routing | React Router v6 | Implemented | Client-side routing with Go SPA fallback (`/`, `/photos`, `/timeline`, `/folders`, `/folders/:albumId`) |
| Backend | Go 1.22+ + Fiber v2 | Implemented | Package separation (config, storage, database, repository, api, server, media, scanner) |
| Database | SQLite via `modernc.org/sqlite` | Implemented | Pure Go (CGO_ENABLED=0), WAL mode, foreign keys, versioned migrations (Migration 1 & 2) |
| Image Processing | Pure-Go decoders & bilinear scaling | Implemented | `image/jpeg`, `image/png`, `image/gif`, `golang.org/x/image/webp`, `golang.org/x/image/tiff`, `golang.org/x/image/draw` |
| EXIF Parser | `github.com/rwcarlsen/goexif` | Implemented | Pure Go, extracts `DateTimeOriginal`, timestamps, orientation tags (1-8), and camera/lens/exposure details |
| Photo storage | Local host bind mount (`/media`) | Implemented | Target `STORAGE_ROOT=/media`; safe path traversal checks; read-write mount preserved; read-only folder browsing |
| Database storage | Separate persistent volume (`/data`) | Implemented | Target `DATABASE_PATH=/data/pixora.db`; parent dir auto-created; never stored in photo dir |
| Path convention | Relative storage paths in database | Implemented | Relative paths (e.g. `originals/...`, `Pictures/vacation/beach.jpg`); validated with `internal/storage` |
| Container access | Non-root `pixora` user (UID 1000) | Implemented | Host permissions documented; no write checks at startup |
| API style | REST + JSON | Implemented | `/api/health`, `/api/photos`, `/api/timeline`, `/api/albums`, `/api/folders`, `/api/photos/scan`; structured error envelopes |
| Deployment | Unified Multi-Stage Dockerfile | Implemented | Builds FE & BE into a single non-root Alpine container (`CGO_ENABLED=0`) |
| Theme | Light, Dark, and System | Implemented | Persisted in `localStorage`, media query sync |

## 3. Development Phases

### Phase 1 — Foundation & UI

**Status:** Completed

Scope & Acceptance Criteria Met:
- [x] Initialized frontend with React, TypeScript, Vite, Tailwind CSS, and Lucide icons.
- [x] Initialized Go/Fiber backend module (`pixora/backend`) with clean package structure.
- [x] Responsive application shell with desktop sidebar and mobile navigation drawer.
- [x] Implemented core pages: Photos gallery, Timeline chronological view, and Folders album view.
- [x] Established design tokens and verified light/dark/system theme switching.
- [x] Created reusable `PhotoCard`, `FolderCard`, `GalleryControls`, `LightboxModal`, and `FallbackImage` components.
- [x] Dedicated mock data modules cleanly decoupled from UI components.
- [x] Lightbox viewer with keyboard navigation (`ArrowLeft`, `ArrowRight`, `Escape`, `I` for metadata) and body scroll lock.
- [x] Frontend/backend health integration with `HealthBadge` in sidebar.
- [x] Persistent host-bound photo storage designed: `STORAGE_ROOT` (`/media`), `DATABASE_PATH` (`/data/pixora.db`).
- [x] Multi-stage `Dockerfile` producing a single non-root Alpine container serving static assets and API.
- [x] Automated Go unit and integration tests passing; frontend TypeScript check and production build passing.
- [x] Basic project documentation (`README.md`, `.env.example`, `.dockerignore`, `docker-compose.yml`).

### Phase 2 — Backend Foundation & Data Model

**Status:** Completed

Scope & Acceptance Criteria Met:
- [x] Centralized configuration with environment variable support (`PORT`, `HOST`, `APP_ENV`, `STATIC_DIR`, `STORAGE_ROOT`, `DATABASE_PATH`) and rigorous validation (`Validate()`).
- [x] SQLite database integration using pure-Go `modernc.org/sqlite` (`CGO_ENABLED=0` compatible).
- [x] Enabled foreign keys (`PRAGMA foreign_keys = ON;`), WAL mode, 5000ms busy timeout, and connection pooling.
- [x] Versioned migration framework (`schema_migrations` table) executed sequentially inside atomic transactions.
- [x] Designed and migrated schema:
  - `photos` table with stable UUID, relative paths, original filename, media type, size, dimensions, timestamps, and indexes.
  - `albums` table with stable UUID, name, description, timestamps, and index.
  - `photo_albums` join table with foreign keys, cascade deletion, and uniqueness constraints.
- [x] Dedicated filesystem storage foundation (`internal/storage`):
  - Resolves `STORAGE_ROOT` safely.
  - Path traversal rejection (`../`, absolute paths, null bytes).
  - Evaluates and rejects symlinks pointing outside storage root.
  - Distinguishes read access from write access.
- [x] Implemented domain models and repository layer (`PhotoRepository`, `AlbumRepository`).
- [x] Established API foundation with consistent JSON error envelopes (`{"error": "...", "message": "..."}`).
- [x] Enhanced `/api/health` with SQLite database connectivity ping and storage status reporting.
- [x] Automated unit and integration tests passing across all packages using isolated temporary databases and storage roots.

### Phase 3 — Photo Management, Media Scanning & Upload

**Status:** Completed

Scope & Acceptance Criteria Met:
- [x] **Existing Media Discovery and Indexing (`internal/scanner`):**
  - Discovers existing photos in `STORAGE_ROOT` recursively across subdirectories.
  - Supports JPEG, PNG, WebP, GIF, and TIFF image formats.
  - Excludes generated directories (`thumbnails/`, `tmp/`) and hidden dotfiles/dotdirs (`.git`, `.hidden`).
  - Evaluates symlinks and rejects any symlinks escaping `STORAGE_ROOT`.\n  - Non-destructive: never modifies, renames, moves, or deletes original host files.
  - Deduplication: uses `relative_path UNIQUE` index; unchanged files are skipped instantly; modified files have their metadata safely updated without duplicate records.
  - Concurrency lock: prevents multiple simultaneous scans from colliding or corrupting database records.
  - Documented endpoints: `POST /api/photos/scan` (supports sync and async modes) and `GET /api/photos/scan` (progress & status reporting).
- [x] **Photo Listing API (`GET /api/photos`):**
  - Reads indexed records directly from SQLite with deterministic sorting and pagination (`limit`, `offset`).
  - Returns `url` (`/api/photos/:id/file`) and `thumbnailUrl` (`/api/photos/:id/thumbnail`).
- [x] **Photo Upload Pipeline (`POST /api/photos`, `POST /api/photos/upload`):**
  - Multipart form upload supporting single or batch files (up to 20 files per request).
  - Server-side magic bytes validation; streaming uploads to bounded temporary files; 25MB per-file upload limit.
  - Decompression bomb safety limits (max 15,000px dimension, max 100 megapixels).
  - Server-assigned UUID filenames under `originals/<uuid>.<ext>` to avoid collisions; original filename stored in metadata.
- [x] **Secure Photo & Thumbnail Serving:**
  - `GET /api/photos/:id/file`: serves original image with `ETag`, `Cache-Control`, and `304 Not Modified` support.
  - `GET /api/photos/:id/thumbnail`: serves high-performance JPEG thumbnail from `thumbnails/<id>.jpg` with EXIF orientation correction.
- [x] **Frontend Integration:**
  - Replaced mock photo data with real API data from `GET /api/photos`.
  - "Scan Media" button and interactive scan progress polling in `GalleryControls` and `EmptyState`.
  - Real file upload modal with drag & drop, file selection preview, client-side validation, progress bar, error list, and auto-refresh.

### Phase 4 — Virtual Albums & Physical Folder Browsing

**Status:** Completed

Scope & Acceptance Criteria Met:

#### Part A: Virtual Albums (Database-backed Collections)
- [x] **Database Schema & Migrations:**
  - Migration 2: Case-insensitive uniqueness for album names (`CREATE UNIQUE INDEX idx_albums_name_nocase ON albums(name COLLATE NOCASE);`).
  - Foreign keys on `photo_albums` with `ON DELETE CASCADE`.
- [x] **Repository Layer (`internal/repository/album.go`):**
  - CRUD operations: `Create`, `GetByID`, `List`, `Update`, `Delete`.
  - Association methods: `AddPhotos`, `RemovePhoto`, `GetPhotos`.
  - Dynamically computes `photo_count` and latest photo cover thumbnail.
- [x] **REST API Endpoints:**
  - `GET /api/albums` — Lists virtual albums with photo counts and cover thumbnails.
  - `POST /api/albums` — Creates a virtual album with validation.
  - `GET /api/albums/:id` — Retrieves album details (`404` if nonexistent).
  - `PATCH /api/albums/:id` — Renames or updates an album.
  - `DELETE /api/albums/:id` — Safely deletes an album record without deleting library photos or filesystem files.
  - `GET /api/albums/:id/photos` — Paginated list of photos in an album.
  - `POST /api/albums/:id/photos` — Adds photo IDs to an album in an atomic transaction.
  - `DELETE /api/albums/:id/photos/:photoId` — Removes a photo association.
- [x] **Virtual Album UI & Workflow:**
  - `AlbumDetailView`: breadcrumbs, search, grid/list view, "Remove from album", LightboxModal integration, and pagination.
  - `CreateFolderModal`, `RenameFolderModal`, `DeleteFolderModal`.
  - `AddPhotosToAlbumModal`: multi-select photos with "In Album" indicators to prevent duplicate assignment.

#### Part B: Physical Folder Browsing (Filesystem-derived under `/media`)
- [x] **Storage & Path Security (`internal/storage/storage.go`):**
  - Added `ValidateFolderPath` and `ResolveFolderPath` to validate paths relative to `STORAGE_ROOT`.
  - Rejects traversal attempts (`..`, `../`), null bytes (`%00`), and absolute host paths.
  - Evaluates symlinks and rejects any target outside `STORAGE_ROOT`.
  - Never exposes absolute host paths in API responses; paths are normalized relative to `/media`.
- [x] **Dynamic Repository Queries without a Folder DB Table (`internal/repository/photo.go`):**
  - Implemented `ListByFolder(ctx, folderPath, offset, limit, sort, order)` to return photos directly in a folder using pure SQLite string operations (`SUBSTR`, `INSTR`).
  - Implemented `CountByFolder(ctx, folderPath)` and `GetFolderCoverPhoto(ctx, folderPath)`.
  - Queries remain completely isolated: subfolder photos do not leak into parent folder queries.
- [x] **Targeted Media Scanning (`internal/scanner/scanner.go`):**
  - Added `ScanPath(ctx, targetRelPath)` to allow scanning a specific subfolder without running an expensive full-library traversal.
  - Skips unchanged files and updates modified files without duplicate database records.
- [x] **Physical Folder REST API (`internal/api/folders.go`):**
  - `GET /api/folders` — Lists immediate subfolders under a given path (defaults to root `/media`), reporting folder name, relative path, indexed photo count, subfolder count, cover thumbnail URL, and `hasUnindexedFiles` flag.
  - `GET /api/folders/contents` — Retrieves folder details, breadcrumbs hierarchy (`Home / Pictures / Anime`), immediate subfolders, paginated indexed photos, and disk vs database unindexed count (`unindexedCount = diskMediaCount - totalPhotos`).
  - `GET /api/folders/photos` — Paginated listing of photos directly residing in the specified physical folder.
  - `POST /api/photos/scan?path=...` — Triggers a targeted scan on the specified folder path.
- [x] **Frontend Folders Tab UI (`PhysicalFolderCard`, `PhysicalFolderDetailView`, `FoldersPage`):**
  - **Concept Separation:** Distinct segmented control at the top of the Folders tab:
    - **Physical Folders (`/media`):** Real filesystem directories discovered from disk; read-only; no modification, file renaming, or moving.
    - **Virtual Albums:** SQLite-backed custom collections that can group photos from multiple physical locations.
  - **Nested Navigation & Breadcrumbs:** Full breadcrumb trail (`Home / ... / Folder`) allowing users to click any parent directory or click the "Up" button.
  - **Deep Linking:** Synchronized with URL query parameters (`?tab=physical&path=Pictures/Anime`), enabling browser back/forward and direct bookmarking.
  - **Unindexed Media Detection:** Detects if files exist on disk that have not yet been indexed into Pixora; shows an alert banner with a one-click "Scan Now" button that runs `POST /api/photos/scan?path=...` and refreshes the folder.
  - **Photo Grid & Viewer:** Renders direct photos with grid/list mode toggle, search filtering, "Load More Photos" pagination, and Lightbox modal integration.
  - **Non-Destructive Guarantee:** Browsing does not create, rename, move, or delete files or directories under `/media`.

### Phase 5 — Timeline & Photo Viewer

**Status:** Completed

Scope & Acceptance Criteria Met:
- [x] **Chronological Timeline API (`GET /api/timeline`):**
  - Reverse-chronological ordering: photos with valid capture dates (`taken_at`) first, followed by photos with file modification times (`file_modified_at`), deterministic tie-breaking by photo ID.
  - Returns paginated photos, aggregated day groups (`TimelineGroupResponse` with formatted date, photo count, and photos list), total photo count, and cursor-based/offset pagination (`hasMore`, `nextCursor`).
- [x] **EXIF Capture Date Precedence & Extraction (`internal/media`):**
  - Strict precedence: EXIF `DateTimeOriginal` -> `DateTimeDigitized` -> `DateTime` -> `nil` (undated fallback).
  - Never fabricates capture dates from file modification timestamps; files lacking EXIF capture timestamps are explicitly flagged as `isUndated: true`.
  - On-demand safe EXIF metadata extraction: Camera (Make + Model), Lens Model, Focal Length, Aperture (F-number), Exposure Time (Shutter speed), and ISO rating without full image decompression.
  - Safe against corrupted, malformed, or missing EXIF tags.
- [x] **Orientation & Bilinear Thumbnail Rendering:**
  - Preserved EXIF orientation tags (1–8) in thumbnail generation with high-performance bilinear scaling (`golang.org/x/image/draw`).
  - Served through `/api/photos/:id/thumbnail` and original files via `/api/photos/:id/file` with caching headers (`ETag`, `Cache-Control`).
- [x] **Adjacent Photo Navigation (`internal/repository/photo.go`, `GET /api/photos/:id`):**
  - SQLite window functions (`LAG` and `LEAD`) across timeline, physical folder, and virtual album collections for deterministic previous/next photo traversal in a single query.
- [x] **Frontend Chronological Timeline Page (`TimelinePage.tsx`):**
  - Groups real indexed photos chronologically by Day (`Saturday, October 24, 2026`) under sticky Month & Year headers (`October 2026`).
  - Dedicated "Undated Photos" section at the bottom for files lacking EXIF capture timestamps, clearly explaining they are ordered by file modification date.
  - Incremental pagination ("Load More Moments") preserving loaded photos and user scroll position.
  - Responsive thumbnail grid using `PhotoCard` with aspect-ratio preservation and badges.
  - Clicking any photo opens the dedicated Photo Viewer in the active timeline collection context.
- [x] **Enhanced Photo Viewer (`LightboxModal.tsx`):**
  - Displays original full-resolution photo in container (`object-contain`) without unwanted cropping or distortion.
  - Smooth zoom controls: 100% to 400%, Zoom In (+), Zoom Out (-), Reset Zoom (0), Fit-to-Width, and Fit-to-Screen.
  - Panning support: mouse drag and touch drag when zoomed (`zoomScale > 1`) with responsive cursor states (`grab` / `grabbing`).
  - Loading indicator and graceful error handling (`ImageOff` fallback).
  - Keyboard navigation: `ArrowLeft`, `ArrowRight`, `Escape`, `I` (toggle metadata), `+`/`=`, `-`/`_`, `0` (reset zoom), strictly guarded against active input/textarea typing.
  - Comprehensive metadata sidebar (desktop) and bottom sheet (mobile): displays filename, capture date (with EXIF badge) vs fallback modified date, relative physical folder (never exposing server absolute paths), image dimensions, file size, and camera/lens/exposure EXIF details when present.
  - Context-preserving previous and next navigation across Timeline, Photos, physical Folders, and virtual Albums.
- [x] **Metadata Consistency Audit & Unified Date-Source Policy:**
  - **Identified Root Cause:** Photos cards rendered `p.takenAt || p.fileModifiedAt || p.createdAt` with a `<Calendar />` icon without distinguishing filesystem modification dates from verified capture dates. Meanwhile, Timeline strictly grouped photos lacking `takenAt` into "Undated Photos". Furthermore, the 50 undated photos in the user library are PNG screenshots and WebP files that genuinely lack EXIF metadata tags.
  - **Unified 3-Tier Policy:**
    1. *Verified EXIF Capture Timestamp (`dateSource: "exif"`):* Extracted with strict tag precedence (`DateTimeOriginal` -> `DateTimeDigitized` -> `DateTime`) and timezone offset support (`OffsetTimeOriginal`, `OffsetTimeDigitized`, `OffsetTime`). Displayed with `<Calendar />` in Photos, grouped by day in Timeline, and marked `isUndated: false`.
    2. *Filesystem Modification Fallback (`dateSource: "file_modified"`):* Used when EXIF metadata is absent. Displayed on Photos cards with `<Clock />` and a distinct `mod`/`Modified` fallback badge and tooltip; placed under "Undated Photos" in Timeline; labeled "File Modified (Fallback)" in Photo Viewer.
    3. *Genuinely Undated (`dateSource: "none"`):* Kept as "Undated" with `<HelpCircle />`; timestamps are never fabricated.
  - **Safe Metadata Refresh Workflow:** Added `forceRefresh` support to `internal/scanner` (`ScanPath(ctx, relPath, true)`) and `POST /api/photos/scan?refresh=true`. Existing records can have metadata re-extracted without modifying host files or generating duplicate records.
- [x] **Automated Tests:**
  - 52 Go tests passing across all packages (`api`, `config`, `database`, `media`, `repository`, `scanner`, `server`, `storage`), including `TestScannerForceRefresh`, `TestTimelineAndPhotoViewerAPI`, `TestExtractExifDetailsSafe`, `TestPhotoRepositoryTimelineAndAdjacent`, and `TestPhase5TimelineAndViewerLifecycle`.
  - Frontend TypeScript verification (`npm run typecheck`) and production build (`npm run build`) passing cleanly with 0 errors.

### Phase 6 — Hardening & Deployment

**Status:** Planned (Next Phase)

Scope:
- Production performance review and security audit.
- Persistent volume mounting documentation.
- Backup and restore procedures for database and photo storage.

## 4. Current Implementation Inventory

| Area | Status | Verified Notes |
|---|---|---|
| Frontend scaffold | Implemented | React 18, Vite, TypeScript strict, Tailwind CSS, Lucide |
| Backend scaffold | Implemented | Go 1.22+, Fiber v2, graceful shutdown, request logging, CORS |
| Storage abstraction | Implemented | `internal/storage`: Safe path resolution, traversal rejection, symlink checks, folder validation |
| Media scanner | Implemented | `internal/scanner`: In-place discovery, recursive walk, format detection, deduplication, full & targeted scan |
| Metadata & thumbnails | Implemented | `internal/media`: EXIF `DateTimeOriginal` precedence, orientation transforms (1-8), bilinear thumbnailing, EXIF camera/lens/exposure details |
| Database integration | Implemented | `internal/database`: Pure-Go SQLite, WAL mode, foreign keys, versioned migrations (v1, v2) |
| Repository layer | Implemented | `internal/repository`: PhotoRepository with folder queries, timeline ordering, and window-function adjacent photos (`LAG`/`LEAD`) |
| API Endpoints | Implemented | `/api/health`, `/api/photos`, `/api/photos/:id`, `/api/photos/:id/file`, `/api/photos/:id/thumbnail`, `/api/photos/scan`, `/api/photos/upload`, `/api/timeline`, `/api/albums`, `/api/albums/:id`, `/api/albums/:id/photos`, `/api/albums/:id/photos/:photoId`, `/api/folders`, `/api/folders/contents`, `/api/folders/photos` |
| Photo gallery UI | Implemented | Centralized `PhotosContext`, real API data, grid/list view, sorting, search filter, empty/loading/error states, Load More pagination |
| Timeline UI | Implemented | Real API data from `/api/timeline`, grouped by Day under sticky Month/Year headers, dedicated Undated section, incremental loading |
| Photo viewer (Lightbox) | Implemented | Full zoom (100%-400%), mouse/touch panning, fit-to-width, fit-to-screen, EXIF details drawer, keyboard navigation, context-aware adjacent photos |
| Photo upload UI | Implemented | Drag & drop, multi-file selection, progress tracking, per-file error handling, auto-refresh |
| Physical Folders UI | Implemented | Read-only filesystem browsing, nested subfolder navigation, breadcrumbs, unindexed photo alerts, targeted scan button |
| Virtual Albums UI | Implemented | Real SQLite albums from `/api/albums`, persistent creation, renaming, safe deletion, album detail view, photo assignment, remove photo |
| Theme switching | Implemented | Light, dark, and system preference with localStorage persistence |
| Automated tests | Verified | 52 Go tests passing across all packages; frontend typecheck and build passing cleanly (0 errors) |
| Unified Dockerfile | Implemented | Multi-Stage build (`CGO_ENABLED=0` statically compiled, non-root UID 1000) |

## 5. Architecture & Concepts: Physical Folders vs Virtual Albums

```
                                  +-----------------------------+
                                  |       STORAGE_ROOT          |
                                  |          /media             |
                                  +--------------+--------------+
                                                 |
                       +-------------------------+-------------------------+
                       |                                                   |
             Physical Directories                                Physical Image Files
        /media/Pictures/Anime/2024/...                      /media/Pictures/vacation.jpg
                       |                                                   |
            [Read-Only Filesystem Walk]                         [Media Scanner Indexer]
                       |                                                   |
                       v                                                   v
           Physical Folders Navigation                          SQLite `photos` Table
           - GET /api/folders                                   - id (UUID)
           - GET /api/folders/contents                          - relative_path (UNIQUE)
           - GET /api/folders/photos                            - title, width, height, etc.
                       |                                        - taken_at (EXIF timestamp)
                       |                                        - file_modified_at
                       +-------------------------+-------------------------+
                                                 |
                                                 v
                                    SQLite `photo_albums` Table
                                                 ^
                                                 |
                                     SQLite `albums` Table
                                     - id (UUID)
                                     - name (UNIQUE NOCASE)
                                     - description
                                                 |
                                                 v
                                        Virtual Albums View
                                        - Can combine photos from
                                          any physical folder
                                        - Mutable CRUD (create/rename/delete)
                                        - Never modifies original files on disk
```

### Storage Safety & Thumbnail Storage
- **Read-Write Mount Configuration:** The application container runs with read-write access to `/media` (`STORAGE_ROOT`). This mount configuration is preserved.
- **Strict Read-Only Browsing:** The physical folder browsing feature performs read-only filesystem reads (`os.ReadDir`). It never creates, renames, moves, or deletes directories or files inside `/media`.
- **Thumbnail Storage:** Application-generated thumbnails are stored in the designated application-managed cache directory (`thumbnails/<id>.jpg` within `STORAGE_ROOT`). Pixora never writes thumbnails or metadata files alongside original user photos in user directories.

## 6. Verification Results

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
PASS
ok  	pixora/backend/internal/repository	0.010s
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
ok  	pixora/backend/internal/scanner	0.124s
=== RUN   TestFrontendBackendUnifiedServing
--- PASS: TestFrontendBackendUnifiedServing (0.00s)
=== RUN   TestPhase3FullServerLifecycle
--- PASS: TestPhase3FullServerLifecycle (0.03s)
=== RUN   TestPhase4FullServerLifecycle
--- PASS: TestPhase4FullServerLifecycle (0.01s)
=== RUN   TestPhysicalFolderBrowsingFullLifecycle
--- PASS: TestPhysicalFolderBrowsingFullLifecycle (0.06s)
=== RUN   TestPhase5TimelineAndViewerLifecycle
--- PASS: TestPhase5TimelineAndViewerLifecycle (0.04s)
=== RUN   TestServerHealthRoute
--- PASS: TestServerHealthRoute (0.00s)
=== RUN   TestServerWithDependencies
--- PASS: TestServerWithDependencies (0.00s)
=== RUN   TestServerUnknownAPIRouteReturns404JSON
--- PASS: TestServerUnknownAPIRouteReturns404JSON (0.00s)
=== RUN   TestServerStaticAndSPAFallback
--- PASS: TestServerStaticAndSPAFallback (0.00s)
PASS
ok  	pixora/backend/internal/server	0.143s
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
ok  	pixora/backend/internal/storage	0.005s

All 52 Go tests passed (0 failures).
```

### Frontend Build & Lint Checks (`frontend`)
```
> tsc --noEmit (Passed cleanly, 0 errors)
> vite build (Built dist/index.html, dist/assets in 1.13s)
```

## 7. Manual Verification Steps for User

```bash
# 1. Run all backend tests
cd backend && go test -v -count=1 ./...

# 2. Run frontend typecheck and build
cd ../frontend && npm run typecheck && npm run build

# 3. Test running server locally
cd ../backend
STORAGE_ROOT=./storage/media DATABASE_PATH=./storage/data/pixora.db go run ./cmd/server

# 4. Fetch chronological timeline photos grouped by Day via API
curl -s "http://localhost:8080/api/timeline?limit=20" | jq .

# 5. Fetch a single photo's detailed metadata with adjacent collection navigation
curl -s "http://localhost:8080/api/photos/<photo-id>?collectionType=timeline" | jq .

# 6. List physical folders in /media via API
curl -s "http://localhost:8080/api/folders" | jq .

# 7. Get contents of a specific physical folder with breadcrumbs and photos
curl -s "http://localhost:8080/api/folders/contents?path=Pictures/Anime" | jq .

# 8. Open the web interface in a browser
# Navigate to http://localhost:8080
# Open Timeline tab:
# - Inspect photos grouped by Day under sticky Month/Year headers.
# - Verify Undated Photos section at the bottom for files lacking EXIF capture timestamps.
# - Click any photo to open the enhanced Photo Viewer.
# - In Photo Viewer: test Zoom In/Out (+/-), Fit to Width, Fit to Screen, Reset Zoom (0), pan by dragging, and press 'I' to inspect EXIF metadata.
# - Navigate previous/next using Arrow keys or on-screen chevron buttons.
```
