package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pixora/backend/internal/config"
	"pixora/backend/internal/database"
	"pixora/backend/internal/repository"
	"pixora/backend/internal/server"
	"pixora/backend/internal/storage"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("[FATAL] Configuration validation error: %v", err)
	}

	// Initialize Storage abstraction (safe path resolution, no startup write tests)
	store, err := storage.New(cfg.StorageRoot)
	if err != nil {
		log.Fatalf("[FATAL] Storage initialization failed: %v", err)
	}

	// Non-blocking read inspection: backend tolerates read-only or absent media root
	if err := store.CheckReadAccess(); err != nil {
		log.Printf("[INFO] StorageRoot '%s' not yet accessible for reading: %v. Ready for future mounting.", cfg.StorageRoot, err)
	} else {
		log.Printf("[INFO] StorageRoot '%s' accessible.", cfg.StorageRoot)
	}

	// Initialize SQLite Database and run schema migrations
	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("[FATAL] Database initialization failed at '%s': %v", cfg.DatabasePath, err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("[ERROR] Database close error during shutdown: %v", err)
		} else {
			log.Println("[INFO] Database connection closed cleanly.")
		}
	}()

	// Initialize Repositories
	photoRepo := repository.NewSQLitePhotoRepository(db)
	albumRepo := repository.NewSQLiteAlbumRepository(db)

	app := server.NewWithDependencies(server.Dependencies{
		Config:    cfg,
		DB:        db,
		Storage:   store,
		PhotoRepo: photoRepo,
		AlbumRepo: albumRepo,
	})

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)

	// Channel to listen for interrupt signals for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[INFO] Pixora server starting on %s (env: %s, static: %s, storage: %s, db: %s)",
			addr, cfg.Env, cfg.StaticDir, cfg.StorageRoot, cfg.DatabasePath)
		if err := app.Listen(addr); err != nil {
			log.Printf("[INFO] Server listen stopped: %v", err)
		}
	}()

	// Wait for termination signal
	sig := <-quit
	log.Printf("[INFO] Received signal %s. Shutting down gracefully...", sig)

	shutdownTimeout := 5 * time.Second
	if err := app.ShutdownWithTimeout(shutdownTimeout); err != nil {
		log.Printf("[WARN] Server forced shutdown: %v", err)
	}

	log.Println("[INFO] Pixora server exited properly.")
}
