package routes

import (
	"BAP_Sandbox/internal/controllers"

	"github.com/gofiber/fiber/v2"
)

// SetupRoutes configures all application routes
func SetupRoutes(app *fiber.App) {
	// Initialize controllers
	forwardController := controllers.NewForwardController()
	webhookController := controllers.NewWebhookController()

	// Health check endpoint
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"message": "Server is running",
		})
	})

	// Forward all POST requests from /api/* to target service and wait for webhook
	app.Post("/api/*", forwardController.ForwardRequest)

	// Webhook endpoint to receive callbacks
	app.Post("/webhook/*", webhookController.HandleWebhook)
}
