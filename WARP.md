# WARP.md

This file provides guidance to WARP (warp.dev) when working with code in this repository.

## Project Overview

This is a real-time distributed ride-hailing platform built using Service-Oriented Architecture (SOA) principles in Go. The system implements three microservices that communicate through PostgreSQL and RabbitMQ to handle ride requests, driver matching, real-time location tracking, and ride coordination.

**Technology Stack:**
- Language: Go 1.25.3
- Database: PostgreSQL with pgx/v5 driver
- Message Broker: RabbitMQ (github.com/rabbitmq/amqp091-go)
- WebSocket: Gorilla WebSocket (github.com/gorilla/websocket)
- Authentication: JWT (github.com/golang-jwt/jwt/v5)

**Package Restrictions:** Only the above packages are allowed. Do not use any other external dependencies.

## Build & Run Commands

### Build
```powershell
# Build the main system binary
go build -o ride-hail-system .

# Build individual service (example: driver location service)
go build -o driver_location_service.exe ./cmd/driver_location
```

### Code Formatting
All code MUST be formatted with gofumpt before committing:
```powershell
# Install gofumpt
go install mvdan.cc/gofumpt@latest

# Format all code
gofumpt -l -w .
```

### Testing
Check the project README for specific test commands. There is no standard test runner configured yet.

### Linting
No specific linting commands are currently configured. Follow gofumpt formatting requirements strictly.

## Architecture

### Service Structure
The codebase follows a three-tier microservices architecture:

1. **Ride Service** - Orchestrates the complete ride lifecycle and manages passenger interactions
2. **Driver & Location Service** (`services/driver_location_service/`) - Handles driver operations, matching algorithms, and real-time location tracking
3. **Admin Service** - Provides monitoring, analytics, and system oversight capabilities

### Communication Patterns

**Message Queue Topology:**
- **ride_topic** (Topic Exchange) - Ride-related messages with routing keys like `ride.request.*` and `ride.status.*`
- **driver_topic** (Topic Exchange) - Driver-related messages with routing keys like `driver.response.*` and `driver.status.*`
- **location_fanout** (Fanout Exchange) - Broadcasts real-time location updates to all interested services

**Key Queues:**
- `ride_requests` - New ride requests
- `ride_status` - Ride status updates
- `driver_matching` - Driver matching requests
- `driver_responses` - Driver acceptance/rejection responses
- `driver_status` - Driver status changes
- `location_updates_ride` - Location updates for ride service

**WebSocket Endpoints:**
- Passengers: `ws://{host}/ws/passengers/{passenger_id}`
- Drivers: `ws://{host}/ws/drivers/{driver_id}`

WebSocket connections require JWT authentication within 5 seconds of connection via an `auth` message type.

### Code Organization

**Standard Project Layout:**
```
cmd/                    # Main entry points for services
  driver_location/      # Driver location service binary
services/               # Service implementations
  driver_location_service/
    internal/
      adapter/          # External integrations (DB, RabbitMQ, REST, WebSocket)
      app/              # Application/business logic layer
      domain/           # Domain models and port interfaces
pkg/                    # Shared packages across services
  auth/                 # JWT authentication utilities
  config/               # Configuration loading from .env
  db/                   # PostgreSQL connection with retry logic
  logger/               # Structured JSON logging
  rabbitmq/             # RabbitMQ connection wrapper with auto-reconnection
  websocket/            # WebSocket utilities
migrations/             # SQL migration files
  ride_service.sql
  driver_location_service.sql
```

**Architectural Patterns:**
- **Hexagonal Architecture** - Services use adapter/port pattern with `internal/adapter`, `internal/app`, and `internal/domain` structure
- **Event Sourcing** - All ride events are tracked in `ride_events` table
- **Event-Driven** - Services communicate via RabbitMQ message queues
- **Shared Database** - Multiple services share the same PostgreSQL database for consistency

### Database Schema

**Core Tables:**
- `users` - All system users (passengers, drivers, admins)
- `roles` - User role enumeration (PASSENGER, DRIVER, ADMIN)
- `rides` - Main rides table with lifecycle tracking
- `coordinates` - Real-time location tracking with `is_current` flag
- `drivers` - Driver-specific data including vehicle info and status
- `ride_events` - Event sourcing audit trail with JSONB event data
- `location_history` - Historical location data for analytics

**Key Design Patterns:**
- Enumeration tables (e.g., `ride_status`, `vehicle_type`, `driver_status`) used as references for data integrity
- JSONB columns for flexible attributes (`users.attrs`, `drivers.vehicle_attrs`, `ride_events.event_data`)
- `is_current` pattern in coordinates table for efficient current location queries
- Timestamps for all lifecycle events (`requested_at`, `matched_at`, `started_at`, etc.)

### Shared Packages

**pkg/logger** - Structured JSON logging with mandatory fields:
- `timestamp` (ISO 8601)
- `level` (INFO, DEBUG, ERROR)
- `service` (service name)
- `action` (event name)
- `message` (human-readable)
- `hostname`
- `request_id` (optional, for correlation)
- `ride_id` (optional, when applicable)

Use `logger.WithFields()` to add context fields like `ride_id` or `request_id`.

**pkg/config** - Environment configuration loader:
- Loads from `.env` file
- Supports defaults for all configuration values
- Database, RabbitMQ, WebSocket, and service port configuration

**pkg/db** - PostgreSQL connection:
- Uses pgxpool for connection pooling
- Implements retry logic (5 attempts with 3-second intervals)
- Auto-reconnection not implemented; handle connection errors gracefully

**pkg/rabbitmq** - RabbitMQ wrapper:
- Automatic reconnection with exponential backoff
- `SetupTopology()` declares all exchanges, queues, and bindings
- `Publish()` for publishing messages (goroutine-safe)
- `Consume()` for consuming with auto-reconnect consumers
- Dedicated publisher channel for thread-safe publishing

**pkg/auth** - JWT authentication:
- `GenerateToken()` creates tokens with user_id and role
- `ParseToken()` validates and extracts claims
- `AuthMiddleware()` HTTP middleware for protected endpoints
- `GetClaims()` extracts claims from context

## Development Guidelines

### Logging Requirements
All services must log structured JSON to stdout with the mandatory fields listed above. For ERROR logs, include an `error` object with `msg` and `stack` fields.

### RabbitMQ Patterns
- Always handle reconnection scenarios
- Use correlation IDs for request tracing across services
- Manual acknowledgment pattern (auto-ack=false) for reliable message processing
- Implement graceful shutdown by closing connections properly

### WebSocket Patterns
- Enforce 5-second authentication timeout
- Send ping every 30 seconds
- Close connection if no pong within 60 seconds
- Use JSON message format with `type` field for message discrimination

### Database Patterns
- Use transactions for multi-step operations
- Handle connection errors gracefully (no auto-reconnect)
- Use PostGIS for geospatial queries (ST_Distance, ST_DWithin)
- Archive historical data (e.g., coordinates to location_history)

### Configuration
Services load configuration from `.env` file with environment variable fallbacks. Configuration structure matches the YAML format specified in `task/README.md`.

### Error Handling
- Never panic in production code
- Services must not crash unexpectedly (handle all nil-pointer, index-out-of-range cases)
- Implement graceful shutdown for all services
- Return structured JSON error responses in HTTP handlers

### Fare Calculation
Dynamic pricing based on vehicle type:
- **ECONOMY:** 500₸ base + 100₸/km + 50₸/min
- **PREMIUM:** 800₸ base + 120₸/km + 60₸/min
- **XL:** 1000₸ base + 150₸/km + 75₸/min

### Security
- JWT tokens required for all API endpoints (except health checks)
- Role-based access control (PASSENGER, DRIVER, ADMIN)
- Validate coordinate ranges (-90 to 90 lat, -180 to 180 lng)
- Sanitize logs (no passwords, tokens, full phone numbers)

## Service-Specific Notes

### Driver Location Service Structure
Located at `services/driver_location_service/`:
- Entry point: `cmd/driver_location/main.go`
- Follows hexagonal architecture with adapter/app/domain layers
- Implements WebSocket handler for driver connections
- REST API for driver status updates and ride lifecycle
- RabbitMQ consumer for ride requests and publisher for responses
- Location tracking with rate limiting (max 1 update per 3 seconds)

### Driver Matching Algorithm
Uses PostGIS for geospatial queries:
- Query available drivers within 5km radius
- Order by distance and rating
- Limit to top 10 candidates
- Send ride offers via WebSocket with 30-second timeout
- Handle driver responses and publish to driver_topic

## Common Pitfalls

1. **Forgetting gofumpt** - All code must be formatted with gofumpt, not standard `go fmt`
2. **Package restrictions** - Only pgx/v5, amqp091-go, gorilla/websocket, and jwt/v5 are allowed
3. **Missing correlation IDs** - Always include correlation_id in message payloads for tracing
4. **Database transactions** - Use transactions for multi-step operations (e.g., creating ride + coordinates)
5. **WebSocket authentication** - Must send auth message within 5 seconds of connection
6. **Location updates** - Remember to set previous coordinate's `is_current=false` when inserting new one
7. **RabbitMQ topology** - Must call `SetupTopology()` after connection establishment
8. **Manual ACK** - RabbitMQ consumers use manual acknowledgment (auto-ack=false)

## Testing Approach

The project uses PostgreSQL and RabbitMQ, which must be running locally for testing:
- PostgreSQL: localhost:5432 (default)
- RabbitMQ: localhost:5672 (default)
- RabbitMQ Management UI: http://localhost:15672

Run migrations manually using psql or a migration tool before testing:
```powershell
# Example: Apply migrations
psql -U ridehail_user -d ridehail_db -f migrations/ride_service.sql
psql -U ridehail_user -d ridehail_db -f migrations/driver_location_service.sql
```

## Additional Resources

Refer to `task/README.md` for:
- Detailed API specifications
- Message format examples
- Request flow diagrams
- Complete database schema with comments
- Security considerations
- Implementation phases
