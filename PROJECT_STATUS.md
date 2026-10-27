# Pixora — Project Status

## 1. Project Overview

**Project:** Pixora  
**Description:** A simple web-based photo gallery for uploading photos, managing folders, and browsing photos by timeline.

**Current overall status:** Phase 2 Complete — Ready for Phase 3  
**Current phase:** Phase 2 — Backend Foundation & Data Model (Completed)

### Product Goals

- Provide a simple and responsive photo gallery.
- Support uploading one or multiple photos.
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
| Frontend | React 18 + TypeScript + Vite | Implemented | Strict TypeScript, component modularity |
| UI styling | Tailwind CSS + semantic tokens | Implemented | Light/dark CSS tokens, custom scrollbars |
| Icons | Lucide React | Implemented | Standard SVG icons across shell & actions |
| Routing | React Router v6 | Implemented | Client-side routing with Go SPA fallback |
| Backend | Go 1.22+ + Fiber v2 | Implemented | Package separation (config, storage, database, repository, api, server) |
| Database | SQLite via `modernc.org/sqlite` | Implemented | Pure Go (CGO_ENABLED=0), WAL mode, foreign keys, versioned migrations |
| Photo storage | Local host bind mount (`/media`) | Implemented | Target `STORAGE_ROOT=/media`; safe path traversal checks; read-only mount tolerated |
| Database storage | Separate persistent volume (`/data`) | Implemented | Target `DATABASE_PATH=/data/pixora.db`; parent dir auto-created; never stored in photo dir |
| Path convention | Relative storage paths in database | Implemented | Relative paths (e.g. `originals/...`); validated with `internal/storage` |
| Container access | Non-root `pixora` user (UID 1000) | Implemented | Host permissions documented; no write checks at startup |
| API style | REST + JSON | Implemented | `/api/health`, `/api/photos`, `/api/albums`; structured error envelopes |
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

### Phase 3 — Upload & Photo Management

**Status:** Up Next (Not started)

Scope reserved for Phase 3:
- Multipart file upload pipeline for single and bulk photos (`POST /api/photos/upload`).
- Image content inspection: magic bytes verification, MIME type detection, dimensions extraction, file size limits.
- Staging incoming uploads under `$STORAGE_ROOT/tmp/`.
- Organizing and committing original images with UUID filenames under `$STORAGE_ROOT/originals/`.
- Generating optimized thumbnails under `$STORAGE_ROOT/thumbnails/`.
- Atomic rollback and cleanup of temporary files on upload failure.
- Scanning existing media files in `$STORAGE_ROOT/` (if scanner feature requested).

### Phase 4 — Folder Management

**Status:** Planned

Scope:
- Full persistent folder/album CRUD (update, delete).
- Assigning and moving photos to folders via API and UI.
- Deletion policy for folders containing photos.

### Phase 5 — Timeline & Photo Viewer

**Status:** Planned

Scope:
- EXIF capture timestamp extraction (`DateTimeOriginal`).
- Fallback timestamp policy (modification time vs upload time).
- Chronological timeline query grouped by Year and Month.
- Lightbox integration with real thumbnail and original URLs replacing mock data.

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
| Database integration | Implemented | `internal/database`: Pure-Go SQLite, WAL mode, foreign keys, versioned migrations |
| Repository layer | Implemented | `internal/repository`: SQLite repositories for Photos and Albums with deterministic ordering & pagination |
| API Foundation | Implemented | `internal/api`: `/api/health`, `/api/photos`, `/api/albums`, structured error formatting |
| Photo gallery UI | Implemented | Responsive grid/list view, sorting, search filter, empty/loading states (mock data) |
| Photo viewer (Lightbox) | Implemented | Full modal, keyboard navigation, EXIF-like mock metadata panel, scroll lock |
| Timeline UI | Implemented | Chronological grouping by Month/Year with date markers (mock data) |
| Folder management UI | Implemented | Folder cards, photo count badge, prototype creation modal |
| Theme switching | Implemented | Light, dark, and system preference with localStorage persistence |
| Automated tests | Verified | 19 Go tests passing across `api`, `config`, `database`, `repository`, `server`, and `storage` |
| Frontend build | Verified | `npm run typecheck` and `npm run build` passing cleanly (0 errors) |
| Unified Dockerfile | Implemented | Multi-stage build (`CGO_ENABLED=0` statically compiled, non-root UID 1000) |

## 5. Verification Results

### Go Backend Tests (`backend`)
```
?   	pixora/backend/cmd/server	[no test files]
=== RUN   TestAlbumsAPI
--- PASS: TestAlbumsAPI (0.00s)
=== RUN   TestPhotosAPI
--- PASS: TestPhotosAPI (0.00s)
=== RUN   TestHealthHandler
--- PASS: TestHealthHandler (0.00s)
=== RUN   TestNewHealthHandlerWithDBAndStorage
--- PASS: TestNewHealthHandlerWithDBAndStorage (0.00s)
PASS
ok  	pixora/backend/internal/api	0.006s
=== RUN   TestConfigDefaultsProduction
--- PASS: TestConfigDefaultsProduction (0.00s)
=== RUN   TestConfigEnvironmentOverrides
--- PASS: TestConfigEnvironmentOverrides (0.00s)
=== RUN   TestConfigValidationInvalidValues
=== RUN   TestConfigValidationInvalidValues/empty_port
=== RUN   TestConfigValidationInvalidValues/non-numeric_port
=== RUN   TestConfigValidationInvalidValues/negative_port
=== RUN   TestConfigValidationInvalidValues/out_of_range_port
=== RUN   TestConfigValidationInvalidValues/empty_host
=== RUN   TestConfigValidationInvalidValues/empty_storage_root
=== RUN   TestConfigValidationInvalidValues/empty_database_path
=== RUN   TestConfigValidationInvalidValues/null_byte_in_storage_root
=== RUN   TestConfigValidationInvalidValues/null_byte_in_database_path
--- PASS: TestConfigValidationInvalidValues (0.00s)
=== RUN   TestResolveStoragePath
--- PASS: TestResolveStoragePath (0.00s)
=== RUN   TestCheckWritable
--- PASS: TestCheckWritable (0.00s)
PASS
ok  	pixora/backend/internal/config	0.001s
=== RUN   TestDatabaseInitializationAndMigrations
--- PASS: TestDatabaseInitializationAndMigrations (0.00s)
=== RUN   TestDatabaseForeignKeyEnforcement
--- PASS: TestDatabaseForeignKeyEnforcement (0.00s)
=== RUN   TestDatabaseUniqueConstraints
--- PASS: TestDatabaseUniqueConstraints (0.00s)
=== RUN   TestDatabasePingAndClose
--- PASS: TestDatabasePingAndClose (0.00s)
PASS
ok  	pixora/backend/internal/database	0.009s
?   	pixora/backend/internal/models	[no test files]
=== RUN   TestPhotoRepositoryCRUDAndListing
--- PASS: TestPhotoRepositoryCRUDAndListing (0.00s)
=== RUN   TestAlbumRepositoryCRUDAndPhotos
--- PASS: TestAlbumRepositoryCRUDAndPhotos (0.00s)
PASS
ok  	pixora/backend/internal/repository	0.007s
=== RUN   TestFrontendBackendUnifiedServing
--- PASS: TestFrontendBackendUnifiedServing (0.00s)
=== RUN   TestServerHealthRoute
--- PASS: TestServerHealthRoute (0.00s)
=== RUN   TestServerWithDependencies
--- PASS: TestServerWithDependencies (0.00s)
=== RUN   TestServerUnknownAPIRouteReturns404JSON
--- PASS: TestServerUnknownAPIRouteReturns404JSON (0.00s)
=== RUN   TestServerStaticAndSPAFallback
--- PASS: TestServerStaticAndSPAFallback (0.00s)
PASS
ok  	pixora/backend/internal/server	0.007s
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

All 19 tests passed (0 failures).
```

### Frontend Build & Lint Checks (`frontend`)
```
> tsc --noEmit (Passed cleanly, 0 errors)
> vite build (Built dist/index.html, dist/assets in 1.14s)
```

## 6. Manual Verification Steps for User

```bash
# 1. Run all backend tests
cd backend && go test -v ./...

# 2. Test running server locally with custom storage and db
STORAGE_ROOT=./storage/media DATABASE_PATH=./storage/data/pixora.db go run ./cmd/server

# 3. Test API health endpoint
curl -s http://localhost:8080/api/health | jq .

# 4. Test album creation & listing
curl -s -X POST http://localhost:8080/api/albums \
  -H "Content-Type: application/json" \
  -d '{"name": "Landscapes", "description": "Mountain and lake photos"}' | jq .

curl -s http://localhost:8080/api/albums | jq .

# 5. Build unified Docker container (as user)
docker build -t pixora:latest .

# 6. Run container with read-only photo mount and persistent database volume
docker run --rm \
  -p 8080:8080 \
  -v /home/user/Pictures:/media:ro \
  -v pixora-data:/data \
  --name pixora-test pixora:latest
```
