# Pixora — Project Status

## 1. Project Overview

**Project:** Pixora  
**Description:** A simple web-based photo gallery for uploading photos, managing folders, and browsing photos by timeline.

**Current overall status:** Phase 3 Bug Audit & UI Fixes Complete — Ready for Phase 4  
**Current phase:** Phase 3 — Photo Management, Media Scanning & Upload (Audited & Refined)

### Product Goals

- Provide a simple and responsive photo gallery.
- Support uploading one or multiple photos.
- Discover and index existing photos in mounted host storage.
- Organize photos into folders and virtual albums.
- Browse photos chronologically using capture dates when available.
- View full-resolution photos in a lightbox.
- Store photos and metadata reliably on local storage with host bind mounts.
- Keep deployment and maintenance simple.

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
| Frontend | React 18 + TypeScript + Vite | Implemented | Strict TypeScript, component modularity, centralized PhotosContext |
| UI styling | Tailwind CSS + semantic tokens | Implemented | Light/dark CSS tokens, custom scrollbars, aspect-ratio preservation |
| Icons | Lucide React | Implemented | Standard SVG icons across shell & actions |
| Routing | React Router v6 | Implemented | Client-side routing with Go SPA fallback |
| Backend | Go 1.22+ + Fiber v2 | Implemented | Package separation (config, storage, database, repository, api, server, media, scanner) |
| Database | SQLite via `modernc.org/sqlite` | Implemented | Pure Go (CGO_ENABLED=0), WAL mode, foreign keys, versioned migrations |
| Image Processing | Pure-Go decoders & bilinear scaling | Implemented | `image/jpeg`, `image/png`, `image/gif`, `golang.org/x/image/webp`, `golang.org/x/image/tiff`, `golang.org/x/image/draw` |
| EXIF Parser | `github.com/rwcarlsen/goexif` | Implemented | Pure Go, extracts `DateTimeOriginal`, timestamps, and orientation tags (1-8); orientation transforms applied to thumbnails |
| Photo storage | Local host bind mount (`/media`) | Implemented | Target `STORAGE_ROOT=/media`; safe path traversal checks; read-only mount tolerated |
| Database storage | Separate persistent volume (`/data`) | Implemented | Target `DATABASE_PATH=/data/pixora.db`; parent dir auto-created; never stored in photo dir |
| Path convention | Relative storage paths in database | Implemented | Relative paths (e.g. `originals/...`, `vacation/beach.jpg`); validated with `internal/storage` |
| Container access | Non-root `pixora` user (UID 1000) | Implemented | Host permissions documented; no write checks at startup |
| API style | REST + JSON | Implemented | `/api/health`, `/api/photos`, `/api/albums`, `/api/photos/scan`; structured error envelopes |
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
- [x] Dedicated mock data modules (`photos.ts`, `folders.ts`) cleanly decoupled from UI components.
- [x] Lightbox viewer with keyboard navigation (`ArrowLeft`, `ArrowRight`, `Escape`, `I` for metadata) and body scroll lock.
- [x] Frontend/backend health integration with `HealthBadge` in sidebar.
- [x] Persistent host-bound photo storage designed: `STORAGE_ROOT` (`/media`), `DATABASE_PATH` (`/data/pixora.db`), separation of photos and DB.
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
  - Read-only bind mount support: server starts cleanly without write access to `/media`; no test files written at startup.
- [x] Implemented domain models (`models.Photo`, `models.Album`, `models.PhotoAlbum`) and repository layer (`PhotoRepository`, `AlbumRepository`).
- [x] Established API foundation with consistent JSON error envelopes (`{"error": "...", "message": "..."}`), input validation, and HTTP status codes.
- [x] Enhanced `/api/health` with SQLite database connectivity ping and storage status reporting.
- [x] Minimal schema verification endpoints: `GET /api/photos`, `GET /api/photos/:id`, `GET /api/albums`, `POST /api/albums`, `GET /api/albums/:id`.
- [x] Automated unit and integration tests passing across all packages using isolated temporary databases and storage roots.

### Phase 3 — Photo Management, Media Scanning & Upload

**Status:** Completed

Scope & Acceptance Criteria Met:
- [x] **Existing Media Discovery and Indexing (`internal/scanner`):**
  - Discovers existing photos in `STORAGE_ROOT` recursively across subdirectories.
  - Never hardcodes host-specific paths.
  - Supports JPEG, PNG, WebP, GIF, and TIFF image formats.
  - Excludes generated directories (`thumbnails/`, `tmp/`) and hidden dotfiles/dotdirs (`.git`, `.hidden`).
  - Evaluates symlinks and rejects any symlinks escaping `STORAGE_ROOT`.
  - Non-destructive: never modifies, renames, moves, or deletes original host files.
  - Indexes relative paths under `STORAGE_ROOT` using stable UUID identifiers.
  - Deduplication: uses `relative_path UNIQUE` index; unchanged files are skipped instantly; modified files have their metadata safely updated without duplicate records.
  - Gracefully handles corrupt or incomplete files without failing the entire scan.
  - Concurrency lock: prevents multiple simultaneous scans from colliding or corrupting database records.
  - Documented endpoints: `POST /api/photos/scan` (supports sync and async modes) and `GET /api/photos/scan` (progress & status reporting).
- [x] **Photo Listing API (`GET /api/photos`):**
  - Reads indexed records directly from SQLite.
  - Deterministic sorting by `date` (preferring `taken_at`), `modified`, `created`, `filename`, `size`; with `asc`/`desc` ordering.
  - Safe pagination via `limit` (default 50, max 100) and `offset` (default 0).
  - Returns `url` (`/api/photos/:id/file`) and `thumbnailUrl` (`/api/photos/:id/thumbnail`).
  - Returns empty collection `{ "photos": [], "total": 0 }` when no photos are indexed.
- [x] **Photo Upload Pipeline (`POST /api/photos`, `POST /api/photos/upload`):**
  - Multipart form upload supporting single or batch files (up to 20 files per request).
  - Server-side magic bytes validation (JPEG, PNG, WebP, GIF, TIFF); rejects untrusted extensions and fake MIME headers.
  - Streaming uploads to bounded temporary files (`tmp/upload_<uuid>.tmp`) to prevent memory exhaustion.
  - 25MB per-file upload limit enforced.
  - Decompression bomb safety limits (max 15,000px dimension, max 100 megapixels).
  - Server-assigned UUID filenames under `originals/<uuid>.<ext>` to avoid collisions; original filename stored in metadata.
  - Atomic rename/move to destination directory; temporary file and database rollback on failure.
  - Read-only storage enforcement: returns `403 Forbidden` (`storage_readonly`) if destination is read-only.
  - Returns created photo records and handles partial failures (`207 Multi-Status`).
- [x] **Secure Photo & Thumbnail Serving:**
  - `GET /api/photos/:id/file`: resolves photo by DB ID, verifies path remains inside `STORAGE_ROOT`, serves original image with `ETag`, `Cache-Control`, and `304 Not Modified` support.
  - `GET /api/photos/:id/thumbnail`: serves high-performance JPEG thumbnail from `thumbnails/<id>.jpg`.
  - Bilinear downscaling preserving original aspect ratio within a 400x400 bounding box.
  - EXIF orientation correction (handles orientations 1 through 8 so smartphone photos are not rotated).
  - Caches thumbnails to disk to avoid repeated decoding; falls back cleanly to original photo if storage is read-only.
- [x] **Frontend Integration:**
  - Replaced mock photo data with real API data from `GET /api/photos`.
  - Added "Scan Media" button and interactive scan progress polling in `GalleryControls` and `EmptyState`.
  - Real file upload modal with drag & drop, file selection preview, client-side validation, progress bar, error list, and auto-refresh.
  - Loading skeleton state and error state with retry button.
  - Incremental pagination ("Load More Photos").
  - LightboxModal connected to real photo URLs and metadata.
- [x] **Automated Testing:**
  - 27 Go tests passing across `api`, `config`, `database`, `media`, `repository`, `scanner`, `server`, and `storage`.
  - Frontend TypeScript verification and production build passing cleanly (0 errors).

### Phase 3 Bug Audit & UI Fixes

**Status:** Completed

Following the Phase 3 implementation, an audit was conducted on three reported issues:

1. **Issue A — Manual Media Scanning:**
   - **Audit finding:** Expected current-phase behavior for Phase 3. The absence of automatic startup scanning or background filesystem watchers is by design in Phase 3.
   - **Verification:**
     - Scan button invokes `POST /api/photos/scan` correctly.
     - Backend scans the dynamically configured `STORAGE_ROOT` (`internal/scanner`), rejecting traversal or symlink escapes.
     - Scan results accurately report indexed, skipped, updated, and failed files.
     - Scan completion automatically triggers a gallery refresh without a browser reload.
     - Concurrency locking prevents duplicate scan jobs (`409 Conflict`).
     - Safe deduplication guarantees no duplicate database entries on repeat scans.
     - Read-only storage roots are supported for scanning without write permissions.
   - **Decision:** Kept manual scanning as expected behavior for Phase 3 without adding unauthorized watchers or scheduled background jobs.

2. **Issue B — Photos Missing from Timeline and Folder Tabs:**
   - **Audit finding:** Real bug and architectural disconnect.
     - `TimelinePage.tsx` was directly importing and displaying static `MOCK_PHOTOS` from Phase 1, never requesting or rendering real photos from SQLite.
     - `FoldersPage.tsx` was using in-memory `MOCK_FOLDERS` and creating volatile client-side folders without connecting to the existing backend `/api/albums` endpoints.
     - Photo state was fragmented across components rather than having a consistent source of truth.
   - **Root Causes:**
     - Missing centralized state management for photos across tabs.
     - `TimelinePage` and `FoldersPage` were not migrated from Phase 1 mock data when Phase 3 backend APIs were introduced.
     - Photo assignment to albums is a planned Phase 4 feature, but the UI displayed mock albums with mock Unsplash covers, confusing users when scanned photos did not appear inside them.
   - **Fixes Applied:**
     - Implemented `PhotosContext` (`PhotosProvider` in `frontend/src/context/PhotosContext.tsx`) as the single source of truth for photo listing, sorting, pagination, search query, and scan state.
     - Updated `App.tsx` to provide `PhotosProvider` to all pages and removed `MOCK_PHOTOS` initialization.
     - Connected `TimelinePage.tsx` to real photo data via `usePhotos()`.
     - Filtered photos in `TimelinePage` strictly by verified EXIF capture dates (`takenAt` / `DateTimeOriginal`), explicitly refusing to invent capture dates for photos without EXIF.
     - Added clear Phase 5 Preview badges and status notices to `TimelinePage.tsx` explaining current EXIF-based grouping and future timeline features.
     - Connected `FoldersPage.tsx` to real backend endpoints (`fetchAlbums()` and `createAlbum()` in `frontend/src/lib/api.ts`), persisting albums to SQLite.
     - Added clear Phase 4 Preview badges and status notices to `FoldersPage.tsx` explaining that album photo assignment is coming in Phase 4.
     - Added friendly empty state in `PhotosPage` when an album has 0 photos.

3. **Issue C — Photos Appear Excessively Zoomed In or Incorrectly Cropped:**
   - **Audit finding:** Real bug spanning thumbnail generation and frontend CSS layout.
     - Backend `ThumbnailService.generateThumbnail` decoded images with `image.Decode`, but never called `ApplyOrientation()`. Smartphone portrait photos with orientation 6 or 8 were generated as landscape thumbnails (400x300 instead of 300x400) without EXIF orientation tags, rendering sideways in the browser.
     - `ExtractMetadata` did not swap dimensions when orientation was 5, 6, 7, or 8, resulting in incorrect width and height stored in SQLite.
     - In `FallbackImage.tsx`, the `className` prop was passed to the wrapper `<div>` while the inner `<img>` had hardcoded `object-cover`. When `LightboxModal` passed `object-contain`, it was ignored and `object-cover` forced full-bleed cropping.
     - In `PhotoCard.tsx`, portrait and panoramic photos in `aspect-[4/3]` with `object-cover` were aggressively cropped (losing up to 50% of the image height or width), making subjects appear excessively zoomed in.
   - **Fixes Applied:**
     - Updated `ThumbnailService.generateThumbnail` (`backend/internal/media/thumbnail.go`) to inspect `io.ReadSeeker` for EXIF orientation and invoke `ApplyOrientation(srcImg, orientation)` before scaling.
     - Updated `ExtractMetadata` (`backend/internal/media/metadata.go`) to swap `Width` and `Height` for orientations 5 through 8.
     - Updated `FallbackImage.tsx` to support `imgClassName` and inherit `object-contain` for proper image fitting.
     - Updated `PhotoCard.tsx` (grid and list views) to use `w-full h-full object-contain` within uniform `aspect-[4/3]` cards, preserving original aspect ratios of portrait, landscape, square, and panoramic photos without cropping or distortion.
     - Updated `LightboxModal.tsx` to apply `object-contain` to the full-size image within viewport constraints (`max-h-[82vh] max-w-[88vw]`).
     - Added unit tests in `media_test.go` verifying portrait (600x800 -> 300x400) and panoramic (1200x400 -> 400x133) thumbnail generation.

### Phase 4 — Folder Management

**Status:** Planned (Next Phase)

Scope reserved for Phase 4:
- Full persistent folder/album CRUD (update, delete).
- Assigning and moving photos to folders via API and UI.
- Setting album cover photo.
- Deletion policy for folders containing photos.

### Phase 5 — Timeline & Photo Viewer

**Status:** Planned

Scope reserved for Phase 5:
- Chronological timeline query grouped by Year and Month using real database timestamps.
- Advanced timeline date markers and infinite scroll.
- Fallback timeline date strategy for photos without EXIF capture dates.
- Enhanced photo viewer pagination and full keyboard controls over large collections.

### Phase 6 — Hardening & Deployment

**Status:** Planned

Scope:
- Production performance review and security audit.
- Persistent volume mounting documentation.
- Backup and restore procedures for database and photo storage.

## 4. Current Implementation Inventory

| Area | Status | Verified Notes |
|---|---|---|
| Frontend scaffold | Implemented | React 18, Vite, TypeScript strict, Tailwind CSS, Lucide |
| Backend scaffold | Implemented | Go 1.22+, Fiber v2, graceful shutdown, request logging |
| Storage abstraction | Implemented | `internal/storage`: Safe resolution, traversal rejection, symlink checks, read-only media mount support |
| Media scanner | Implemented | `internal/scanner`: In-place discovery, recursive walk, format detection, deduplication, scan lock, status API |
| Metadata & thumbnails | Implemented | `internal/media`: EXIF `DateTimeOriginal`, orientation transforms, bilinear thumbnailing, decompression bomb checks |
| Database integration | Implemented | `internal/database`: Pure-Go SQLite, WAL mode, foreign keys, versioned migrations |
| Repository layer | Implemented | `internal/repository`: SQLite repositories for Photos (Create, Update, Delete, Get, List, Count) and Albums |
| API Endpoints | Implemented | `/api/health`, `/api/photos`, `/api/photos/:id`, `/api/photos/:id/file`, `/api/photos/:id/thumbnail`, `/api/photos/scan`, `/api/photos/upload`, `/api/albums` |
| Photo gallery UI | Implemented | Centralized `PhotosContext`, real API data, grid/list view, sorting, search filter, empty/loading/error states, Load More pagination |
| Photo viewer (Lightbox) | Implemented | Real photo URLs and metadata, `object-contain` aspect preservation, keyboard navigation, EXIF resolution and file size display |
| Photo upload UI | Implemented | Drag & drop, multi-file selection, progress tracking, per-file error handling, auto-refresh |
| Media scan UI | Implemented | One-click media scan, progress polling, scan summary banner, instant gallery refresh |
| Timeline UI | Implemented | Real photo data, chronological grouping by EXIF `DateTimeOriginal`, Phase 5 status notices |
| Folders UI | Implemented | Real SQLite albums from `/api/albums`, persistent creation, Phase 4 status notices |
| Theme switching | Implemented | Light, dark, and system preference with localStorage persistence |
| Automated tests | Verified | 27 Go tests passing across `api`, `config`, `database`, `media`, `repository`, `scanner`, `server`, and `storage` |
| Frontend build | Verified | `npm run typecheck` and `npm run build` passing cleanly (0 errors) |
| Unified Dockerfile | Implemented | Multi-Stage build (`CGO_ENABLED=0` statically compiled, non-root UID 1000) |

## 5. Verification Results

### Go Backend Tests (`backend`)
```
?   	pixora/backend/cmd/server	[no test files]
=== RUN   TestAlbumsAPI
--- PASS: TestAlbumsAPI (0.00s)
=== RUN   TestPhotosAPI
--- PASS: TestPhotosAPI (0.03s)
=== RUN   TestUploadAPI
--- PASS: TestUploadAPI (0.01s)
=== RUN   TestScanAPI
--- PASS: TestScanAPI (0.01s)
=== RUN   TestHealthHandler
--- PASS: TestHealthHandler (0.00s)
=== RUN   TestNewHealthHandlerWithDBAndStorage
--- PASS: TestNewHealthHandlerWithDBAndStorage (0.00s)
PASS
ok  	pixora/backend/internal/api	0.066s
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
=== RUN   TestThumbnailService
--- PASS: TestThumbnailService (0.09s)
=== RUN   TestThumbnailServiceReadOnly
--- PASS: TestThumbnailServiceReadOnly (0.00s)
PASS
ok  	pixora/backend/internal/media	0.113s
?   	pixora/backend/internal/models	[no test files]
=== RUN   TestPhotoRepositoryCRUDAndListing
--- PASS: TestPhotoRepositoryCRUDAndListing (0.00s)
=== RUN   TestAlbumRepositoryCRUDAndPhotos
--- PASS: TestAlbumRepositoryCRUDAndPhotos (0.00s)
PASS
ok  	pixora/backend/internal/repository	0.006s
=== RUN   TestScannerDiscoveryAndRepeatedScan
--- PASS: TestScannerDiscoveryAndRepeatedScan (0.10s)
=== RUN   TestScannerSymlinkEscapePrevention
--- PASS: TestScannerSymlinkEscapePrevention (0.00s)
=== RUN   TestScannerConcurrentScanProtection
--- PASS: TestScannerConcurrentScanProtection (0.00s)
=== RUN   TestScannerMissingStorageRoot
--- PASS: TestScannerMissingStorageRoot (0.00s)
PASS
ok  	pixora/backend/internal/scanner	0.102s
=== RUN   TestFrontendBackendUnifiedServing
--- PASS: TestFrontendBackendUnifiedServing (0.00s)
=== RUN   TestPhase3FullServerLifecycle
--- PASS: TestPhase3FullServerLifecycle (0.03s)
=== RUN   TestServerHealthRoute
--- PASS: TestServerHealthRoute (0.00s)
=== RUN   TestServerWithDependencies
--- PASS: TestServerWithDependencies (0.00s)
=== RUN   TestServerUnknownAPIRouteReturns404JSON
--- PASS: TestServerUnknownAPIRouteReturns404JSON (0.00s)
=== RUN   TestServerStaticAndSPAFallback
--- PASS: TestServerStaticAndSPAFallback (0.00s)
PASS
ok  	pixora/backend/internal/server	0.045s
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
PASS
ok  	pixora/backend/internal/storage	0.002s

All 27 Go tests passed (0 failures).
```

### Frontend Build & Lint Checks (`frontend`)
```
> tsc --noEmit (Passed cleanly, 0 errors)
> vite build (Built dist/index.html, dist/assets in 1.37s)
```

## 6. Manual Verification Steps for User

```bash
# 1. Run all backend tests
cd backend && go test -v -count=1 ./...

# 2. Run frontend typecheck and build
cd ../frontend && npm run typecheck && npm run build

# 3. Test running server locally with custom storage and db
cd ../backend
STORAGE_ROOT=./storage/media DATABASE_PATH=./storage/data/pixora.db go run ./cmd/server

# 4. In another terminal, trigger a media scan to index existing files
curl -s -X POST "http://localhost:8080/api/photos/scan?sync=true" | jq .

# 5. Check scan status
curl -s http://localhost:8080/api/photos/scan | jq .

# 6. List indexed photos
curl -s "http://localhost:8080/api/photos?limit=10" | jq .

# 7. Upload a photo via curl
curl -s -X POST http://localhost:8080/api/photos \
  -F "photos=@/path/to/my_photo.jpg" | jq .

# 8. Open the web interface in a browser
# Navigate to http://localhost:8080
# View real photos, test upload drag & drop, and test "Scan Media".

# 9. Build and test unified Docker container (as user)
docker build -t pixora:latest .

docker run --rm \
  -p 8080:8080 \
  -v /home/user/Pictures:/media:ro \
  -v pixora-data:/data \
  --name pixora-test pixora:latest
```
