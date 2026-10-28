package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"pixora/backend/internal/api"
	"pixora/backend/internal/config"
	"pixora/backend/internal/database"
	"pixora/backend/internal/media"
	"pixora/backend/internal/repository"
	"pixora/backend/internal/scanner"
	"pixora/backend/internal/storage"
)

// Dependencies bundles all server runtime dependencies.
type Dependencies struct {
	Config    *config.Config
	DB        *database.DB
	Storage   *storage.Storage
	PhotoRepo repository.PhotoRepository
	AlbumRepo repository.AlbumRepository
	Thumbnail *media.ThumbnailService
	Scanner   *scanner.Scanner
}

// New creates and configures the Fiber application using Config only (backward compatibility).
func New(cfg *config.Config) *fiber.App {
	return NewWithDependencies(Dependencies{
		Config: cfg,
	})
}

// NewWithDependencies creates and configures the Fiber application with all runtime dependencies.
func NewWithDependencies(deps Dependencies) *fiber.App {
	cfg := deps.Config
	if cfg == nil {
		cfg = config.Load()
	}

	app := fiber.New(fiber.Config{
		AppName:      "Pixora Photo Gallery",
		ServerHeader: "Pixora",
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}

			// For API routes, return structured JSON error
			if strings.HasPrefix(c.Path(), "/api") {
				return c.Status(code).JSON(fiber.Map{
					"error":   "server_error",
					"message": err.Error(),
				})
			}

			return c.Status(code).SendString(err.Error())
		},
	})

	// Middlewares
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format:     "[${time}] ${status} - ${latency} ${method} ${path}\n",
		TimeFormat: "2006-01-02 15:04:05",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
		AllowMethods: "GET, POST, HEAD, OPTIONS, PATCH, DELETE",
	}))

	// API Routes
	apiGroup := app.Group("/api")

	// Health check with database and storage inspection
	if deps.DB != nil || deps.Storage != nil {
		apiGroup.Get("/health", api.NewHealthHandler(deps.DB, deps.Storage))
	} else {
		apiGroup.Get("/health", api.HealthHandler)
	}

	// Album routes
	if deps.AlbumRepo != nil {
		apiGroup.Get("/albums", api.ListAlbumsHandler(deps.AlbumRepo))
		apiGroup.Post("/albums", api.CreateAlbumHandler(deps.AlbumRepo))
		apiGroup.Get("/albums/:id", api.GetAlbumHandler(deps.AlbumRepo))
		apiGroup.Patch("/albums/:id", api.UpdateAlbumHandler(deps.AlbumRepo))
		apiGroup.Delete("/albums/:id", api.DeleteAlbumHandler(deps.AlbumRepo))
		apiGroup.Get("/albums/:id/photos", api.GetAlbumPhotosHandler(deps.AlbumRepo))
		apiGroup.Post("/albums/:id/photos", api.AddPhotosToAlbumHandler(deps.AlbumRepo))
		apiGroup.Delete("/albums/:id/photos/:photoId", api.RemovePhotoFromAlbumHandler(deps.AlbumRepo))
	}

	// Physical Folder browsing routes
	if deps.Storage != nil && deps.PhotoRepo != nil {
		apiGroup.Get("/folders/contents", api.GetFolderContentsHandler(deps.Storage, deps.PhotoRepo))
		apiGroup.Get("/folders/photos", api.GetFolderPhotosHandler(deps.Storage, deps.PhotoRepo))
		apiGroup.Get("/folders", api.ListFoldersHandler(deps.Storage, deps.PhotoRepo))
	}

	// Photo Collection and Upload routes
	if deps.PhotoRepo != nil {
		apiGroup.Get("/photos", api.ListPhotosHandler(deps.PhotoRepo))
		apiGroup.Post("/photos", api.UploadPhotosHandler(deps.PhotoRepo, deps.Storage, deps.Thumbnail))
		apiGroup.Post("/photos/upload", api.UploadPhotosHandler(deps.PhotoRepo, deps.Storage, deps.Thumbnail))
	}

	// Photo Media Scanner routes
	if deps.Scanner != nil {
		apiGroup.Get("/photos/scan", api.GetScanStatusHandler(deps.Scanner))
		apiGroup.Get("/photos/scan/status", api.GetScanStatusHandler(deps.Scanner))
		apiGroup.Post("/photos/scan", api.ScanPhotosHandler(deps.Scanner))
	}

	// Individual Photo and Asset Serving routes
	if deps.PhotoRepo != nil {
		apiGroup.Get("/photos/:id", api.GetPhotoHandler(deps.PhotoRepo))
		if deps.Storage != nil {
			apiGroup.Get("/photos/:id/file", api.ServeOriginalPhotoHandler(deps.PhotoRepo, deps.Storage))
			apiGroup.Get("/photos/:id/thumbnail", api.ServeThumbnailHandler(deps.PhotoRepo, deps.Storage, deps.Thumbnail))
		}
	}

	// Explicit 404 handler for unknown /api/* endpoints
	apiGroup.All("/*", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   "not_found",
			"message": fmt.Sprintf("API endpoint '%s' not found", c.Path()),
		})
	})

	// Static files & SPA handling
	setupStaticAndSPA(app, cfg.StaticDir)

	return app
}

func setupStaticAndSPA(app *fiber.App, staticDir string) {
	cleanStaticDir := filepath.Clean(staticDir)
	indexPath := filepath.Join(cleanStaticDir, "index.html")

	// Serve static assets if the directory exists
	if dirInfo, err := os.Stat(cleanStaticDir); err == nil && dirInfo.IsDir() {
		app.Static("/", cleanStaticDir, fiber.Static{
			Browse:   false,
			Index:    "index.html",
			Compress: true,
			MaxAge:   3600,
		})
	}

	// SPA fallback for all other GET requests (non-API)
	app.Get("/*", func(c *fiber.Ctx) error {
		p := c.Path()

		// Never handle API routes in SPA fallback
		if strings.HasPrefix(p, "/api") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error":   "not_found",
				"message": fmt.Sprintf("API endpoint '%s' not found", p),
			})
		}

		// If a file with an extension was requested (e.g. .js, .css, .png) and wasn't found by static middleware,
		// return 404 instead of serving index.html to avoid MIME mismatch errors.
		ext := filepath.Ext(p)
		if ext != "" && ext != ".html" {
			return c.Status(fiber.StatusNotFound).SendString("File not found")
		}

		// Check if index.html exists
		if _, err := os.Stat(indexPath); err == nil {
			c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
			return c.SendFile(indexPath)
		}

		// When staticDir is not yet built, serve a friendly status
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"message": "Pixora API Server is running. Frontend static assets have not been built or static directory does not exist.",
			"path":    p,
		})
	})
}
