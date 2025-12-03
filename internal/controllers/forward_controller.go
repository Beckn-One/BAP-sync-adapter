package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
)

// ForwardController handles forwarding requests to another service
type ForwardController struct {
	targetURL  string
	httpClient *http.Client
}

// NewForwardController creates a new forward controller
func NewForwardController() *ForwardController {
	targetURL := os.Getenv("ONIX_URL")
	if targetURL == "" {
		targetURL = "http://localhost:8080" // Default fallback
	}

	return &ForwardController{
		targetURL: targetURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// RequestContext represents the context from the request body
type RequestContext struct {
	Context struct {
		TransactionID string `json:"transaction_id"`
		MessageID     string `json:"message_id"`
	} `json:"context"`
}

// isSyncRoute checks if the route should use synchronous forwarding
func (fc *ForwardController) isSyncRoute(subRoute string) bool {
	return subRoute == "search" || subRoute == "discover"
}

// ForwardRequest forwards the incoming request to the target service and waits for callback
func (fc *ForwardController) ForwardRequest(c *fiber.Ctx) error {
	// Get the sub-route from params
	subRoute := c.Params("*")
	if subRoute == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "sub-route is required",
		})
	}

	log.Printf("[Forward] ========== NEW REQUEST ==========")
	log.Printf("[Forward] Received request for route: %s", subRoute)

	// Read the request body
	body := c.Body()

	// Parse request to extract context
	var reqContext RequestContext
	if err := json.Unmarshal(body, &reqContext); err != nil {
		log.Printf("[Forward] ERROR: Invalid JSON body: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid JSON body",
		})
	}

	transactionID := reqContext.Context.TransactionID
	messageID := reqContext.Context.MessageID

	log.Printf("[Forward] TransactionID: %s", transactionID)
	log.Printf("[Forward] MessageID: %s", messageID)

	if transactionID == "" || messageID == "" {
		log.Printf("[Forward] ERROR: Missing transaction_id or message_id")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "context.transaction_id and context.message_id are required",
		})
	}

	// Check if this is a synchronous route (search/discover)
	if fc.isSyncRoute(subRoute) {
		log.Printf("[Forward] Route '%s' uses synchronous forwarding", subRoute)
		return fc.forwardRequestSync(c, subRoute, body)
	}

	// For other routes, use the async webhook-based mechanism
	log.Printf("[Forward] Route '%s' uses async webhook-based forwarding", subRoute)

	// Register pending request in Redis
	log.Printf("[Forward] Registering pending request in Redis...")
	callbackManager := GetCallbackManager()
	if err := callbackManager.AddPendingRequest(subRoute, transactionID, messageID); err != nil {
		log.Printf("[Forward] ERROR: Failed to register pending request: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to register pending request",
		})
	}
	defer func() {
		log.Printf("[Forward] Cleaning up pending request from Redis")
		callbackManager.RemovePendingRequest(subRoute, transactionID, messageID)
	}()

	// Forward the request asynchronously
	log.Printf("[Forward] Forwarding request to: %s/%s", fc.targetURL, subRoute)
	go fc.forwardRequestAsync(subRoute, body, c.GetReqHeaders())

	// Wait for callback response via Redis pub/sub or timeout
	log.Printf("[Forward] Waiting for callback response (30s timeout)...")
	response, err := callbackManager.WaitForCallback(subRoute, transactionID, messageID, 30*time.Second)
	if err != nil {
		// Timeout - return static response
		log.Printf("[Forward] ERROR: Request timed out after 30 seconds")
		return c.Status(fiber.StatusRequestTimeout).JSON(fiber.Map{
			"message": fiber.Map{
				"ack": fiber.Map{
					"status": "NACK",
				},
			},
			"error": fiber.Map{
				"type":    "TIMEOUT",
				"code":    "REQUEST_TIMEOUT",
				"message": "No response received within 30 seconds",
			},
		})
	}

	// Received callback response
	log.Printf("[Forward] ✓ Received callback response, returning to client")
	for key, value := range response.Headers {
		c.Set(key, value)
	}
	return c.Status(response.StatusCode).Send(response.Body)
}

// forwardRequestSync forwards the request synchronously and returns the direct response
func (fc *ForwardController) forwardRequestSync(c *fiber.Ctx, subRoute string, body []byte) error {
	// Construct the target URL
	targetURL := fmt.Sprintf("%s/%s", fc.targetURL, subRoute)
	log.Printf("[Forward] Making synchronous request to: %s", targetURL)

	// Create a new request
	req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("[Forward] ERROR: Failed to create request: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create request",
		})
	}

	// Copy headers from original request
	c.Request().Header.VisitAll(func(key, value []byte) {
		keyStr := string(key)
		if keyStr != "Host" {
			req.Header.Add(keyStr, string(value))
		}
	})

	// Ensure Content-Type is set
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Make the synchronous request
	resp, err := fc.httpClient.Do(req)
	if err != nil {
		log.Printf("[Forward] ERROR: Request failed: %v", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "Failed to forward request to ONIX service",
		})
	}
	defer resp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[Forward] ERROR: Failed to read response body: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to read response",
		})
	}

	// Copy response headers
	for key, values := range resp.Header {
		if key != "Host" {
			for _, value := range values {
				c.Set(key, value)
			}
		}
	}

	log.Printf("[Forward] ✓ Received synchronous response (status: %d), returning to client", resp.StatusCode)
	return c.Status(resp.StatusCode).Send(respBody)
}

// forwardRequestAsync forwards the request to the target service asynchronously
func (fc *ForwardController) forwardRequestAsync(subRoute string, body []byte, headers map[string][]string) {
	// Construct the target URL
	targetURL := fmt.Sprintf("%s/%s", fc.targetURL, subRoute)

	// Create a new request
	req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewBuffer(body))
	if err != nil {
		return
	}

	// Copy headers from original request
	for key, values := range headers {
		if key != "Host" {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}

	// Ensure Content-Type is set
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Make the request (ignore errors in async mode)
	resp, err := fc.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// Read and discard the response body
	io.ReadAll(resp.Body)
}
