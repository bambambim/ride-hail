package ports

import (
	"context"
	"ride-hail/services/driver_location_service/internal/domain"
)

// DriverRepository defines the interface for driver data operations
type DriverRepository interface {
	// GetByID retrieves a driver by ID
	GetByID(ctx context.Context, driverID string) (*domain.Driver, error)

	// UpdateStatus updates a driver's status
	UpdateStatus(ctx context.Context, driverID string, status domain.DriverStatus) error

	// UpdateStats updates driver statistics (total rides, earnings)
	UpdateStats(ctx context.Context, driverID string, totalRides int, totalEarnings float64) error

	// GetNearbyDrivers finds available drivers near a location
	GetNearbyDrivers(ctx context.Context, lat, lng float64, radiusKm float64, vehicleType string, limit int) ([]*domain.NearbyDriver, error)
}

// SessionRepository defines the interface for driver session operations
type SessionRepository interface {
	// CreateSession creates a new driver session
	CreateSession(ctx context.Context, driverID string) (*domain.DriverSession, error)

	// GetActiveSession retrieves the active session for a driver
	GetActiveSession(ctx context.Context, driverID string) (*domain.DriverSession, error)

	// EndSession ends a driver session
	EndSession(ctx context.Context, sessionID string, totalRides int, totalEarnings float64) (*domain.DriverSession, error)

	// UpdateSessionStats updates session statistics
	UpdateSessionStats(ctx context.Context, sessionID string, totalRides int, totalEarnings float64) error
}

// LocationRepository defines the interface for location operations
type LocationRepository interface {
	// GetCurrentLocation retrieves the current location for a driver
	GetCurrentLocation(ctx context.Context, driverID string) (*domain.Coordinate, error)

	// UpdateLocation updates a driver's current location
	UpdateLocation(ctx context.Context, update *domain.LocationUpdate) (*domain.Coordinate, error)

	// SaveLocationHistory saves location to history
	SaveLocationHistory(ctx context.Context, history *domain.LocationHistory) error

	// GetLocationHistory retrieves location history for a driver
	GetLocationHistory(ctx context.Context, driverID string, rideID *string, limit int) ([]*domain.LocationHistory, error)

	// ArchiveOldLocation marks old coordinate as not current
	ArchiveOldLocation(ctx context.Context, driverID string) error
}

// Repository aggregates all repository interfaces
type Repository interface {
	DriverRepository
	SessionRepository
	LocationRepository
}
