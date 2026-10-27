# Pixora

> A clean, image-focused personal photo gallery application.

Pixora is designed for organizing photos into curated albums, browsing capture moments across an interactive timeline, and viewing high-resolution imagery in an accessible, distraction-free lightbox.

---

## Architecture Overview

Pixora uses a modern, unified single-container architecture:
- **Frontend:** React 18, TypeScript (strict), Vite, Tailwind CSS, Lucide icons, and React Router.
- **Backend:** Go 1.22+, Fiber v2 HTTP framework, and structured logging.
- **Unified Deployment:** A multi-stage `Dockerfile` compiles the frontend static assets and Go backend binary into a minimal Alpine container. The Go server directly serves both the compiled SPA assets (with client-side routing fallback) and `/api/*` REST endpoints.
- **Persistent Storage:** Host-bound photo filesystem via bind mount (`/media`), completely separated from the SQLite database volume (`/data`).

```
                          ┌──────────────────────────────────────────────┐
                          │         Pixora Alpine Container              │
                          │                                              │
  Browser Request ───────►│  Go / Fiber (Port 8080)                      │
                          │   ├── /api/health                            │
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

Pixora is designed from Phase 1 with strict separation between photo assets and relational database storage:

### 1. Storage Boundaries
- **Photo Storage Root (`STORAGE_ROOT`):** Default container mount point is `/media`. Configurable via the `STORAGE_ROOT` environment variable.
- **Application Database (`DATABASE_PATH`):** Default container path is `/data/pixora.db`. Configurable via the `DATABASE_PATH` environment variable.
- **Separation:** Photo binary files and the SQLite metadata database live on separate filesystems/mounts. The database must **never** be placed inside the photo directory, and image binaries are never stored inside SQLite.

### 2. Photo Directory Structure under `STORAGE_ROOT`
Future upload and processing phases organize files into dedicated subdirectories under `STORAGE_ROOT`:
- `/media/originals/`: Read-only preservation of uploaded original photographs, named with secure internal UUIDs.
- `/media/thumbnails/`: High-performance generated thumbnails (`w=600`) and previews (`w=1600`).
- `/media/tmp/`: Temporary processing area for file validation, EXIF extraction, and atomic moves.

### 3. Relative Database Paths
- Host-specific or container-specific absolute paths are **never** stored in the database.
- Database records will store relative storage paths (e.g., `originals/2026/10/photo-uuid.jpg` and `thumbnails/photo-uuid-thumb.webp`).
- Application code joins the configured `STORAGE_ROOT` with the relative path at runtime, with directory traversal safeguards (`ResolveStoragePath`).

### 4. Non-Root Container Permissions & Host Ownership
The runtime container runs as non-root user `pixora` with explicit UID `1000` and GID `1000`:
- **Do not assume `/media` is writable simply because it exists.** The Go server validates write permissions before operations.
- When bind-mounting a host folder (e.g. `/home/user/Pictures:/media`), ensure user `1000` on the host has read and write access:

```bash
# Option A: Set ownership to UID 1000:
sudo chown -R 1000:1000 /home/user/Pictures

# Option B: Grant group write permissions:
chmod -R u+rwX,g+rwX /home/user/Pictures
```

---

## Repository Structure

```
pixora/
├── backend/                      # Go Fiber backend service
│   ├── cmd/
│   │   └── server/
│   │       └── main.go           # Application entrypoint & graceful shutdown
│   ├── internal/
│   │   ├── api/
│   │   │   ├── health.go         # Health check handler
│   │   │   └── health_test.go    # Unit test for health endpoint
│   │   ├── config/
│   │   │   ├── config.go         # StorageRoot, DatabasePath & permission validation
│   │   │   └── config_test.go    # Storage configuration & path traversal tests
│   │   └── server/
│   │       ├── server.go         # Fiber router, static file & SPA fallback
│   │       ├── server_test.go    # Routing and 404 tests
│   │       └── integration_test.go # End-to-end frontend/backend serving tests
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
├── Dockerfile                    # Multi-stage build (FE build + BE build -> Alpine runtime)
├── docker-compose.yml            # Compose configuration with host bind mount & data volume
├── .dockerignore                 # Excludes build caches, secrets, and node_modules
├── .env.example                  # Documented environment variables
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
The backend starts on `http://localhost:8080`.

Verify the health endpoint:
```bash
curl http://localhost:8080/api/health
# Response: {"status":"ok","service":"pixora-api","version":"0.1.0"}
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
- **Stage A (Node 22 Alpine):** Installs frontend dependencies with `npm ci` and compiles static assets to `/app/frontend/dist`.
- **Stage B (Go 1.24 Alpine):** Downloads Go modules and builds a stripped, statically-linked binary (`pixora-server`).
- **Stage C (Alpine 3.21 Runtime):** Creates user `pixora` (UID 1000), prepares `/media` and `/data` mount targets, copies assets, and starts `pixora-server`.

> **Note:** As specified in `AGENTS.md`, AI agents do not execute container commands directly. You can build and run the image manually with:

```bash
# Build the unified container
docker build -t pixora:latest .

# Run with host photo bind mount and persistent database volume
docker run --rm \
  -p 8080:8080 \
  -v /home/user/Pictures:/media \
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

| Variable | Default in Container | Description |
|---|---|---|
| `PORT` | `8080` | Port on which the HTTP server listens |
| `HOST` | `0.0.0.0` | Host address to bind inside container |
| `STATIC_DIR` | `/app/public` | Path to compiled frontend assets |
| `STORAGE_ROOT` | `/media` | Target root directory for photo files |
| `DATABASE_PATH` | `/data/pixora.db` | Target file path for SQLite database |
| `APP_ENV` | `production` | Environment name (`development` or `production`) |

---

## API Contract (Phase 1)

### `GET /api/health`
Checks backend service availability.

**Response (200 OK):**
```json
{
  "status": "ok",
  "service": "pixora-api",
  "version": "0.1.0"
}
```

### Routing Rules
- All `/api/*` routes are handled by Go API handlers.
- Unknown `/api/*` routes return a JSON 404 response (`{"error": "not_found", "message": "..."}`).
- All non-API page routes (`/`, `/photos`, `/timeline`, `/folders`) fall back to `index.html` for client-side routing.
- Missing static assets with extensions (e.g. `.js`, `.css`) return a 404 status rather than falling back to `index.html`.

---

## Development Phases & Boundaries

- **Phase 1 (Completed):** Foundation, UI components, mock data separation, light/dark themes, backend health API, persistent storage architecture design, unified multi-stage Dockerfile, automated test suite.
- **Phase 2 (Next):** SQLite integration at `DATABASE_PATH`, schema migrations, photo and folder data models, and initial REST persistence.
- **Phase 3:** Photo upload pipeline, file validation, thumbnail generation under `STORAGE_ROOT`, and filesystem storage.
- **Phase 4:** Persistent folder CRUD and photo categorization.
- **Phase 5:** EXIF capture date extraction, timeline queries, and photo viewer pagination.
- **Phase 6:** Hardening, security review, backups, and production deployment.
