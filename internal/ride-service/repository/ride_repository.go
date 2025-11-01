package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RideRepository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *RideRepository {
	return &RideRepository{db: db}
}

type Ride struct {
	RideID        string
	PassengerID   string
	DriverID      *string
	Status        string
	RideType      string
	EstimatedFare float64
	RequestedAt   time.Time
	MatchedAt     *time.Time
	CompletedAt   *time.Time
}

type Coordinate struct {
	RideID    string
	Type      string // PICKUP or DESTINATION
	Latitude  float64
	Longitude float64
	IsCurrent bool
}

// CreateRide creates a new ride with pickup and destination coordinates
func (r *RideRepository) CreateRide(ctx context.Context, ride Ride, pickup, dest Coordinate) error {
	// Start transaction
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert ride with ride_number generation
	rideNumber := fmt.Sprintf("RIDE-%d", time.Now().Unix())

	_, err = tx.Exec(ctx, `
		INSERT INTO rides (id, ride_number, passenger_id, status, vehicle_type, estimated_fare, requested_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, ride.RideID, rideNumber, ride.PassengerID, ride.Status, ride.RideType, ride.EstimatedFare, ride.RequestedAt)
	if err != nil {
		return fmt.Errorf("insert ride: %w", err)
	}

	// Insert pickup coordinate
	var pickupCoordID string
	err = tx.QueryRow(ctx, `
		INSERT INTO coordinates (entity_id, entity_type, address, latitude, longitude, is_current)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, ride.PassengerID, "passenger", "Pickup Location", pickup.Latitude, pickup.Longitude, pickup.IsCurrent).Scan(&pickupCoordID)
	if err != nil {
		return fmt.Errorf("insert pickup: %w", err)
	}

	// Insert destination coordinate
	var destCoordID string
	err = tx.QueryRow(ctx, `
		INSERT INTO coordinates (entity_id, entity_type, address, latitude, longitude, is_current)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, ride.PassengerID, "passenger", "Destination", dest.Latitude, dest.Longitude, dest.IsCurrent).Scan(&destCoordID)
	if err != nil {
		return fmt.Errorf("insert destination: %w", err)
	}

	// Update ride with coordinate references
	_, err = tx.Exec(ctx, `
		UPDATE rides 
		SET pickup_coordinate_id = $1, destination_coordinate_id = $2
		WHERE id = $3
	`, pickupCoordID, destCoordID, ride.RideID)
	if err != nil {
		return fmt.Errorf("update ride coordinates: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// UpdateRideStatus updates the status of a ride
func (r *RideRepository) UpdateRideStatus(ctx context.Context, rideID, status string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE rides SET status = $1, updated_at = now() WHERE id = $2
	`, status, rideID)
	return err
}

// AssignDriver assigns a driver to a ride
func (r *RideRepository) AssignDriver(ctx context.Context, rideID, driverID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE rides SET driver_id = $1, matched_at = $2, updated_at = now() WHERE id = $3
	`, driverID, time.Now(), rideID)
	return err
}

// GetRide retrieves a ride by ID
func (r *RideRepository) GetRide(ctx context.Context, rideID string) (*Ride, error) {
	var ride Ride
	err := r.db.QueryRow(ctx, `
		SELECT id, passenger_id, driver_id, status, vehicle_type, estimated_fare, 
		       requested_at, matched_at, completed_at
		FROM rides 
		WHERE id = $1
	`, rideID).Scan(
		&ride.RideID,
		&ride.PassengerID,
		&ride.DriverID,
		&ride.Status,
		&ride.RideType,
		&ride.EstimatedFare,
		&ride.RequestedAt,
		&ride.MatchedAt,
		&ride.CompletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &ride, nil
}

// GetRideByPassenger retrieves a ride and verifies passenger ownership
func (r *RideRepository) GetRideByPassenger(ctx context.Context, rideID, passengerID string) (*Ride, error) {
	var ride Ride
	err := r.db.QueryRow(ctx, `
		SELECT id, passenger_id, driver_id, status, vehicle_type, estimated_fare, 
		       requested_at, matched_at, completed_at
		FROM rides 
		WHERE id = $1 AND passenger_id = $2
	`, rideID, passengerID).Scan(
		&ride.RideID,
		&ride.PassengerID,
		&ride.DriverID,
		&ride.Status,
		&ride.RideType,
		&ride.EstimatedFare,
		&ride.RequestedAt,
		&ride.MatchedAt,
		&ride.CompletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &ride, nil
}

// CancelRide cancels a ride by updating its status
func (r *RideRepository) CancelRide(ctx context.Context, rideID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE rides 
		SET status = 'CANCELLED', cancelled_at = $1, updated_at = now() 
		WHERE id = $2
	`, time.Now(), rideID)
	return err
}
