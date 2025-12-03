package controllers

import (
	"BAP_Sandbox/internal/storage"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// CallbackResponse represents a response waiting to be delivered
type CallbackResponse struct {
	Body       []byte            `json:"body"`
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
}

// RouteMapping maps forward routes to their callback routes
var RouteMapping = map[string]string{
	"discover": "on_discover",
	"search":   "on_search",
	"select":   "on_select",
	"init":     "on_init",
	"confirm":  "on_confirm",
	"update":   "on_update",
	"track":    "on_track",
	"rating":   "on_rating",
	"support":  "on_support",
	"cancel":   "on_cancel",
	"status":   "on_status",
}

// CallbackManager manages pending requests using Redis
type CallbackManager struct{}

// GetCallbackManager returns a callback manager instance
func GetCallbackManager() *CallbackManager {
	return &CallbackManager{}
}

// AddPendingRequest adds a new pending request to Redis
func (cm *CallbackManager) AddPendingRequest(subRoute, transactionID, messageID string) error {
	ctx := storage.GetContext()
	key := cm.makePendingKey(subRoute, transactionID, messageID)

	log.Printf("[Redis] Adding pending request - Route: %s, TransactionID: %s, MessageID: %s", subRoute, transactionID, messageID)
	log.Printf("[Redis] Pending key: %s", key)

	// Store pending request metadata in Redis with 35 second TTL
	metadata := map[string]string{
		"transaction_id": transactionID,
		"message_id":     messageID,
		"created_at":     time.Now().Format(time.RFC3339),
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		log.Printf("[Redis] ERROR: Failed to marshal metadata: %v", err)
		return err
	}

	err = storage.RedisClient.Set(ctx, key, data, 35*time.Second).Err()
	if err != nil {
		log.Printf("[Redis] ERROR: Failed to set key in Redis: %v", err)
		return err
	}

	log.Printf("[Redis] ✓ Successfully added pending request with TTL 35s")
	return nil
}

// WaitForCallback waits for a callback response via Redis pub/sub
func (cm *CallbackManager) WaitForCallback(subRoute, transactionID, messageID string, timeout time.Duration) (*CallbackResponse, error) {
	ctx, cancel := context.WithTimeout(storage.GetContext(), timeout)
	defer cancel()

	// Subscribe to the callback channel
	channel := cm.makeCallbackChannel(subRoute, transactionID, messageID)
	log.Printf("[Redis] Waiting for callback - Route: %s, TransactionID: %s, MessageID: %s", subRoute, transactionID, messageID)
	log.Printf("[Redis] Subscribing to channel: %s (timeout: %v)", channel, timeout)

	pubsub := storage.RedisClient.Subscribe(ctx, channel)
	defer pubsub.Close()

	log.Printf("[Redis] ✓ Subscribed, waiting for message...")

	// Wait for message or timeout
	ch := pubsub.Channel()
	select {
	case msg := <-ch:
		// Received callback response
		log.Printf("[Redis] ✓ Received message on channel")
		var response CallbackResponse
		if err := json.Unmarshal([]byte(msg.Payload), &response); err != nil {
			log.Printf("[Redis] ERROR: Failed to unmarshal callback response: %v", err)
			return nil, err
		}
		log.Printf("[Redis] ✓ Successfully processed callback response")
		return &response, nil

	case <-ctx.Done():
		// Timeout
		log.Printf("[Redis] ERROR: Timeout waiting for callback after %v", timeout)
		return nil, fmt.Errorf("timeout waiting for callback")
	}
}

// PublishCallback publishes a callback response to Redis pub/sub
func (cm *CallbackManager) PublishCallback(subRoute, transactionID, messageID string, response CallbackResponse) error {
	ctx := storage.GetContext()

	log.Printf("[Redis] Publishing callback - Route: %s, TransactionID: %s, MessageID: %s", subRoute, transactionID, messageID)

	// Check if pending request exists
	key := cm.makePendingKey(subRoute, transactionID, messageID)
	log.Printf("[Redis] Checking for pending key: %s", key)

	exists, err := storage.RedisClient.Exists(ctx, key).Result()
	if err != nil {
		log.Printf("[Redis] ERROR: Failed to check if key exists: %v", err)
		return err
	}

	log.Printf("[Redis] Key exists check result: %d (0=not found, 1=found)", exists)

	if exists == 0 {
		log.Printf("[Redis] ERROR: No pending request found for key: %s", key)

		// Debug: List all keys matching pattern
		pattern := "Sync#*"
		keys, _ := storage.RedisClient.Keys(ctx, pattern).Result()
		log.Printf("[Redis] DEBUG: All pending keys in Redis (%d total):", len(keys))
		for _, k := range keys {
			log.Printf("[Redis] DEBUG:   - %s", k)
		}

		return fmt.Errorf("no pending request found")
	}

	log.Printf("[Redis] ✓ Found pending request")

	// Marshal response
	data, err := json.Marshal(response)
	if err != nil {
		log.Printf("[Redis] ERROR: Failed to marshal response: %v", err)
		return err
	}

	// Publish to callback channel
	channel := cm.makeCallbackChannel(subRoute, transactionID, messageID)
	log.Printf("[Redis] Publishing to channel: %s", channel)

	numSubscribers, err := storage.RedisClient.Publish(ctx, channel, data).Result()
	if err != nil {
		log.Printf("[Redis] ERROR: Failed to publish to channel: %v", err)
		return err
	}

	log.Printf("[Redis] ✓ Published to %d subscriber(s)", numSubscribers)

	// Delete pending request from Redis
	err = storage.RedisClient.Del(ctx, key).Err()
	if err != nil {
		log.Printf("[Redis] ERROR: Failed to delete pending key: %v", err)
		return err
	}

	log.Printf("[Redis] ✓ Deleted pending request key")
	return nil
}

// RemovePendingRequest removes a pending request from Redis
func (cm *CallbackManager) RemovePendingRequest(subRoute, transactionID, messageID string) error {
	ctx := storage.GetContext()
	key := cm.makePendingKey(subRoute, transactionID, messageID)
	return storage.RedisClient.Del(ctx, key).Err()
}

// makePendingKey creates a Redis key for pending requests
// Format: Sync#{sub-route}#{message_id}#{transaction_id}
func (cm *CallbackManager) makePendingKey(subRoute, transactionID, messageID string) string {
	return fmt.Sprintf("Sync#%s#%s#%s", subRoute, messageID, transactionID)
}

// makeCallbackChannel creates a Redis pub/sub channel name
// Format: Callback#{sub-route}#{message_id}#{transaction_id}
func (cm *CallbackManager) makeCallbackChannel(subRoute, transactionID, messageID string) string {
	return fmt.Sprintf("Callback#%s#%s#%s", subRoute, messageID, transactionID)
}
