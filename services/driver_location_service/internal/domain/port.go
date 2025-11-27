package domain

import (
	"context"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// DriverLocationRepository handles persistence operations for driver location service
type DriverLocationRepository interface {
	// Driver operations
	GetDriver(ctx context.Context, driverID string) (*Driver, error)
	UpdateDriverStatus(ctx context.Context, driverID string, status string) error
	UpdateDriverSessionStats(ctx context.Context, driverID string, rides int, earnings float64) error

	// Session operations
	CreateDriverSession(ctx context.Context, driverID string) (string, error)
	EndDriverSession(ctx context.Context, sessionID string) (*DriverSession, error)
	GetActiveSession(ctx context.Context, driverID string) (*DriverSession, error)

	// Location operations
	SaveDriverLocation(ctx context.Context, driverID string, latitude, longitude float64, address string) (string, error)
	UpdateLocationWithMetrics(ctx context.Context, coordinateID string, accuracy, speed, heading float64) error
	ArchiveLocation(ctx context.Context, driverID string, lat, lng, accuracy, speed, heading float64, rideID string) error
	GetCurrentLocation(ctx context.Context, driverID string) (*Coordinate, error)
	GetLastLocationUpdate(ctx context.Context, driverID string) (*time.Time, error)

	// Matching operations
	FindNearbyDrivers(ctx context.Context, latitude, longitude float64, vehicleType string, radiusMeters float64, limit int) ([]*NearbyDriver, error)

	// Ride tracking
	SetDriverCurrentRide(ctx context.Context, driverID string, rideID string) error
	ClearDriverCurrentRide(ctx context.Context, driverID string) error
}

// DriverLocationService exposes the business operations used by adapters.
type DriverLocationService interface {
	DriverGoOnline(ctx context.Context, driverID string, latitude, longitude float64, address string) (string, error)
	DriverGoOffline(ctx context.Context, driverID string) (*DriverSession, error)
	UpdateDriverLocation(ctx context.Context, driverID string, latitude, longitude, accuracy, speed, heading float64, address string) (string, error)
	StartRide(ctx context.Context, driverID, rideID string) error
	CompleteRide(ctx context.Context, driverID, rideID string, actualDistanceKM float64, actualDurationMin int) (float64, error)
	HandleRideMatchingRequest(ctx context.Context, req *RideMatchingRequest) error
	HandleRideStatusUpdate(ctx context.Context, rideID string, status string, finalFare float64) error
	HandleDriverRideResponse(ctx context.Context, driverID, offerID, rideID string, accepted bool) error
}

// DriverLocationPublisher handles publishing events to message queues
type DriverLocationPublisher interface {
	PublishDriverResponse(ctx context.Context, exchange, routingKey string, body []byte) error
	PublishDriverStatus(ctx context.Context, exchange, routingKey string, body []byte) error
	PublishLocationUpdate(ctx context.Context, exchange string, body []byte) error
}

// DriverLocationSubscriber handles consuming messages from queues
type DriverLocationSubscriber interface {
	ConsumeDriverMatching(ctx context.Context, handler func(amqp.Delivery)) error
	ConsumeRideStatus(ctx context.Context, handler func(amqp.Delivery)) error
}

// WebSocketManager manages WebSocket connections for drivers
type WebSocketManager interface {
	SendRideOffer(driverID string, offer interface{}) error
	SendRideDetails(driverID string, details interface{}) error
	BroadcastToAll(message interface{}) error
	IsDriverConnected(driverID string) bool
}
