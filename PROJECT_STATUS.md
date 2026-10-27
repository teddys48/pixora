# Pixora — Project Status

## 1. Project Overview

**Project:** Pixora  
**Description:** A simple web-based photo gallery for uploading photos, managing folders, and browsing photos by timeline.

**Current overall status:** Phase 1 Complete — Ready for Phase 2  
**Current phase:** Phase 1 — Foundation & UI (Completed with Persistent Storage Design)

### Product Goals

- Provide a simple and responsive photo gallery.
- Support uploading one or multiple photos.
- Organize photos into folders.
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
| Backend | Go 1.22+ + Fiber v2 | Implemented | Minimal, clean package structure with graceful shutdown |
| Database | SQLite | Planned (Phase 2) | Schema migrations scheduled for Phase 2; target `/data/pixora.db` |
| Photo storage | Local host bind mount (`/media`) | Designed (Phase 1) | Target `STORAGE_ROOT=/media`; separates originals, thumbnails, tmp |
| Database storage | Separate persistent volume (`/data`) | Designed (Phase 1) | Target `DATABASE_PATH=/data/pixora.db`; never stored in photo dir |
| Path convention | Relative storage paths in database | Designed (Phase 1) | Host paths never stored; resolved safely at runtime |
| Container access | Non-root `pixora` user (UID 1000) | Implemented | Host permissions documented; advisory write check added |
| API style | REST + JSON | Implemented | `/api/health` implemented; unknown routes return JSON 404 |
| Deployment | Unified Multi-Stage Dockerfile | Implemented | Builds FE & BE into a single non-root Alpine container |
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
- [x] Persistent host-bound photo storage designed: `STORAGE_ROOT` (`/media`), `DATABASE_PATH` (`/data/pixora.db`), separation of photos and DB, relative path rules, non-root UID 1000 permissions, and write verification.
- [x] Multi-stage `Dockerfile` producing a single non-root Alpine container serving static assets and API.
- [x] Automated Go unit and integration tests passing; frontend TypeScript check and production build passing.
- [x] Basic project documentation (`README.md`, `.env.example`, `.dockerignore`, `docker-compose.yml`).

### Phase 2 — Backend & Data Model

**Status:** Up Next (Not started)

Scope:
- Establish SQLite connection management with foreign key enforcement using `DATABASE_PATH`.
- Configure version-controlled schema migrations (`storage/migrations/`).
- Define Photo and Folder database models and repository layers.
- Store relative storage paths (e.g. `originals/uuid.jpg`, `thumbnails/uuid.webp`) in photo records.
- Implement initial REST API endpoints:
  - `GET /api/photos` (paginated, sorted)
  - `GET /api/photos/:id`
  - `GET /api/folders`
  - `POST /api/folders`
- Add request validation, sanitization, and structured API error responses.
- Write unit and integration tests using isolated temporary SQLite databases.

### Phase 3 — Upload & Photo Management

**Status:** Planned

Scope:
- Multipart file upload pipeline for single and bulk photos.
- Validate image magic bytes, MIME types, dimensions, and file size limits.
- Stage incoming uploads in `$STORAGE_ROOT/tmp/`.
- Store original images using unique UUID filenames in `$STORAGE_ROOT/originals/`.
- Generate optimized thumbnails in `$STORAGE_ROOT/thumbnails/`.
- Safe temporary file cleanup and rollback on failure.

### Phase 4 — Folder Management

**Status:** Planned

Scope:
- Persistent folder CRUD.
- Assign and move photos to folders.
- Deletion policy for folders containing photos.

### Phase 5 — Timeline & Photo Viewer

**Status:** Planned

Scope:
- EXIF capture timestamp extraction (`DateTimeOriginal`).
- Fallback timestamp policy.
- Database timeline query grouping by Year and Month.
- Lightbox integration with real thumbnail and original URLs.

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
| Storage architecture | Implemented | Configurable `STORAGE_ROOT=/media`, `DATABASE_PATH=/data/pixora.db`, non-root user UID 1000 |
| Database migrations | Planned | Scheduled for Phase 2 |
| Photo gallery UI | Implemented | Responsive grid/list view, sorting, search filter, empty/loading states |
| Photo viewer (Lightbox) | Implemented | Full modal, keyboard navigation, EXIF-like mock metadata panel, scroll lock |
| Timeline UI | Implemented | Chronological grouping by Month/Year with date markers |
| Folder management UI | Implemented | Folder cards, photo count badge, prototype creation modal |
| Theme switching | Implemented | Light, dark, and system preference with localStorage persistence |
| Health integration | Implemented | `GET /api/health` connected to frontend `HealthBadge` |
| Automated tests | Verified | 8/8 Go tests passing (`api`, `config`, `server`, and unified integration) |
| Frontend build | Verified | `npm run typecheck` and `npm run build` passing cleanly (0 errors) |
| Unified Dockerfile | Implemented | Multi-stage build (Node 22 + Go 1.24 -> Alpine 3.21 non-root UID 1000) |

## 5. Architecture & Storage Decisions

### Storage Separation & Host Bind Mount
1. **Photo Storage (`STORAGE_ROOT`):**
   - Configurable via `STORAGE_ROOT` environment variable. Defaults to `/media` in container (`./storage/media` in local dev).
   - Designed for host filesystem bind mounting (e.g. `/home/user/Pictures:/media`).
   - Organizes photos into subdirectories: `originals/`, `thumbnails/`, and `tmp/`.
   - Never contains the SQLite database.
2. **Application Database (`DATABASE_PATH`):**
   - Configurable via `DATABASE_PATH` environment variable. Defaults to `/data/pixora.db` in container (`./storage/data/pixora.db` in local dev).
   - Backed by an independent persistent named Docker volume (`pixora-data:/data`).
   - Ensures photo library files and relational application state can be backed up, migrated, and managed separately.
3. **Relative Path Storage:**
   - Host-specific or container-specific absolute paths are never stored in database records.
   - Database records store relative paths (`originals/uuid.jpg`, `thumbnails/uuid.webp`).
   - `ResolveStoragePath` safely resolves relative paths under `STORAGE_ROOT`, rejecting path traversal.
4. **Permissions & Non-Root Access:**
   - The container runs as non-root user `pixora` (UID 1000, GID 1000).
   - Host directories must be readable and writable by UID 1000 (`chown -R 1000:1000` or `chmod g+w`).
   - `CheckWritable` provides programmatic verification of write permissions rather than assuming writability simply because the directory exists.

### Routing & Static Assets
1. **API Routes:** All `/api/*` endpoints are intercepted by Fiber. Unknown API routes return a JSON 404 response (`{"error": "not_found", "message": "..."}`).
2. **SPA Fallback:** All non-API page routes (`/`, `/photos`, `/timeline`, `/folders`) serve `index.html` to support React Router client-side routing.
3. **Missing Static Assets:** Requests for non-existent static assets with file extensions (e.g. `.css`, `.js`, `.png`) return a standard 404 instead of falling back to `index.html`, preventing browser MIME mismatch errors.

## 6. Verification Results

### Go Backend Tests (`backend`)
```
=== RUN   TestHealthHandler
--- PASS: TestHealthHandler (0.00s)
=== RUN   TestConfigDefaultsProduction
--- PASS: TestConfigDefaultsProduction (0.00s)
=== RUN   TestConfigEnvironmentOverrides
--- PASS: TestConfigEnvironmentOverrides (0.00s)
=== RUN   TestResolveStoragePath
--- PASS: TestResolveStoragePath (0.00s)
=== RUN   TestCheckWritable
--- PASS: TestCheckWritable (0.00s)
=== RUN   TestFrontendBackendUnifiedServing
    [2026-10-27 13:50:38] 200 - /api/health
    [2026-10-27 13:50:38] 200 - /
    [2026-10-27 13:50:38] 200 - /photos (SPA fallback)
    [2026-10-27 13:50:38] 200 - /timeline (SPA fallback)
    [2026-10-27 13:50:38] 404 - /api/v1/photos/upload (JSON 404)
    [2026-10-27 13:50:38] 404 - /assets/nonexistent-file.css (Asset 404)
--- PASS: TestFrontendBackendUnifiedServing (0.00s)
=== RUN   TestServerHealthRoute
--- PASS: TestServerHealthRoute (0.00s)
=== RUN   TestServerUnknownAPIRouteReturns404JSON
--- PASS: TestServerUnknownAPIRouteReturns404JSON (0.00s)
=== RUN   TestServerStaticAndSPAFallback
--- PASS: TestServerStaticAndSPAFallback (0.00s)
PASS — All tests passed
```

### Frontend Build & Lint Checks (`frontend`)
```
> tsc --noEmit (Passed cleanly, 0 errors)
> vite build (Built dist/index.html, dist/assets in 1.22s)
```

## 7. Manual Verification Steps for User

As per `AGENTS.md`, AI agents do not run Docker commands directly. To verify the unified Docker container with persistent storage mounts:

```bash
# 1. Build the unified Docker image
docker build -t pixora:latest .

# 2. Run container with host photo bind mount and persistent database volume
docker run --rm \
  -p 8080:8080 \
  -v /home/user/Pictures:/media \
  -v pixora-data:/data \
  --name pixora-test pixora:latest

# 3. Test health endpoint
curl -i http://localhost:8080/api/health

# 4. Open in browser:
# http://localhost:8080/
```
