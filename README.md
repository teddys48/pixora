# Pixora

> A clean, image-focused personal photo gallery application.

Pixora is designed for organizing photos into curated albums, browsing capture moments across an interactive timeline, and viewing high-resolution imagery in an accessible, distraction-free lightbox.

---

## Architecture Overview

Pixora uses a modern, unified single-container architecture:
- **Frontend:** React 18, TypeScript (strict), Vite, Tailwind CSS, Lucide icons, and React Router.
- **Backend:** Go 1.22+, Fiber v2 HTTP framework, and structured logging.
- **Database:** SQLite with pure-Go driver (`modernc.org/sqlite`), versioned schema migrations, WAL journal mode, and foreign key enforcement.
- **Image Processing:** Pure-Go decoders and encoders (`image/jpeg`, `image/png`, `image/gif`, `golang.org/x/image/webp`, `golang.org/x/image/tiff`), EXIF parser (`github.com/rwcarlsen/goexif`), and bilinear resampler (`golang.org/x/image/draw`) compatible with `CGO_ENABLED=0`.
- **Unified Deployment:** A multi-stage `Dockerfile` compiles frontend static assets and the Go backend binary into a minimal Alpine container. The Go server directly serves both the compiled SPA assets (with client-side routing fallback) and `/api/*` REST endpoints.
- **Persistent Storage:** Host-bound photo filesystem via bind mount (`/media`), completely separated from the SQLite database volume (`/data`).

```
                          ┌──────────────────────────────────────────────┐
                          │         Pixora Alpine Container              │
                          │                                              │
  Browser Request ───────►│  Go / Fiber (Port 8080)                      │
                          │   ├── /api/health                            │
                          │   ├── /api/albums                            │
                          │   ├── /api/photos                            │
                          │   │   ├── /scan (Initiate / Query Scan)      │
                          │   │   ├── /upload (Multipart Photo Upload)   │
                          │   │   ├── :id/file (Serve Original)          │
                          │   │   └── :id/thumbnail (Serve Thumbnail)    │
                          │   └── /* (Static Assets & SPA Fallback)      │
                          │                                              │
                          │  Storage Architecture:                       │
                          │   ├── /media (STORAGE_ROOT) ◄── [Host Bind]  │
                          │   │   ├── originals/                         │
                          │   │   ├── thumbnails/                        │
                          │   │   └── tmp/                               │
                          │   └── /data (DATABASE_PATH) ◄── [Data Volume]│
                          │       └── pixora.db                          │
                          └──────────────────────────────────────────────┘
```

---

## Persistent Storage Architecture

Pixora maintains strict separation between photo assets and relational database storage:

### 1. Storage Boundaries
- **Photo Storage Root (`STORAGE_ROOT`):** Default container mount point is `/media`. Configurable via `STORAGE_ROOT`.
- **Application Database (`DATABASE_PATH`):** Default container path is `/data/pixora.db`. Configurable via `DATABASE_PATH`.
- **Separation:** Photo binary files and the SQLite metadata database live on separate filesystems/mounts. The database must **never** be placed inside the photo directory, and image binaries are never stored inside SQLite.

### 2. Read-Only Bind Mount Support
- Pixora fully supports mounting `/media` in read-only mode (`:ro`), ideal for browsing existing photo collections without risk of modification.
- The server starts normally without requiring write access to `STORAGE_ROOT`.
- Media discovery and scanning only require read permissions; existing photos are indexed in-place without copying, renaming, or modifying host files.
- Operations requiring write permissions (uploads and thumbnail generation) validate write access on-demand when invoked. If storage is mounted read-only, uploads return `403 Forbidden` (`storage_readonly`), and thumbnail requests fall back cleanly to serving the original photo.
- Persistent database storage (`/data`) must remain writable for SQLite state and migrations.

### 3. Photo Directory Structure under `STORAGE_ROOT`
Photo assets are organized into dedicated subdirectories under `STORAGE_ROOT`:
- **Existing User Photos:** Retained in their original directory structure (e.g. `/media/vacation/beach.jpg`, `/media/2026/10/img.jpg`). Existing files are indexed without moving or copying.
- `/media/originals/`: Stores newly uploaded photographs using UUID filenames, preserving original extensions.
- `/media/thumbnails/`: High-performance generated JPEG thumbnails (`thumbnails/<id>.jpg`), automatically excluded from media discovery.
- `/media/tmp/`: Temporary processing area for file upload streaming, validation, and atomic moves (`tmp/`), automatically excluded from media discovery.

### 4. Relative Database Paths & Path Traversal Protection
- Host-specific or container-specific absolute paths are **never** stored in database records.
- Records store clean relative paths (e.g. `vacation/beach.jpg` or `originals/<uuid>.jpg`).
- The storage abstraction (`internal/storage`) validates all paths, rejecting lexical traversal (`../`, absolute paths, null bytes) and symlinks that resolve outside `STORAGE_ROOT`.

### 5. Non-Root Container Permissions & Host Ownership
The runtime container runs as non-root user `pixora` with explicit UID `1000` and GID `1000`:
- When bind-mounting a host directory (e.g. `/home/user/Pictures:/media`), ensure user `1000` on the host has read access (for read-only browsing) or read/write access (for uploads):

```bash
# Set ownership to UID 1000:
sudo chown -R 1000:1000 /home/user/Pictures

# Or grant group permissions:
chmod -R u+rwX,g+rwX /home/user/Pictures
```

---

## Media Discovery & Scanning

Pixora discovers and indexes existing photos directly from `STORAGE_ROOT`:

- **In-Place Indexing:** Existing photos remain in their folders and are never moved, renamed, or modified.
- **Supported Formats:** JPEG (`.jpg`, `.jpeg`), PNG (`.png`), WebP (`.webp`), GIF (`.gif`), and TIFF (`.tif`, `.tiff`).
- **Directory Walking:** Recursively traverses subdirectories while ignoring thumbnails (`thumbnails/`), temporary files (`tmp/`), and hidden/dot directories (`.git`, `.hidden`).
- **Duplicate Prevention:** Records are uniquely keyed by `relative_path` in SQLite. Subsequent scans check file modification times and sizes; unchanged files are skipped instantly without re-processing.
- **Reconciliation:** If a file's size or timestamp changes on disk, its metadata (dimensions, timestamps) is automatically re-extracted and updated in the database without altering its stable photo ID.
- **Fault-Tolerant:** Corrupt or incomplete images are counted as failed and logged without interrupting the discovery of valid files.
- **Symlink Protection:** Symlinks resolving outside `STORAGE_ROOT` are safely rejected.
- **Concurrency Control:** Atomic locking ensures only one media scan can run at any given time, preventing duplicate database entries and resource exhaustion.

---

## Photo Upload Pipeline

New photos can be uploaded via `POST /api/photos` or `POST /api/photos/upload`:

- **Content Validation:** The server sniffs magic bytes and inspects image headers rather than trusting client filenames or `Content-Type` headers.
- **Streaming Uploads:** Multipart files are streamed directly into temporary files under `tmp/` using bounded readers, preventing buffer exhaustion.
- **Collision Avoidance:** Server assigns collision-safe UUID filenames (`originals/<uuid>.<ext>`), storing original filenames in database metadata.
- **Decompression Bomb Protection:** Pre-validates decoded image dimensions against safety thresholds (max 15,000px per dimension, max 100 megapixels total).
- **Atomic Commits:** Files are moved atomically into `originals/`. If database persistence fails, uploaded files and generated thumbnails are immediately cleaned up.
- **Upload Limits:**
  - Max file size: 25MB per file.
  - Max batch size: 20 files per request.

---

## Metadata & Thumbnail Generation

- **Metadata Extraction:**
  - MIME type from magic headers.
  - Image dimensions (width and height).
  - File size and modification time.
  - Capture time (`taken_at`) extracted from EXIF `DateTimeOriginal` / `DateTime` when present.
- **Orientation Normalization:** EXIF orientation tags (1–8) are respected during thumbnail creation, correctly rotating and flipping smartphone photos.
- **Thumbnail Optimization:** High-quality bilinear resampling within a 400×400px bounding box preserving original aspect ratio. Encoded as optimized JPEG (85% quality).
- **On-Demand & Cached:** Thumbnails are cached on disk under `thumbnails/<id>.jpg`. Subsequent requests serve the cached file with conditional HTTP caching (`ETag`, `304 Not Modified`, `Cache-Control`).
- **Read-Only Fallback:** If `STORAGE_ROOT` is read-only and a thumbnail does not exist, the server falls back to serving the original photo.

---

## API Contract

### Health Check
- `GET /api/health` — Checks backend service availability, SQLite database connection, and storage readiness.

### Photo Discovery & Scanning
- `POST /api/photos/scan` — Initiates a media discovery scan.
  - Query parameters: `sync=true` runs synchronously; default runs asynchronously.
  - Returns `202 Accepted` with initial status, or `200 OK` when synchronous.
  - Returns `409 Conflict` if a scan is already in progress.
- `GET /api/photos/scan` (alias: `GET /api/photos/scan/status`) — Returns current or last scan progress:
```json
{
  "status": "completed",
  "scanned": 15,
  "indexed": 12,
  "updated": 0,
  "skipped": 3,
  "failed": 0,
  "durationMs": 45
}
```

### Photo Collection & Retrieval
- `GET /api/photos` — Paginated photo listing.
  - Query parameters:
    - `limit` (integer, default: 50, max: 100)
    - `offset` (integer, default: 0)
    - `sort` (`date`, `filename`, `size`, `created`, `modified`, default: `date`)
    - `order` (`asc`, `desc`, default: `desc`)
  - Returns:
```json
{
  "photos": [
    {
      "id": "e2f12c1b-...",
      "relativePath": "vacation/beach.jpg",
      "originalFilename": "beach.jpg",
      "mediaType": "image/jpeg",
      "fileSize": 1048576,
      "width": 1920,
      "height": 1080,
      "takenAt": "2026-08-15T10:30:00Z",
      "fileModifiedAt": "2026-10-20T12:00:00Z",
      "createdAt": "2026-10-27T10:00:00Z",
      "updatedAt": "2026-10-27T10:00:00Z",
      "url": "/api/photos/e2f12c1b-.../file",
      "thumbnailUrl": "/api/photos/e2f12c1b-.../thumbnail"
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

- `GET /api/photos/:id` — Retrieves metadata for a single photo.
- `GET /api/photos/:id/file` — Securely serves the original photograph with `ETag` and `Cache-Control`.
- `GET /api/photos/:id/thumbnail` — Serves the generated thumbnail with `ETag` and `Cache-Control`.

### Photo Upload
- `POST /api/photos` (alias: `POST /api/photos/upload`) — Upload one or multiple photos via `multipart/form-data` (`photos` field).
  - Returns `201 Created` with created photo records.
  - Returns `207 Multi-Status` on partial upload success with error list.
  - Returns `403 Forbidden` if storage is read-only.
  - Returns `400 Bad Request` or `422 Unprocessable Entity` on invalid files.

### Virtual Albums
- `GET /api/albums` — Lists virtual albums with photo counts.
- `POST /api/albums` — Creates a new virtual album (`{"name": "...", "description": "..."}`).
- `GET /api/albums/:id` — Retrieves album details.

---

## Development Phases & Boundaries

- **Phase 1 (Completed):** Foundation, UI components, mock data separation, light/dark themes, backend health API, persistent storage architecture design, unified multi-stage Dockerfile, automated test suite.
- **Phase 2 (Completed):** Backend foundation & data model. SQLite integration with pure-Go driver (`modernc.org/sqlite`), versioned schema migrations, photo and album entities, repository layer, storage abstraction with path traversal guards, read-only media root support, and structured API error handling.
- **Phase 3 (Completed):** Existing media discovery & recursive scanning, deduplication and change reconciliation, secure photo and thumbnail serving, metadata extraction (EXIF timestamps, dimensions, orientation), streaming upload pipeline, and frontend integration with real API.
- **Phase 4 (Planned):** Persistent folder/album CRUD and photo categorization.
- **Phase 5 (Planned):** Chronological timeline grouping by Month/Year and full lightbox keyboard viewer integration.
- **Phase 6 (Planned):** Hardening, security review, backups, and production deployment.
