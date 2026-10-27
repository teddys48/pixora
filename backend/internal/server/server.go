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
)

// New creates and configures the Fiber application
func New(cfg *config.Config) *fiber.App {
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
		AllowMethods: "GET, POST, HEAD, OPTIONS",
	}))

	// API Routes
	apiGroup := app.Group("/api")
	apiGroup.Get("/health", api.HealthHandler)

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
