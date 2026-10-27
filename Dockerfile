# ==============================================================================
# Stage A: Frontend Build
# ==============================================================================
FROM node:22-alpine AS frontend-builder

WORKDIR /app/frontend

# Install dependencies using committed lockfile for reproducible builds
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

# Copy frontend source code and configuration
COPY frontend/ ./

# Build production static assets (outputs to /app/frontend/dist)
RUN npm run build

# ==============================================================================
# Stage B: Backend Build
# ==============================================================================
FROM golang:1.24-alpine AS backend-builder

WORKDIR /app/backend

# Install build prerequisites
RUN apk add --no-cache git ca-certificates

# Download dependencies using Go module files
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy backend source files
COPY backend/ ./

# Build statically-linked binary stripped of debug information
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w -extldflags '-static'" \
    -o /app/backend/pixora-server \
    ./cmd/server

# ==============================================================================
# Stage C: Minimal Runtime
# ==============================================================================
FROM alpine:3.21 AS runtime

# Install basic root certificates for HTTPS outgoing requests and tzdata
RUN apk --no-cache add ca-certificates tzdata

# Create dedicated non-root user and group with standard UID/GID 1000
# (Aligns with typical Linux host user for seamless bind mount access)
RUN addgroup -g 1000 -S pixora && \
    adduser -u 1000 -S pixora -G pixora

WORKDIR /app

# Create separate mount points for photo files and SQLite database
RUN mkdir -p /media /data && \
    chown -R pixora:pixora /media /data /app

# Copy compiled Go backend binary from Stage B
COPY --from=backend-builder --chown=pixora:pixora /app/backend/pixora-server /app/pixora-server

# Copy compiled frontend static assets from Stage A into /app/public
COPY --from=frontend-builder --chown=pixora:pixora /app/frontend/dist /app/public

# Default environment configuration
ENV PORT=8080 \
    HOST=0.0.0.0 \
    STATIC_DIR=/app/public \
    APP_ENV=production \
    STORAGE_ROOT=/media \
    DATABASE_PATH=/data/pixora.db

# Mount point documentation
VOLUME ["/media", "/data"]

# Switch to non-root user
USER pixora

# Expose default application port
EXPOSE 8080

# Health check using wget (included in Alpine busybox)
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:${PORT:-8080}/api/health || exit 1

# Start Pixora unified server
ENTRYPOINT ["/app/pixora-server"]
