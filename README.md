# BAP Sync Adapter

A lightweight API adapter built with Go and Fiber framework that implements a synchronous wrapper over Beckn protocol APIs. Use this adapter when your BAP application must receive synchronous API responses from Beckn ONIX that is both synchronous (discover) and asynchronous (callback-based) patterns.

This service is designed to still between BAP application and BAP Beckn ONIX.

## Overview

This service implements both **async** and **sync** request-response patterns:

### Async Pattern (Default for most routes)
1. Receives POST requests at `/api/{sub-route}` (e.g., `/api/select`, `/api/init`)
2. Forwards them to target service at `{ONIX_URL}/{sub-route}`
3. Waits for async callback on `/webhook/{on_sub-route}` (e.g., `/webhook/on_select`)
4. Matches request and callback using `transaction_id` and `message_id`
5. Returns the callback response to the original caller
6. Returns timeout error if no callback within 30 seconds

### Sync Pattern (For search and discover calls)
1. Receives POST requests at `/api/search` or `/api/discover`
2. Forwards them synchronously to target service at `{ONIX_URL}/search` or `{ONIX_URL}/discover`
3. Waits for immediate HTTP response from target service
4. Returns the response directly to the caller (no webhook involved)

## Project Structure

```
BAP_Sandbox/
├── cmd/
│   └── server/
│       └── main.go                      # Application entry point
├── internal/
│   ├── controllers/
│   │   ├── callback_manager.go          # Redis-based callback manager
│   │   ├── forward_controller.go        # Request forwarding & waiting logic
│   │   └── webhook_controller.go        # Webhook callback handler
│   ├── storage/
│   │   └── redis_client.go              # Redis connection management
│   └── routes/
│       └── routes.go                    # Route definitions
├── config/
│   └── config.go                        # Configuration loader
├── bin/
│   └── app                              # Compiled binary (11MB)
├── .env                                 # Environment variables (not committed)
├── .env.example                         # Example environment variables
├── go.mod & go.sum                      # Go module dependencies
└── README.md                            # This file
```

## Prerequisites

- Go 1.25.1 or higher
- **Redis 6.0 or higher** (required for distributed state management)
- Git (optional)

## Setup

### 1. Install and Start Redis

**macOS (using Homebrew):**
```bash
brew install redis
brew services start redis
```

**Ubuntu/Debian:**
```bash
sudo apt update
sudo apt install redis-server
sudo systemctl start redis-server
```

**Docker:**
```bash
docker run -d -p 6379:6379 --name redis redis:latest
```

**Verify Redis is running:**
```bash
redis-cli ping
# Should return: PONG
```

### 2. Configure Application

Clone or navigate to the project directory:
```bash
cd BAP_Sandbox
```

Copy the example environment file:
```bash
cp .env.example .env
```

Update the `.env` file with your configuration:
```bash
PORT=3000
APP_ENV=development
ONIX_URL=http://localhost:8080
REDIS_URL=localhost:6379
REDIS_PASSWORD=              # Leave empty if no password
```

Install Go dependencies:
```bash
go mod download
```

## Running the Application

Run from source:
```bash
go run cmd/server/main.go
```

Or use the compiled binary:
```bash
./bin/app
```

The server will start on the port specified in your `.env` file (default: 3000).

## Available Endpoints

### Health Check
- `GET /health` - Health check endpoint
  ```json
  Response: {"status": "ok", "message": "Server is running"}
  ```

### Sync Endpoints (For search and discover)
- `POST /api/search` - Synchronous search endpoint
  - Forwards request to `{ONIX_URL}/search`
  - Returns immediate response from target service
  - No webhook callback involved
  - No timeout (uses standard HTTP timeout)

- `POST /api/discover` - Synchronous discover endpoint
  - Forwards request to `{ONIX_URL}/discover`
  - Returns immediate response from target service
  - No webhook callback involved
  - No timeout (uses standard HTTP timeout)

### Async Forwarding Endpoint (For other routes)
- `POST /api/{sub-route}` - Forwards request and waits for webhook callback
  - Applies to: select, init, confirm, update, track, rating, support, cancel, status
  - Requires `context.transaction_id` and `context.message_id` in request body
  - Forwards to `{ONIX_URL}/{sub-route}`
  - Waits up to 30 seconds for callback on `/webhook/{on_sub-route}`
  - Returns webhook response or timeout error

### Webhook Endpoint
- `POST /webhook/{on_sub-route}` - Receives async callbacks from target service
  - Only for async routes (not search/discover)
  - Matches with pending requests using `transaction_id`, `message_id`, and route mapping
  - Returns ACK/NACK response

## Route Mapping

### Sync Routes (No webhook)
| Forward Route | Response Type |
|--------------|---------------|
| `/api/search` | Synchronous HTTP response |
| `/api/discover` | Synchronous HTTP response |

### Async Routes (Webhook-based)
| Forward Route | Webhook Route  |
|--------------|----------------|
| `/api/select`   | `/webhook/on_select`   |
| `/api/init`     | `/webhook/on_init`     |
| `/api/confirm`  | `/webhook/on_confirm`  |
| `/api/update`   | `/webhook/on_update`   |
| `/api/track`    | `/webhook/on_track`    |
| `/api/rating`   | `/webhook/on_rating`   |
| `/api/support`  | `/webhook/on_support`  |
| `/api/cancel`   | `/webhook/on_cancel`   |
| `/api/status`   | `/webhook/on_status`   |

## Configuration

### Environment Variables

Configure the following in your `.env` file:

- **PORT** - Server port (default: 3000)
- **APP_ENV** - Application environment (development/production)
- **ONIX_URL** - The base URL where requests will be forwarded to
- **REDIS_URL** - Redis server address (default: localhost:6379)
- **REDIS_PASSWORD** - Redis password (leave empty if none)

Example `.env`:
```bash
PORT=3000
APP_ENV=development
ONIX_URL=http://localhost:8080
REDIS_URL=localhost:6379
REDIS_PASSWORD=
```

### Redis Configuration for Production

**AWS ElastiCache:**
```bash
REDIS_URL=your-cluster.cache.amazonaws.com:6379
REDIS_PASSWORD=your-password
```

**Redis Cloud:**
```bash
REDIS_URL=redis-12345.c1.us-east-1-2.ec2.cloud.redislabs.com:12345
REDIS_PASSWORD=your-password
```

**Kubernetes (Redis Service):**
```bash
REDIS_URL=redis-service:6379
REDIS_PASSWORD=your-password
```

## Usage Example

### Sync Flow (Search/Discover)

**Request to BAP Sandbox (returns immediately):**
```bash
curl -X POST http://localhost:3000/api/search \
  -H "Content-Type: application/json" \
  -d '{
    "context": {
      "domain": "retail",
      "action": "search",
      "location": {
        "country": {
          "name": "India",
          "code": "IND"
        }
      }
    },
    "message": {
      "intent": {
        "item": {
          "descriptor": {
            "name": "laptop"
          }
        }
      }
    }
  }'
```

**BAP Sandbox forwards to target service and returns immediate response:**
```json
{
  "context": {
    "domain": "retail",
    "action": "on_search"
  },
  "message": {
    "catalog": {
      "items": [
        {
          "id": "item-1",
          "descriptor": {
            "name": "Dell Laptop"
          },
          "price": {
            "value": "50000",
            "currency": "INR"
          }
        }
      ]
    }
  }
}
```

### Async Flow (Select/Init/Confirm/etc.)

**1. Client sends request to BAP Sandbox:**
```bash
curl -X POST http://localhost:3000/api/select \
  -H "Content-Type: application/json" \
  -d '{
    "context": {
      "transaction_id": "txn-12345",
      "message_id": "msg-67890"
    },
    "message": {
      "order": {
        "items": [
          {
            "id": "item-1",
            "quantity": {
              "count": 1
            }
          }
        ]
      }
    }
  }'
```

**2. BAP Sandbox forwards to target service:**
```
POST http://localhost:8080/select
Body: (same as above)
```

**3. Target service later sends callback:**
```bash
curl -X POST http://localhost:3000/webhook/on_select \
  -H "Content-Type: application/json" \
  -d '{
    "context": {
      "transaction_id": "txn-12345",
      "message_id": "msg-67890"
    },
    "message": {
      "order": {
        "items": [...],
        "quote": {
          "price": {
            "value": "50000",
            "currency": "INR"
          }
        }
      }
    }
  }'
```

**4. BAP Sandbox returns callback response to original client**

### Timeout Example

If no callback is received within 30 seconds:
```json
{
  "message": {
    "ack": {
      "status": "NACK"
    }
  },
  "error": {
    "type": "TIMEOUT",
    "code": "REQUEST_TIMEOUT",
    "message": "No response received within 30 seconds"
  }
}
```

## How It Works

### Sync Routes (Search/Discover)
1. **Client Request** - Sends POST to `/api/search` or `/api/discover`
2. **Forward Request** - Gateway forwards request to `{ONIX_URL}/search` or `{ONIX_URL}/discover`
3. **Wait for Response** - Gateway waits for synchronous HTTP response
4. **Return Response** - Gateway returns response directly to client

### Async Routes (Redis-Based)

1. **Client Request** - Sends POST to `/api/{sub-route}` with `context.transaction_id` and `context.message_id`
2. **Register in Redis** - Gateway stores pending request metadata in Redis with 35s TTL
3. **Subscribe to Pub/Sub** - Gateway subscribes to Redis channel `callback:{txn_id}:{msg_id}`
4. **Forward Request** - Gateway forwards request to `{ONIX_URL}/{sub-route}` asynchronously
5. **Wait for Callback** - Gateway waits for Redis pub/sub message or 30s timeout
6. **Webhook Arrives** - Target service calls `/webhook/{on_sub-route}`
7. **Validate & Publish** - Gateway validates route mapping and IDs, publishes to Redis pub/sub
8. **Deliver Response** - Waiting request receives pub/sub message, returns callback to client
9. **Cleanup** - Redis auto-expires pending requests after 35s (TTL)

### Redis Data Flow

```
[Request]                              [Redis]                         [Webhook]
   │                                      │                                │
   ├─ POST /api/discover ────────────────>│                                │
   │  {txn:123, msg:456}                  │                                │
   │                                      │                                │
   │  ┌─ SET pending:123:456 (TTL 35s)   │                                │
   │  └─ SUBSCRIBE callback:123:456      │                                │
   │                                      │                                │
   │  [Waiting on pub/sub...]            │                                │
   │                                      │<───── POST /webhook/on_discover
   │                                      │       {txn:123, msg:456}       │
   │                                      │                                │
   │                                 ┌─ EXISTS pending:123:456?           │
   │                                 └─ PUBLISH callback:123:456          │
   │                                      │                                │
   │<─ [Pub/sub message received] ────────┤                                │
   │  Response delivered!                 │                                │
   │                                 ┌─ DEL pending:123:456                │
```

## Building

To build the application:

```bash
go build -o bin/app cmd/server/main.go
```

To run the built binary:

```bash
./bin/app
```

## Dependencies

- [Fiber v2](https://github.com/gofiber/fiber) - Fast HTTP web framework

## Features

- **Dual Request Patterns**: Supports both sync (search/discover) and async (other routes) request patterns
- **Synchronous Endpoints**: Direct HTTP response for search and discover operations
- **Distributed State Management**: Uses Redis for cross-instance state sharing (async routes)
- **Horizontal Scalability**: Deploy multiple instances behind load balancer (Nginx/K8s)
- **Async Request-Response Pattern**: Implements Beckn protocol's callback mechanism for most routes
- **Real-time Pub/Sub**: Redis pub/sub for instant callback delivery (async routes)
- **Request Matching**: Matches callbacks using transaction_id, message_id, and route mapping
- **Timeout Handling**: 30-second timeout with NACK response for async routes
- **Automatic TTL Cleanup**: Redis auto-expires pending requests after 35 seconds
- **Concurrent Request Handling**: Multiple requests can wait for callbacks simultaneously
- **Health Monitoring**: Health check endpoint for uptime monitoring
- **CORS Support**: Built-in CORS middleware
- **Request Logging**: Automatic request/response logging
- **Panic Recovery**: Automatic recovery from panics
- **Graceful Shutdown**: Properly closes Redis connections on shutdown

## Deployment Architecture

### Single Instance (Development)
```
Client → BAP Sandbox (Redis) → Target Service
```

### Multi-Instance (Production with Nginx/K8s)
```
                    ┌─→ Instance 1 ─┐
Client → Nginx/LB ──┼─→ Instance 2 ─┼─→ Redis ←─→ Target Service
                    └─→ Instance 3 ─┘
```

**Benefits of Redis:**
- Request on Instance 1, callback to Instance 2 → Works!
- No sticky sessions required
- Fault-tolerant (instances can restart without data loss)
- Auto-cleanup with TTL (no memory leaks)

### Kubernetes Deployment Example

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bap-sandbox
spec:
  replicas: 3  # Multiple instances
  selector:
    matchLabels:
      app: bap-sandbox
  template:
    metadata:
      labels:
        app: bap-sandbox
    spec:
      containers:
      - name: bap-sandbox
        image: your-registry/bap-sandbox:latest
        env:
        - name: REDIS_URL
          value: "redis-service:6379"
        - name: ONIX_URL
          value: "http://target-service:8080"
        ports:
        - containerPort: 3000
---
apiVersion: v1
kind: Service
metadata:
  name: bap-sandbox
spec:
  type: LoadBalancer
  ports:
  - port: 80
    targetPort: 3000
  selector:
    app: bap-sandbox
```

## Key Components

- **ForwardController** (`forward_controller.go:45-109`): Forwards requests and waits for Redis pub/sub
- **WebhookController** (`webhook_controller.go:18-103`): Receives callbacks and publishes to Redis
- **CallbackManager** (`callback_manager.go:36-133`): Manages Redis-based pending requests and pub/sub
- **RedisClient** (`redis_client.go:14-49`): Redis connection management
- **Route Mapping** (`callback_manager.go:21-31`): Maps forward routes to callback routes
