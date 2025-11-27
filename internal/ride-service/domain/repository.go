package domain

import "context"

// RideRepository is the interface (port) for ride persistence
// This belongs in domain layer - implementation is in infrastructure
type RideRepository interface {
	// Save persists a new ride
	Save(ctx context.Context, ride *Ride) error

	// Update updates an existing ride
	Update(ctx context.Context, ride *Ride) error

	// FindByID retrieves a ride by its ID
	FindByID(ctx context.Context, rideID string) (*Ride, error)

	// FindByPassenger retrieves a ride by ID and verifies passenger ownership
	FindByPassenger(ctx context.Context, rideID string, passengerID string) (*Ride, error)

	// FindActiveByPassenger retrieves active rides for a passenger
	FindActiveByPassenger(ctx context.Context, passengerID string) ([]*Ride, error)

	// FindByStatus retrieves rides by status
	FindByStatus(ctx context.Context, status RideStatus) ([]*Ride, error)

	// Delete removes a ride (soft delete recommended)
	Delete(ctx context.Context, rideID string) error
}
