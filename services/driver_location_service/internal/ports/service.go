package ports

import (
	"context"
	"ride-hail/services/driver_location_service/internal/domain"
)

// DriverService defines the business logic interface for driver operations
type DriverService interface {
	// GoOnline sets a driver's status to available and creates a session
	GoOnline(ctx context.Context, driverID string, req *domain.OnlineRequest) (*domain.OnlineResponse, error)

	// GoOffline sets a driver's status to offline and ends the session
	GoOffline(ctx context.Context, driverID string) (*domain.OfflineResponse, error)

	// UpdateLocation updates a driver's current location
	UpdateLocation(ctx context.Context, driverID string, req *domain.UpdateLocationRequest) (*domain.UpdateLocationResponse, error)

	// StartRide marks a driver as busy and starts tracking a ride
	StartRide(ctx context.Context, driverID string, req *domain.StartRideRequest) (*domain.StartRideResponse, error)

	// CompleteRide marks a ride as complete and updates driver stats
	CompleteRide(ctx context.Context, driverID string, req *domain.CompleteRideRequest) (*domain.CompleteRideResponse, error)

	// GetDriver retrieves driver information
	GetDriver(ctx context.Context, driverID string) (*domain.Driver, error)

	// GetNearbyDrivers finds available drivers near a location
	GetNearbyDrivers(ctx context.Context, lat, lng float64, radiusKm float64, vehicleType string, limit int) ([]*domain.NearbyDriver, error)

	// HandleRideRequest processes incoming ride requests and finds drivers
	HandleRideRequest(ctx context.Context, request *domain.RideRequest) error

	// HandleRideStatusUpdate processes ride status updates
	HandleRideStatusUpdate(ctx context.Context, update *domain.RideStatusUpdate) error
}

// MessageBroker defines the interface for message queue operations
type MessageBroker interface {
	// PublishDriverResponse publishes a driver's response to a ride offer
	PublishDriverResponse(ctx context.Context, response *domain.DriverMatchResponse) error

	// PublishDriverStatusUpdate publishes driver status changes
	PublishDriverStatusUpdate(ctx context.Context, update *domain.DriverStatusUpdate) error

	// PublishLocationUpdate publishes location updates to fanout exchange
	PublishLocationUpdate(ctx context.Context, broadcast *domain.LocationBroadcast) error

	// ConsumeRideRequests starts consuming ride request messages
	ConsumeRideRequests(ctx context.Context, handler func(context.Context, *domain.RideRequest) error) error

	// ConsumeRideStatusUpdates starts consuming ride status update messages
	ConsumeRideStatusUpdates(ctx context.Context, handler func(context.Context, *domain.RideStatusUpdate) error) error

	// Close closes the message broker connection
	Close() error
}

// WebSocketHub defines the interface for WebSocket connections management
type WebSocketHub interface {
	// SendRideOffer sends a ride offer to a specific driver
	SendRideOffer(driverID string, offer *domain.RideOffer) error

	// SendRideDetails sends ride details after acceptance
	SendRideDetails(driverID string, details *domain.RideDetails) error

	// BroadcastToDriver sends a generic message to a driver
	BroadcastToDriver(driverID string, message interface{}) error

	// RegisterDriver registers a driver's WebSocket connection
	RegisterDriver(driverID string, conn interface{}) error

	// UnregisterDriver removes a driver's WebSocket connection
	UnregisterDriver(driverID string) error

	// IsDriverConnected checks if a driver is connected via WebSocket
	IsDriverConnected(driverID string) bool
}

// RateLimiter defines the interface for rate limiting operations
type RateLimiter interface {
	// Allow checks if an action is allowed for a given key
	Allow(ctx context.Context, key string) (bool, error)

	// AllowN checks if N actions are allowed for a given key
	AllowN(ctx context.Context, key string, n int) (bool, error)

	// Reset resets the rate limit for a given key
	Reset(ctx context.Context, key string) error
}
