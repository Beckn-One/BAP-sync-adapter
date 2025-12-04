package main

import (
	"BAP_Sandbox/config"
	"BAP_Sandbox/internal/routes"
	"BAP_Sandbox/internal/storage"
	"BAP_Sandbox/internal/transformers"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file (ignore error if file doesn't exist - useful for production with real env vars)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Load configuration
	cfg := config.Load()

	// Initialize Redis
	if err := storage.InitRedis(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer storage.CloseRedis()

	log.Println("Successfully connected to Redis")

	// Initialize Transformer
	// Get the path to mappings.yaml relative to the project root
	mappingsPath := filepath.Join("config", "mappings.yaml")
	if err := transformers.InitTransformer(mappingsPath); err != nil {
		log.Printf("WARNING: Failed to initialize transformer: %v", err)
		log.Println("Application will continue without transformation capabilities")
	} else {
		log.Println("Successfully initialized transformer with mappings")
	}

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName: "BAP Sandbox",
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New())

	// Setup routes
	routes.SetupRoutes(app)

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down gracefully...")
		app.Shutdown()
		storage.CloseRedis()
	}()

	// Start server
	log.Printf("Server starting on port %s", cfg.Port)
	log.Fatal(app.Listen(":" + cfg.Port))
}
