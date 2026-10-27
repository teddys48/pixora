package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pixora/backend/internal/config"
	"pixora/backend/internal/server"
)

func main() {
	cfg := config.Load()
	app := server.New(cfg)

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)

	// Channel to listen for interrupt signals for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// Advisory storage permission check (non-blocking in Phase 1)
	if _, err := os.Stat(cfg.StorageRoot); err == nil {
		if err := config.CheckWritable(cfg.StorageRoot); err != nil {
			log.Printf("[WARN] StorageRoot '%s' exists but is not writable: %v. Please verify host permissions.", cfg.StorageRoot, err)
		} else {
			log.Printf("[INFO] StorageRoot '%s' verified writable.", cfg.StorageRoot)
		}
	} else {
		log.Printf("[INFO] StorageRoot '%s' will be created when needed in future phases.", cfg.StorageRoot)
	}

	go func() {
		log.Printf("Pixora server starting on %s (env: %s, static: %s, storage: %s, db: %s)",
			addr, cfg.Env, cfg.StaticDir, cfg.StorageRoot, cfg.DatabasePath)
		if err := app.Listen(addr); err != nil {
			log.Printf("Server listen error: %v", err)
		}
	}()

	// Wait for termination signal
	sig := <-quit
	log.Printf("Received signal %s. Shutting down gracefully...", sig)

	shutdownTimeout := 5 * time.Second
	if err := app.ShutdownWithTimeout(shutdownTimeout); err != nil {
		log.Printf("Server forced shutdown: %v", err)
	}

	log.Println("Pixora server exited properly.")
}
