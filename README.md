# Pixora

> A clean, image-focused personal photo gallery application.

Pixora is designed for organizing photos into curated albums, browsing capture moments across an interactive timeline, and viewing high-resolution imagery in an accessible, distraction-free lightbox.

---

## Architecture Overview

Pixora uses a modern, unified single-container architecture:
- **Frontend:** React 18, TypeScript (strict), Vite, Tailwind CSS, Lucide icons, and React Router.
- **Backend:** Go 1.22+, Fiber v2 HTTP framework, and structured logging.
- **Database:** SQLite with pure-Go driver (`modernc.org/sqlite`), versioned schema migrations, WAL journal mode, and foreign key enforcement.
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
- Startup routines do not create test files or modify host directories.
- Operations requiring write permissions (such as uploads and thumbnail generation in future phases) validate write access on-demand when invoked.
- Persistent database storage (`/data`) must remain writable for SQLite state and migrations.

### 3. Photo Directory Structure under `STORAGE_ROOT`
Photo assets are conceptually organized into dedicated subdirectories under `STORAGE_ROOT`:
- `/media/originals/`: Preservation of original photographs, referenced via relative paths.
- `/media/thumbnails/`: High-performance generated thumbnails (`thumbnails/`).
- `/media/tmp/`: Temporary processing area for file validation and atomic moves (`tmp/`).

### 4. Relative Database Paths & Path Traversal Protection
- Host-specific or container-specific absolute paths are **never** stored in database records.
- Records store clean relative paths (e.g. `originals/2026/10/img_1234.jpg`).
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

## Database Architecture & Migrations

Pixora uses SQLite with `modernc.org/sqlite`, a pure Go driver compatible with `CGO_ENABLED=0` static builds.

- **Foreign Keys:** Enforced on every connection (`PRAGMA foreign_keys = ON;`).
- **Journal Mode:** Write-Ahead Logging (`PRAGMA journal_mode = WAL;`) for concurrent read access without blocking readers during writes.
- **Busy Timeout:** 5000ms (`PRAGMA busy_timeout = 5000;`).
- **Connection Pool:** Max 10 open connections, 5 idle connections, 1 hour lifetime.
- **Schema Migrations:** Version-controlled migration system (`schema_migrations` table) executed sequentially inside atomic database transactions.

### Database Schema (Version 1)

- **`photos`**:
  - `id` (TEXT PRIMARY KEY) — UUID.
  - `relative_path` (TEXT NOT NULL UNIQUE) — Storage path relative to `STORAGE_ROOT`.
  - `original_filename` (TEXT NOT NULL).
  - `media_type` (TEXT NOT NULL) — MIME type.
  - `file_size` (INTEGER NOT NULL) — In bytes.
  - `width` (INTEGER NULLABLE), `height` (INTEGER NULLABLE).
  - `taken_at` (DATETIME NULLABLE) — Capture time.
  - `file_modified_at` (DATETIME NOT NULL).
  - `created_at` (DATETIME NOT NULL), `updated_at` (DATETIME NOT NULL).
  - *Indexes:* `idx_photos_taken_at`, `idx_photos_created_at`, `idx_photos_relative_path`.

- **`albums`**:
  - `id` (TEXT PRIMARY KEY) — UUID.
  - `name` (TEXT NOT NULL).
  - `description` (TEXT NOT NULL DEFAULT '').
  - `created_at` (DATETIME NOT NULL), `updated_at` (DATETIME NOT NULL).
  - *Indexes:* `idx_albums_name`.

- **`photo_albums`** (Virtual album join table):
  - `photo_id` (TEXT NOT NULL REFERENCES photos(id) ON DELETE CASCADE).
  - `album_id` (TEXT NOT NULL REFERENCES albums(id) ON DELETE CASCADE).
  - `added_at` (DATETIME NOT NULL).
  - *Primary Key:* `(photo_id, album_id)`.
  - *Indexes:* `idx_photo_albums_album_id`, `idx_photo_albums_photo_id`.

---

## Repository Structure

```
pixora/
├── backend/                      # Go Fiber backend service
│   ├── cmd/
│   │   └── server/
│   │       └── main.go           # Application entrypoint & graceful shutdown
│   ├── internal/
│   │   ├── api/                  # API handlers and standard JSON envelopes
│   │   │   ├── albums.go         # Album listing and creation handlers
│   │   │   ├── api_test.go       # API integration tests
│   │   │   ├── health.go         # Health check handler (DB & storage status)
│   │   │   ├── health_test.go    # Health check tests
│   │   │   ├── photos.go         # Photo listing and lookup handlers
│   │   │   └── response.go       # Standard error response helper
│   │   ├── config/               # Centralized configuration & validation
│   │   │   ├── config.go         # Env loader, validation, defaults
│   │   │   └── config_test.go    # Config tests (valid & invalid values)
│   │   ├── database/             # SQLite connection & migration engine
│   │   │   ├── db.go             # DB connection, PRAGMAs, pool configuration
│   │   │   ├── db_test.go        # Foreign keys, constraints, migration tests
│   │   │   └── migrations.go     # Versioned schema migrations
│   │   ├── models/               # Domain models (Photo, Album, PhotoAlbum)
│   │   │   └── models.go
│   │   ├── repository/           # Data access repository layer
│   │   │   ├── album.go          # SQLite album repository implementation
│   │   │   ├── errors.go         # Common repository errors
│   │   │   ├── photo.go          # SQLite photo repository implementation
│   │   │   └── repository_test.go # Repository CRUD, sorting, and pagination tests
│   │   ├── server/               # Fiber router, middleware, and static serving
│   │   │   ├── integration_test.go # Unified frontend/backend tests
│   │   │   ├── server.go         # Server assembly with dependency injection
│   │   │   └── server_test.go    # Route tests
│   │   └── storage/              # Storage abstraction & path traversal safeguards
│   │       ├── storage.go        # Path resolution, read/write checks
│   │       └── storage_test.go   # Traversal, symlink, read-only tests
│   ├── go.mod
│   └── go.sum
├── frontend/                     # React + TypeScript + Vite frontend
│   ├── public/
│   │   └── favicon.svg           # Application SVG icon
│   ├── src/
│   │   ├── components/
│   │   │   ├── folders/          # FolderCard, CreateFolderModal
│   │   │   ├── gallery/          # PhotoCard, GalleryControls, LightboxModal, states
│   │   │   ├── layout/           # AppLayout, Sidebar, Header, HealthBadge, ThemeToggle
│   │   │   └── ui/               # Button, Input, Modal, Badge, Skeleton, FallbackImage
│   │   ├── context/
│   │   │   └── ThemeContext.tsx  # Light/dark/system theme state & localStorage
│   │   ├── data/
│   │   │   └── mock/             # Dedicated mock datasets (photos, folders)
│   │   ├── lib/
│   │   │   ├── api.ts            # Centralized API client & health check
│   │   │   └── utils.ts          # Class merging, date & byte formatters
│   │   ├── pages/
│   │   │   ├── PhotosPage.tsx    # Gallery landing with search, sort, and grid/list view
│   │   │   ├── TimelinePage.tsx  # Chronological date-grouped browsing
│   │   │   └── FoldersPage.tsx   # Album collections overview & prototype modal
│   │   ├── types/                # TypeScript models (Photo, Folder, Health)
│   │   ├── App.tsx               # Client routes and Lightbox orchestration
│   │   ├── main.tsx              # React DOM mounting
│   │   └── index.css             # Tailwind base styles and semantic tokens
│   ├── package.json
│   ├── tsconfig.json
│   ├── tailwind.config.js
│   └── vite.config.ts
├── Dockerfile                    # Multi-stage build (Node 22 + Go 1.24 -> Alpine 3.21)
├── docker-compose.yml            # Compose configuration with host bind mount & data volume
├── .dockerignore                 # Excludes build caches, secrets, and node_modules
├── .env.example                  # Documented environment variables with read-only notes
├── AGENTS.md                     # Engineering standards & phase rules
└── PROJECT_STATUS.md             # Implementation inventory & phase tracking
```

---

## Getting Started

### Prerequisites
- Node.js 18+ and npm 9+
- Go 1.22+
- Docker or Podman (for containerized deployment)

---

### Local Development

#### 1. Backend Server
In a terminal, start the Go server:

```bash
cd backend
go run ./cmd/server
```
The backend automatically creates `./storage/data/pixora.db`, applies schema migrations, and listens on `http://localhost:8080`.

Verify the health endpoint:
```bash
curl http://localhost:8080/api/health
# Response: {"status":"ok","service":"pixora-api","version":"0.1.0","database":"connected","storage":"ready"}
```

#### 2. Frontend Development Server
In a separate terminal, start Vite:

```bash
cd frontend
npm install
npm run dev
```
Open `http://localhost:3000` in your browser. Requests to `/api/*` are automatically proxied to `http://localhost:8080`.

---

## Production Build & Unified Container

### Building Without Docker
To build the static frontend and run it directly with the Go binary:

```bash
# 1. Build frontend
cd frontend
npm run build
cd ..

# 2. Run backend pointing to built frontend assets
cd backend
STATIC_DIR=../frontend/dist go run ./cmd/server
```
Open `http://localhost:8080` to experience the fully compiled application served by Go.

### Building With Docker

The repository includes a multi-stage `Dockerfile`:
- **Stage A (Node 22 Alpine):** Compiles static assets to `/app/frontend/dist`.
- **Stage B (Go 1.24 Alpine):** Compiles static pure-Go binary with `CGO_ENABLED=0`.
- **Stage C (Alpine 3.21 Runtime):** Runs non-root `pixora` (UID 1000) with volume mount points `/media` and `/data`.

> **Note:** As specified in `AGENTS.md`, AI agents do not execute container commands directly. You can build and run the image manually with:

```bash
# Build the unified container
docker build -t pixora:latest .

# Run with host photo bind mount (read-only supported) and persistent database volume
docker run --rm \
  -p 8080:8080 \
  -v /home/user/Pictures:/media:ro \
  -v pixora-data:/data \
  --name pixora pixora:latest

# Verify health check in running container
curl http://localhost:8080/api/health
```

Or using Docker Compose:
```bash
# Set your photos directory in .env or run directly:
PIXORA_PHOTOS_DIR=/home/user/Pictures docker compose up --build
```

---

## Environment Variables

| Variable | Default in Container | Local Dev Default | Description |
|---|---|---|---|
| `PORT` | `8080` | `8080` | Port on which the HTTP server listens |
| `HOST` | `0.0.0.0` | `0.0.0.0` | Host address to bind |
| `STATIC_DIR` | `/app/public` | `./frontend/dist` | Path to compiled frontend assets |
| `STORAGE_ROOT` | `/media` | `./storage/media` | Target root directory for photo files |
| `DATABASE_PATH` | `/data/pixora.db` | `./storage/data/pixora.db` | Target file path for SQLite database |
| `APP_ENV` | `production` | `development` | Environment name (`development` or `production`) |

---

## API Contract (Implemented in Phase 2)

### `GET /api/health`
Checks backend service availability, SQLite database connection, and storage readiness.

**Response (200 OK):**
```json
{
  "status": "ok",
  "service": "pixora-api",
  "version": "0.1.0",
  "database": "connected",
  "storage": "ready"
}
```

### `GET /api/photos`
Retrieves a paginated list of indexed photos.
- Query parameters:
  - `limit` (integer, 1–100, default: 50)
  - `offset` (integer, default: 0)
  - `sort` (`date`, `filename`, `size`, `created`, default: `date`)
  - `order` (`asc`, `desc`, default: `desc`)

**Response (200 OK):**
```json
{
  "photos": [],
  "total": 0,
  "limit": 50,
  "offset": 0
}
```

### `GET /api/photos/:id`
Retrieves a single photo by its stable ID.

### `GET /api/albums`
Retrieves all virtual albums with their respective photo counts.

**Response (200 OK):**
```json
{
  "albums": [
    {
      "id": "c61b2a24-...",
      "name": "Summer Vacation",
      "description": "Beach trip",
      "photoCount": 12,
      "createdAt": "2026-10-27T12:00:00Z",
      "updatedAt": "2026-10-27T12:00:00Z"
    }
  ]
}
```

### `POST /api/albums`
Creates a new virtual album.
- Body: `{"name": "string (1-100 chars)", "description": "optional string"}`
- Returns `201 Created` with created album, or `400 Bad Request` with structured error.

### `GET /api/albums/:id`
Retrieves a single album by its ID.

---

## Development Phases & Boundaries

- **Phase 1 (Completed):** Foundation, UI components, mock data separation, light/dark themes, backend health API, persistent storage architecture design, unified multi-stage Dockerfile, automated test suite.
- **Phase 2 (Completed):** Backend foundation & data model. SQLite integration with pure-Go driver (`modernc.org/sqlite`), versioned schema migrations, photo and album entities, repository layer, storage abstraction with path traversal guards, read-only media root support, and structured API error handling.
- **Phase 3 (Next):** Photo upload pipeline, file validation, thumbnail generation under `STORAGE_ROOT`, and filesystem storage.
- **Phase 4:** Persistent folder CRUD and photo categorization.
- **Phase 5:** EXIF capture date extraction, timeline queries, and photo viewer pagination.
- **Phase 6:** Hardening, security review, backups, and production deployment.
