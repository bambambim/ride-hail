# Driver Location Service - API Documentation

## Overview

The Driver Location Service manages driver availability, location tracking, and driver-ride matching for the ride-hail platform. It provides REST APIs for driver operations and integrates with RabbitMQ for asynchronous messaging and WebSocket for real-time communication.

## Table of Contents

- [REST API Endpoints](#rest-api-endpoints)
- [WebSocket API](#websocket-api)
- [Message Queue Integration](#message-queue-integration)
- [Authentication](#authentication)
- [Error Handling](#error-handling)
- [Rate Limiting](#rate-limiting)

---

## REST API Endpoints

### Base URL

```
http://localhost:8082
```

### Authentication

All driver endpoints require JWT authentication via the `Authorization` header:

```
Authorization: Bearer {driver_token}
```

---

### 1. Health Check

Check service health status.

**Endpoint:** `GET /health`

**Authentication:** Not required

**Response (200 OK):**
```json
{
  "status": "healthy",
  "service": "driver-location-service",
  "time": "2024-12-16T10:00:00Z"
}
```

---

### 2. Go Online

Set driver status to available and start a session.

**Endpoint:** `POST /drivers/{driver_id}/online`

**Authentication:** Required

**Headers:**
```
Content-Type: application/json
Authorization: Bearer {driver_token}
```

**Request Body:**
```json
{
  "latitude": 43.238949,
  "longitude": 76.889709
}
```

**Validation:**
- `latitude`: Required, must be between -90 and 90
- `longitude`: Required, must be between -180 and 180

**Response (200 OK):**
```json
{
  "status": "AVAILABLE",
  "session_id": "660e8400-e29b-41d4-a716-446655440001",
  "message": "You are now online and ready to accept rides"
}
```

**Error Responses:**

- `400 Bad Request`: Invalid coordinates
```json
{
  "error": "Latitude must be between -90 and 90"
}
```

- `401 Unauthorized`: Missing or invalid token
```json
{
  "error": "Unauthorized"
}
```

- `500 Internal Server Error`: Server error
```json
{
  "error": "Failed to create session"
}
```

---

### 3. Go Offline

Set driver status to offline and end the current session.

**Endpoint:** `POST /drivers/{driver_id}/offline`

**Authentication:** Required

**Headers:**
```
Content-Type: application/json
Authorization: Bearer {driver_token}
```

**Request Body:** None

**Response (200 OK):**
```json
{
  "status": "OFFLINE",
  "session_id": "660e8400-e29b-41d4-a716-446655440001",
  "session_summary": {
    "duration_hours": 5.5,
    "rides_completed": 12,
    "earnings": 18500.0
  },
  "message": "You are now offline"
}
```

**Error Responses:**

- `401 Unauthorized`: Missing or invalid token
- `404 Not Found`: No active session found
- `500 Internal Server Error`: Server error

---

### 4. Update Location

Update driver's current location in real-time.

**Endpoint:** `POST /drivers/{driver_id}/location`

**Authentication:** Required

**Headers:**
```
Content-Type: application/json
Authorization: Bearer {driver_token}
```

**Request Body:**
```json
{
  "latitude": 43.238949,
  "longitude": 76.889709,
  "accuracy_meters": 5.0,
  "speed_kmh": 45.0,
  "heading_degrees": 180.0
}
```

**Request Parameters:**
- `latitude` (required): Current latitude (-90 to 90)
- `longitude` (required): Current longitude (-180 to 180)
- `accuracy_meters` (optional): GPS accuracy in meters
- `speed_kmh` (optional): Current speed in km/h
- `heading_degrees` (optional): Heading direction (0-360)

**Response (200 OK):**
```json
{
  "coordinate_id": "770e8400-e29b-41d4-a716-446655440002",
  "updated_at": "2024-12-16T10:30:00Z"
}
```

**Rate Limiting:**
- Maximum 1 update per 3 seconds per driver
- Exceeding this limit returns `429 Too Many Requests`

**Error Responses:**

- `400 Bad Request`: Invalid request data
```json
{
  "error": "Invalid latitude"
}
```

- `401 Unauthorized`: Missing or invalid token
- `429 Too Many Requests`: Rate limit exceeded
```json
{
  "error": "rate limit exceeded: maximum 1 update per 3 seconds"
}
```

- `500 Internal Server Error`: Server error

**Notes:**
- Location updates are automatically archived to `location_history`
- Updates are broadcast via RabbitMQ fanout exchange for real-time tracking
- Previous coordinates are marked as `is_current=false`

---

### 5. Start Ride

Mark driver as busy and start tracking a ride.

**Endpoint:** `POST /drivers/{driver_id}/start`

**Authentication:** Required

**Headers:**
```
Content-Type: application/json
Authorization: Bearer {driver_token}
```

**Request Body:**
```json
{
  "ride_id": "550e8400-e29b-41d4-a716-446655440000",
  "driver_location": {
    "latitude": 43.238949,
    "longitude": 76.889709
  }
}
```

**Request Parameters:**
- `ride_id` (required): UUID of the ride to start
- `driver_location` (required): Current driver location
  - `latitude` (required): -90 to 90
  - `longitude` (required): -180 to 180

**Response (200 OK):**
```json
{
  "ride_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "BUSY",
  "started_at": "2024-12-16T10:35:00Z",
  "message": "Ride started successfully"
}
```

**Error Responses:**

- `400 Bad Request`: Invalid request or driver not in correct state
```json
{
  "error": "driver must be available or en route to start a ride"
}
```

- `401 Unauthorized`: Missing or invalid token
- `500 Internal Server Error`: Server error

**Notes:**
- Driver status changes from `AVAILABLE` or `EN_ROUTE` to `BUSY`
- Location is updated and linked to the ride
- Status update is published via RabbitMQ

---

### 6. Complete Ride

Mark ride as complete and update driver statistics.

**Endpoint:** `POST /drivers/{driver_id}/complete`

**Authentication:** Required

**Headers:**
```
Content-Type: application/json
Authorization: Bearer {driver_token}
```

**Request Body:**
```json
{
  "ride_id": "550e8400-e29b-41d4-a716-446655440000",
  "final_location": {
    "latitude": 43.222015,
    "longitude": 76.851511
  },
  "actual_distance_km": 5.5,
  "actual_duration_minutes": 16
}
```

**Request Parameters:**
- `ride_id` (required): UUID of the ride to complete
- `final_location` (required): Final drop-off location
  - `latitude` (required): -90 to 90
  - `longitude` (required): -180 to 180
- `actual_distance_km` (required): Actual distance traveled (> 0)
- `actual_duration_minutes` (required): Actual ride duration (> 0)

**Response (200 OK):**
```json
{
  "ride_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "AVAILABLE",
  "completed_at": "2024-12-16T10:51:00Z",
  "driver_earnings": 1216.0,
  "message": "Ride completed successfully"
}
```

**Error Responses:**

- `400 Bad Request`: Invalid request or driver not busy
```json
{
  "error": "driver must be busy to complete a ride"
}
```

- `401 Unauthorized`: Missing or invalid token
- `500 Internal Server Error`: Server error

**Notes:**
- Driver status changes from `BUSY` to `AVAILABLE`
- Driver statistics updated (total_rides, total_earnings)
- Session statistics updated
- Final location recorded in location history

---

### 7. Get Driver Info

Retrieve driver information including current session.

**Endpoint:** `GET /drivers/{driver_id}`

**Authentication:** Required

**Headers:**
```
Authorization: Bearer {driver_token}
```

**Response (200 OK):**
```json
{
  "id": "660e8400-e29b-41d4-a716-446655440001",
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-12-16T10:00:00Z",
  "license_number": "DL123456",
  "vehicle_type": "ECONOMY",
  "vehicle_attrs": {
    "vehicle_make": "Toyota",
    "vehicle_model": "Camry",
    "vehicle_color": "White",
    "vehicle_plate": "KZ 123 ABC",
    "vehicle_year": 2020
  },
  "rating": 4.8,
  "total_rides": 150,
  "total_earnings": 250000.0,
  "status": "AVAILABLE",
  "is_verified": true,
  "email": "driver@example.com",
  "current_session": {
    "id": "session-123",
    "driver_id": "660e8400-e29b-41d4-a716-446655440001",
    "started_at": "2024-12-16T05:00:00Z",
    "total_rides": 12,
    "total_earnings": 18500.0
  }
}
```

**Error Responses:**

- `401 Unauthorized`: Missing or invalid token
- `404 Not Found`: Driver not found
- `500 Internal Server Error`: Server error

---

### 8. Get Nearby Drivers (Internal)

Find available drivers near a location. This is an internal endpoint for service-to-service communication.

**Endpoint:** `GET /internal/drivers/nearby`

**Authentication:** Internal service authentication

**Query Parameters:**
- `latitude` (required): Search center latitude
- `longitude` (required): Search center longitude
- `radius` (optional): Search radius in km (default: 5.0)
- `vehicle_type` (optional): Filter by vehicle type (default: "ECONOMY")
- `limit` (optional): Maximum number of results (default: 10)

**Example Request:**
```
GET /internal/drivers/nearby?latitude=43.238949&longitude=76.889709&radius=5&vehicle_type=ECONOMY&limit=10
```

**Response (200 OK):**
```json
{
  "drivers": [
    {
      "driver_id": "660e8400-e29b-41d4-a716-446655440001",
      "email": "driver@example.com",
      "rating": 4.8,
      "location": {
        "latitude": 43.236,
        "longitude": 76.888,
        "address": "123 Main St, Almaty"
      },
      "distance_km": 0.5,
      "vehicle": {
        "vehicle_make": "Toyota",
        "vehicle_model": "Camry",
        "vehicle_color": "White",
        "vehicle_plate": "KZ 123 ABC",
        "vehicle_year": 2020
      }
    }
  ],
  "count": 1
}
```

**Notes:**
- Uses PostGIS for geospatial queries
- Only returns drivers with status `AVAILABLE`
- Results sorted by distance (ascending) and rating (descending)

---

## WebSocket API

### Connection

**Endpoint:** `ws://localhost:8082/ws/drivers/{driver_id}`

**Authentication:** JWT token sent in first message

### Message Format

All WebSocket messages use JSON format:

```json
{
  "type": "message_type",
  "payload": { }
}
```

### Client to Server Messages

#### 1. Authentication

**Must be sent first after connection:**

```json
{
  "type": "auth",
  "token": "Bearer {driver_token}"
}
```

**Success Response:**
```json
{
  "type": "auth_success",
  "message": "Authentication successful"
}
```

**Error Response:**
```json
{
  "type": "auth_error",
  "error": "Invalid token"
}
```

---

#### 2. Ride Response

Driver's response to a ride offer:

```json
{
  "type": "ride_response",
  "offer_id": "offer_123456",
  "ride_id": "550e8400-e29b-41d4-a716-446655440000",
  "accepted": true,
  "current_location": {
    "latitude": 43.235,
    "longitude": 76.885
  }
}
```

**Parameters:**
- `offer_id` (required): The offer ID from the ride offer
- `ride_id` (required): The ride ID
- `accepted` (required): true if accepting, false if rejecting
- `current_location` (required): Driver's current location

---

#### 3. Location Update (via WebSocket)

Alternative to REST API for location updates:

```json
{
  "type": "location_update",
  "latitude": 43.236,
  "longitude": 76.886,
  "accuracy_meters": 5.0,
  "speed_kmh": 45.0,
  "heading_degrees": 180.0
}
```

---

#### 4. Ping

Keep-alive ping:

```json
{
  "type": "ping"
}
```

**Response:**
```json
{
  "type": "pong"
}
```

---

### Server to Client Messages

#### 1. Ride Offer

Server sends ride offer to available drivers:

```json
{
  "type": "ride_offer",
  "offer_id": "offer_123456",
  "ride_id": "550e8400-e29b-41d4-a716-446655440000",
  "ride_number": "RIDE_20241216_001",
  "pickup_location": {
    "latitude": 43.238949,
    "longitude": 76.889709,
    "address": "Almaty Central Park"
  },
  "destination_location": {
    "latitude": 43.222015,
    "longitude": 76.851511,
    "address": "Kok-Tobe Hill"
  },
  "estimated_fare": 1500.0,
  "driver_earnings": 1200.0,
  "distance_to_pickup_km": 2.1,
  "estimated_ride_duration_minutes": 15,
  "expires_at": "2024-12-16T10:32:00Z"
}
```

**Note:** Driver has until `expires_at` to respond (typically 30 seconds)

---

#### 2. Ride Details

Sent after driver accepts a ride:

```json
{
  "type": "ride_details",
  "ride_id": "550e8400-e29b-41d4-a716-446655440000",
  "passenger_name": "Saule Karimova",
  "passenger_phone": "+7-XXX-XXX-XX-XX",
  "pickup_location": {
    "latitude": 43.238949,
    "longitude": 76.889709,
    "address": "Almaty Central Park",
    "notes": "Near the main entrance"
  }
}
```

**Note:** Phone number partially masked for privacy

---

#### 3. Ride Cancelled

Notification that a ride was cancelled:

```json
{
  "type": "ride_cancelled",
  "ride_id": "550e8400-e29b-41d4-a716-446655440000",
  "reason": "Passenger cancelled",
  "timestamp": "2024-12-16T10:35:00Z"
}
```

---

## Message Queue Integration

### RabbitMQ Configuration

**Exchanges:**
- `ride_topic` (type: topic) - Ride-related events
- `driver_topic` (type: topic) - Driver-related events
- `location_fanout` (type: fanout) - Location broadcasts

**Queues:**
- `driver_matching` - Ride match requests
- `ride_status_update` - Ride status changes

---

### Incoming Messages

#### 1. Driver Match Request

**Exchange:** `ride_topic`  
**Routing Key:** `ride.request.{ride_type}`  
**Queue:** `driver_matching`

```json
{
  "ride_id": "550e8400-e29b-41d4-a716-446655440000",
  "ride_number": "RIDE_20241216_001",
  "pickup_location": {
    "latitude": 43.238949,
    "longitude": 76.889709,
    "address": "Almaty Central Park"
  },
  "destination_location": {
    "latitude": 43.222015,
    "longitude": 76.851511,
    "address": "Kok-Tobe Hill"
  },
  "ride_type": "ECONOMY",
  "estimated_fare": 1450.0,
  "max_distance_km": 5.0,
  "timeout_seconds": 30,
  "correlation_id": "req_123456"
}
```

**Processing:**
1. Query nearby available drivers using PostGIS
2. Filter by vehicle type and availability
3. Send ride offers via WebSocket to selected drivers
4. Wait for responses with timeout handling

---

#### 2. Ride Status Update

**Exchange:** `ride_topic`  
**Routing Key:** `ride.status.*`  
**Queue:** `ride_status_update`

```json
{
  "ride_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "COMPLETED",
  "timestamp": "2024-12-16T10:51:00Z",
  "final_fare": 1520.0,
  "correlation_id": "req_123456"
}
```

**Status Values:**
- `COMPLETED` - Ride completed successfully
- `CANCELLED` - Ride cancelled by passenger or driver
- `STARTED` - Ride started (passenger picked up)

---

### Outgoing Messages

#### 1. Driver Match Response

**Exchange:** `driver_topic`  
**Routing Key:** `driver.response.{ride_id}`

```json
{
  "ride_id": "550e8400-e29b-41d4-a716-446655440000",
  "driver_id": "660e8400-e29b-41d4-a716-446655440001",
  "accepted": true,
  "estimated_arrival_minutes": 3,
  "driver_location": {
    "latitude": 43.235,
    "longitude": 76.885
  },
  "driver_info": {
    "name": "Aidar Nurlan",
    "rating": 4.8,
    "vehicle": {
      "vehicle_make": "Toyota",
      "vehicle_model": "Camry",
      "vehicle_color": "White",
      "vehicle_plate": "KZ 123 ABC"
    }
  },
  "correlation_id": "req_123456"
}
```

---

#### 2. Driver Status Update

**Exchange:** `driver_topic`  
**Routing Key:** `driver.status.{driver_id}`

```json
{
  "driver_id": "660e8400-e29b-41d4-a716-446655440001",
  "status": "BUSY",
  "ride_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2024-12-16T10:35:00Z"
}
```

**Triggered By:**
- Driver goes online/offline
- Ride starts
- Ride completes

---

#### 3. Location Broadcast

**Exchange:** `location_fanout`  
**Routing Key:** (none - fanout)

```json
{
  "driver_id": "660e8400-e29b-41d4-a716-446655440001",
  "ride_id": "550e8400-e29b-41d4-a716-446655440000",
  "location": {
    "latitude": 43.236,
    "longitude": 76.886
  },
  "speed_kmh": 45.0,
  "heading_degrees": 180.0,
  "timestamp": "2024-12-16T10:35:30Z"
}
```

**Note:** Broadcast to all interested services (ride service, passenger app, analytics)

---

## Authentication

### JWT Token

All protected endpoints require a JWT token in the Authorization header:

```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### Token Claims

Expected claims in JWT:
```json
{
  "sub": "660e8400-e29b-41d4-a716-446655440001",
  "role": "driver",
  "exp": 1702742400
}
```

### Token Validation

1. Verify token signature
2. Check expiration
3. Validate role is "driver"
4. Extract driver_id from "sub" claim

---

## Error Handling

### Standard Error Response

```json
{
  "error": "Error message",
  "code": "ERROR_CODE",
  "details": {
    "field": "Additional context"
  }
}
```

### HTTP Status Codes

- `200 OK` - Request successful
- `400 Bad Request` - Invalid request data
- `401 Unauthorized` - Missing or invalid authentication
- `403 Forbidden` - Insufficient permissions
- `404 Not Found` - Resource not found
- `429 Too Many Requests` - Rate limit exceeded
- `500 Internal Server Error` - Server error

### Error Codes

- `UNAUTHORIZED` - Authentication required
- `INVALID_TOKEN` - Invalid JWT token
- `INVALID_LATITUDE` - Invalid latitude value
- `INVALID_LONGITUDE` - Invalid longitude value
- `INVALID_ACCURACY` - Invalid accuracy value
- `INVALID_SPEED` - Invalid speed value
- `INVALID_HEADING` - Invalid heading value
- `MISSING_PARAMETER` - Required parameter missing
- `RATE_LIMIT_EXCEEDED` - Too many requests

---

## Rate Limiting

### Location Updates

**Limit:** 1 update per 3 seconds per driver

**Implementation:** Token bucket algorithm with in-memory storage

**Rate Limit Headers:**
```
X-RateLimit-Limit: 1
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1702742403
```

**Exceeded Response:**
```
HTTP/1.1 429 Too Many Requests
Content-Type: application/json

{
  "error": "rate limit exceeded: maximum 1 update per 3 seconds"
}
```

---

## Database Schema

### Drivers Table

```sql
CREATE TABLE drivers (
    id UUID PRIMARY KEY REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    license_number VARCHAR(50) UNIQUE NOT NULL,
    vehicle_type TEXT REFERENCES vehicle_type(value),
    vehicle_attrs JSONB,
    rating DECIMAL(3,2) DEFAULT 5.0 CHECK (rating BETWEEN 1.0 AND 5.0),
    total_rides INTEGER DEFAULT 0 CHECK (total_rides >= 0),
    total_earnings DECIMAL(10,2) DEFAULT 0 CHECK (total_earnings >= 0),
    status TEXT REFERENCES driver_status(value),
    is_verified BOOLEAN DEFAULT FALSE
);
```

### Driver Sessions Table

```sql
CREATE TABLE driver_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    driver_id UUID REFERENCES drivers(id) NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMPTZ,
    total_rides INTEGER DEFAULT 0,
    total_earnings DECIMAL(10,2) DEFAULT 0
);
```

### Location History Table

```sql
CREATE TABLE location_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    coordinate_id UUID REFERENCES coordinates(id),
    driver_id UUID REFERENCES drivers(id),
    latitude DECIMAL(10,8) NOT NULL CHECK (latitude BETWEEN -90 AND 90),
    longitude DECIMAL(11,8) NOT NULL CHECK (longitude BETWEEN -180 AND 180),
    accuracy_meters DECIMAL(6,2),
    speed_kmh DECIMAL(5,2),
    heading_degrees DECIMAL(5,2) CHECK (heading_degrees BETWEEN 0 AND 360),
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ride_id UUID REFERENCES rides(id)
);
```

---

## Environment Variables

```bash
# Server Configuration
HOST=0.0.0.0
PORT=8082

# Database
DATABASE_URL=postgres://postgres:postgres@localhost:5432/ride_hail?sslmode=disable

# RabbitMQ
RABBITMQ_URL=amqp://guest:guest@localhost:5672/

# Logging
LOG_LEVEL=INFO
```

---

## Example Usage

### Complete Driver Workflow

#### 1. Driver Goes Online
```bash
curl -X POST http://localhost:8082/drivers/660e8400-e29b-41d4-a716-446655440001/online \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{
    "latitude": 43.238949,
    "longitude": 76.889709
  }'
```

#### 2. Driver Receives Ride Offer (WebSocket)
```json
{
  "type": "ride_offer",
  "ride_id": "550e8400-e29b-41d4-a716-446655440000",
  "estimated_fare": 1500.0,
  "driver_earnings": 1200.0
}
```

#### 3. Driver Accepts Ride (WebSocket)
```json
{
  "type": "ride_response",
  "ride_id": "550e8400-e29b-41d4-a716-446655440000",
  "accepted": true,
  "current_location": {
    "latitude": 43.235,
    "longitude": 76.885
  }
}
```

#### 4. Driver Updates Location During Ride
```bash
curl -X POST http://localhost:8082/drivers/660e8400-e29b-41d4-a716-446655440001/location \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{
    "latitude": 43.230,
    "longitude": 76.880,
    "speed_kmh": 45.0,
    "heading_degrees": 180.0
  }'
```

#### 5. Driver Starts Ride
```bash
curl -X POST http://localhost:8082/drivers/660e8400-e29b-41d4-a716-446655440001/start \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{
    "ride_id": "550e8400-e29b-41d4-a716-446655440000",
    "driver_location": {
      "latitude": 43.238949,
      "longitude": 76.889709
    }
  }'
```

#### 6. Driver Completes Ride
```bash
curl -X POST http://localhost:8082/drivers/660e8400-e29b-41d4-a716-446655440001/complete \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{
    "ride_id": "550e8400-e29b-41d4-a716-446655440000",
    "final_location": {
      "latitude": 43.222015,
      "longitude": 76.851511
    },
    "actual_distance_km": 5.5,
    "actual_duration_minutes": 16
  }'
```

#### 7. Driver Goes Offline
```bash
curl -X POST http://localhost:8082/drivers/660e8400-e29b-41d4-a716-446655440001/offline \
  -H "Authorization: Bearer {token}"
```

---

## Performance Considerations

### PostGIS Queries

- Spatial indexes on coordinates table
- Efficient distance calculations using geography type
- Limited result sets (default 10 drivers)

### Rate Limiting

- In-memory token bucket algorithm
- Automatic cleanup of stale buckets
- Low overhead per request

### Location Updates

- Async broadcasting via RabbitMQ fanout
- Transient messages (not persisted)
- Historical data archived separately

### WebSocket Connections

- Automatic ping/pong for keep-alive
- Buffered send channels to prevent blocking
- Graceful handling of disconnections

---

## Monitoring & Logging

### Structured Logging

All logs use JSON format:

```json
{
  "timestamp": "2024-12-16T10:30:00.000Z",
  "level": "INFO",
  "service": "driver-location-service",
  "action": "driver.go_online",
  "message": "Driver 660e8400 going online",
  "hostname": "server-01"
}
```

### Key Metrics to Monitor

- Active driver sessions
- Location updates per second
- WebSocket connections count
- Ride match success rate
- Average response time per endpoint
- Database connection pool usage
- RabbitMQ message throughput

---

## Security Best Practices

1. **JWT Validation**: Always validate token signature and expiration
2. **HTTPS**: Use TLS in production
3. **Rate Limiting**: Prevent abuse of location updates
4. **Input Validation**: Validate all coordinates and numeric values
5. **SQL Injection**: Use parameterized queries (pgx handles this)
6. **CORS**: Configure allowed origins appropriately
7. **Secrets**: Never commit credentials to version control

---

## Support

For issues or questions, contact the platform team or open an issue in the project repository.