package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"db-sync-scheduler/internal/app"
	"db-sync-scheduler/internal/config"
	"db-sync-scheduler/internal/handlers"
	"db-sync-scheduler/internal/middleware"

	configLoader "github.com/andiksetyawan/config"
)

func main() {
	// Load configuration menggunakan github.com/andiksetyawan/config
	log.Println("Loading configuration...")

	cfg := &config.AppConfig{}
	loader := configLoader.New(
		configLoader.WithEnvPath(".env"),
	)

	if err := loader.Load(cfg); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Configuration loaded (Server Port: %s)", cfg.Server.Port)

	// Inisialisasi database
	masterDB, backupDB, err := config.InitDatabase(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer masterDB.Close()
	defer backupDB.Close()

	// Create application instance with dependency injection
	application := app.NewApplication(cfg, masterDB, backupDB)
	defer application.Close()

	// Create handler with dependencies
	handler := handlers.NewHandler(application.SyncService)

	// Setup routes with CORS middleware
	http.HandleFunc("/", middleware.CORS(handler.RootHandler))
	http.HandleFunc("/health", middleware.CORS(handler.HealthHandler))
	http.HandleFunc("/api/sync/start", middleware.CORS(handler.StartSyncHandler))
	http.HandleFunc("/api/sync/stop", middleware.CORS(handler.StopSyncHandler))
	http.HandleFunc("/api/sync/status", middleware.CORS(handler.StatusHandler))
	http.HandleFunc("/api/sync/config", middleware.CORS(handler.ConfigHandler))
	http.HandleFunc("/api/schema/sync", middleware.CORS(handler.SchemaSyncHandler))

	// Get port from config
	port := cfg.Server.Port

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("\nShutting down server...")
		os.Exit(0)
	}()

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
