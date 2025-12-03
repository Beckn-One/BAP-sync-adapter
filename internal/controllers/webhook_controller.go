package controllers

import (
	"encoding/json"
	"log"

	"github.com/gofiber/fiber/v2"
)

// WebhookController handles incoming webhook callbacks
type WebhookController struct{}

// NewWebhookController creates a new webhook controller
func NewWebhookController() *WebhookController {
	return &WebhookController{}
}

// HandleWebhook processes incoming webhook callbacks
func (wc *WebhookController) HandleWebhook(c *fiber.Ctx) error {
	// Get the sub-route from params
	subRoute := c.Params("*")
	if subRoute == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "sub-route is required",
		})
	}

	log.Printf("[Webhook] ========== CALLBACK RECEIVED ==========")
	log.Printf("[Webhook] Callback route: %s", subRoute)

	// Read the request body
	body := c.Body()

	// Parse request to extract context
	var reqContext RequestContext
	if err := json.Unmarshal(body, &reqContext); err != nil {
		log.Printf("[Webhook] ERROR: Invalid JSON body: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid JSON body",
		})
	}

	transactionID := reqContext.Context.TransactionID
	messageID := reqContext.Context.MessageID

	log.Printf("[Webhook] TransactionID: %s", transactionID)
	log.Printf("[Webhook] MessageID: %s", messageID)

	if transactionID == "" || messageID == "" {
		log.Printf("[Webhook] ERROR: Missing transaction_id or message_id")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "context.transaction_id and context.message_id are required",
		})
	}

	// Validate that this is a valid callback route and get corresponding forward route
	var forwardRoute string
	isValidCallback := false
	for fwdRoute, callbackRoute := range RouteMapping {
		if callbackRoute == subRoute {
			isValidCallback = true
			forwardRoute = fwdRoute
			break
		}
	}

	log.Printf("[Webhook] Callback route '%s' mapped to forward route: '%s' (valid: %v)", subRoute, forwardRoute, isValidCallback)

	if !isValidCallback {
		log.Printf("[Webhook] ERROR: Invalid callback route: %s", subRoute)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid callback route: " + subRoute,
		})
	}

	// Prepare the callback response
	headers := make(map[string]string)
	c.Request().Header.VisitAll(func(key, value []byte) {
		keyStr := string(key)
		if keyStr != "Host" {
			headers[keyStr] = string(value)
		}
	})

	callbackResponse := CallbackResponse{
		Body:       body,
		StatusCode: fiber.StatusOK,
		Headers:    headers,
	}

	// Publish callback to Redis pub/sub using the forward route name
	log.Printf("[Webhook] Publishing callback to Redis...")
	callbackManager := GetCallbackManager()
	err := callbackManager.PublishCallback(forwardRoute, transactionID, messageID, callbackResponse)

	if err == nil {
		// Successfully published to waiting request
		log.Printf("[Webhook] ✓ Successfully published callback, returning ACK")
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"message": fiber.Map{
				"ack": fiber.Map{
					"status": "ACK",
				},
			},
		})
	}

	// No pending request found - might have timed out or doesn't exist
	log.Printf("[Webhook] ERROR: Failed to publish callback: %v", err)
	log.Printf("[Webhook] Returning NACK to caller")
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
		"message": fiber.Map{
			"ack": fiber.Map{
				"status": "NACK",
			},
		},
		"error": fiber.Map{
			"message": "No pending request found for this transaction",
		},
	})
}
