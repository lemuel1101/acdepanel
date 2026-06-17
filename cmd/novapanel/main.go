package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/novapanel/novapanel/internal/api"
	"github.com/novapanel/novapanel/internal/config"
	"github.com/novapanel/novapanel/internal/db"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting NovaPanel Server...")

	configPath := "/etc/novapanel/config.yaml"
	if envPath := os.Getenv("NOVAPANEL_CONFIG"); envPath != "" {
		configPath = envPath
	}

	var cfg *config.Config
	if _, err := os.Stat(configPath); err == nil {
		cfg, err = config.LoadConfig(configPath)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
	} else {
		cfg = config.DefaultConfig()
		log.Println("No config found, using defaults")
	}

	if err := os.MkdirAll(cfg.System.DataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	if err := os.MkdirAll(cfg.System.LogsDir, 0755); err != nil {
		log.Fatalf("Failed to create logs directory: %v", err)
	}

	if err := db.InitDatabase(cfg); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	if err := db.RunMigrations(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	router := api.SetupRouter(cfg)

	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: router,
	}

	go func() {
		log.Printf("NovaPanel API listening on %s:%d", cfg.Server.Host, cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
}
