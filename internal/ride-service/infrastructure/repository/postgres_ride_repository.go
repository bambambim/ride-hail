package repository

import (
	"context"
	"fmt"
	"time"

	"ride-hail/internal/ride-service/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRideRepository implements domain.RideRepository interface
type PostgresRideRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRideRepository creates a new PostgreSQL repository
func NewPostgresRideRepository(db *pgxpool.Pool) *PostgresRideRepository {
	return &PostgresRideRepository{
		db: db,
	}
}

// Save persists a new ride
func (r *PostgresRideRepository) Save(ctx context.Context, ride *domain.Ride) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert ride
	_, err = tx.Exec(ctx, `
		INSERT INTO rides (
			id, ride_number, passenger_id, status, vehicle_type,
			estimated_fare, requested_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
	`,
		ride.ID(),
		ride.RideNumber(),
		ride.PassengerID(),
		ride.Status().String(),
		ride.RideTypeValue().String(),
		ride.EstimatedFare(),
		ride.RequestedAt(),
	)
	if err != nil {
		return fmt.Errorf("insert ride: %w", err)
	}

	// Insert pickup coordinate
	var pickupCoordID string
	pickup := ride.PickupLocation()
	err = tx.QueryRow(ctx, `
		INSERT INTO coordinates (
			entity_id, entity_type, address, latitude, longitude, is_current
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`,
		ride.PassengerID(),
		"passenger",
		pickup.Address(),
		pickup.Latitude(),
		pickup.Longitude(),
		true,
	).Scan(&pickupCoordID)
	if err != nil {
		return fmt.Errorf("insert pickup coordinate: %w", err)
	}

	// Insert destination coordinate
	var destCoordID string
	dest := ride.DestLocation()
	err = tx.QueryRow(ctx, `
		INSERT INTO coordinates (
			entity_id, entity_type, address, latitude, longitude, is_current
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`,
		ride.PassengerID(),
		"passenger",
		dest.Address(),
		dest.Latitude(),
		dest.Longitude(),
		false,
	).Scan(&destCoordID)
	if err != nil {
		return fmt.Errorf("insert destination coordinate: %w", err)
	}

	// Update ride with coordinate references
	_, err = tx.Exec(ctx, `
		UPDATE rides
		SET pickup_coordinate_id = $1, destination_coordinate_id = $2
		WHERE id = $3
	`, pickupCoordID, destCoordID, ride.ID())
	if err != nil {
		return fmt.Errorf("update ride coordinates: %w", err)
	}

	return tx.Commit(ctx)
}

// Update updates an existing ride
func (r *PostgresRideRepository) Update(ctx context.Context, ride *domain.Ride) error {
	_, err := r.db.Exec(ctx, `
		UPDATE rides
		SET
			status = $1,
			driver_id = $2,
			final_fare = $3,
			matched_at = $4,
			started_at = $5,
			completed_at = $6,
			cancelled_at = $7,
			cancellation_reason = $8,
			updated_at = NOW()
		WHERE id = $9
	`,
		ride.Status().String(),
		ride.DriverID(),
		ride.FinalFare(),
		ride.MatchedAt(),
		ride.StartedAt(),
		ride.CompletedAt(),
		ride.CancelledAt(),
		ride.CancelReason(),
		ride.ID(),
	)
	if err != nil {
		return fmt.Errorf("update ride: %w", err)
	}

	return nil
}

// FindByID retrieves a ride by its ID
func (r *PostgresRideRepository) FindByID(ctx context.Context, rideID string) (*domain.Ride, error) {
	var (
		id            string
		rideNumber    string
		passengerID   string
		driverID      *string
		status        string
		rideType      string
		estimatedFare float64
		finalFare     *float64
		requestedAt   interface{}
		matchedAt     *interface{}
		startedAt     *interface{}
		completedAt   *interface{}
		cancelledAt   *interface{}
		cancelReason  string
		pickupLat     float64
		pickupLng     float64
		pickupAddr    string
		destLat       float64
		destLng       float64
		destAddr      string
	)

	err := r.db.QueryRow(ctx, `
		SELECT
			r.id, r.ride_number, r.passenger_id, r.driver_id, r.status, r.vehicle_type,
			r.estimated_fare, r.final_fare, r.requested_at, r.matched_at, r.started_at,
			r.completed_at, r.cancelled_at, COALESCE(r.cancellation_reason, ''),
			COALESCE(cp.latitude, 0), COALESCE(cp.longitude, 0), COALESCE(cp.address, ''),
			COALESCE(cd.latitude, 0), COALESCE(cd.longitude, 0), COALESCE(cd.address, '')
		FROM rides r
		LEFT JOIN coordinates cp ON r.pickup_coordinate_id = cp.id
		LEFT JOIN coordinates cd ON r.destination_coordinate_id = cd.id
		WHERE r.id = $1
	`, rideID).Scan(
		&id, &rideNumber, &passengerID, &driverID, &status, &rideType,
		&estimatedFare, &finalFare, &requestedAt, &matchedAt, &startedAt,
		&completedAt, &cancelledAt, &cancelReason,
		&pickupLat, &pickupLng, &pickupAddr,
		&destLat, &destLng, &destAddr,
	)
	if err != nil {
		return nil, fmt.Errorf("query ride: %w", err)
	}

	return reconstructRide(
		id, rideNumber, passengerID, driverID, status, rideType,
		estimatedFare, finalFare, requestedAt, matchedAt, startedAt,
		completedAt, cancelledAt, cancelReason,
		pickupLat, pickupLng, pickupAddr,
		destLat, destLng, destAddr,
	)
}

// FindByPassenger retrieves a ride by ID and verifies passenger ownership
func (r *PostgresRideRepository) FindByPassenger(ctx context.Context, rideID string, passengerID string) (*domain.Ride, error) {
	var (
		id            string
		rideNumber    string
		pID           string
		driverID      *string
		status        string
		rideType      string
		estimatedFare float64
		finalFare     *float64
		requestedAt   interface{}
		matchedAt     *interface{}
		startedAt     *interface{}
		completedAt   *interface{}
		cancelledAt   *interface{}
		cancelReason  string
		pickupLat     float64
		pickupLng     float64
		pickupAddr    string
		destLat       float64
		destLng       float64
		destAddr      string
	)

	err := r.db.QueryRow(ctx, `
		SELECT
			r.id, r.ride_number, r.passenger_id, r.driver_id, r.status, r.vehicle_type,
			r.estimated_fare, r.final_fare, r.requested_at, r.matched_at, r.started_at,
			r.completed_at, r.cancelled_at, COALESCE(r.cancellation_reason, ''),
			COALESCE(cp.latitude, 0), COALESCE(cp.longitude, 0), COALESCE(cp.address, ''),
			COALESCE(cd.latitude, 0), COALESCE(cd.longitude, 0), COALESCE(cd.address, '')
		FROM rides r
		LEFT JOIN coordinates cp ON r.pickup_coordinate_id = cp.id
		LEFT JOIN coordinates cd ON r.destination_coordinate_id = cd.id
		WHERE r.id = $1 AND r.passenger_id = $2
	`, rideID, passengerID).Scan(
		&id, &rideNumber, &pID, &driverID, &status, &rideType,
		&estimatedFare, &finalFare, &requestedAt, &matchedAt, &startedAt,
		&completedAt, &cancelledAt, &cancelReason,
		&pickupLat, &pickupLng, &pickupAddr,
		&destLat, &destLng, &destAddr,
	)
	if err != nil {
		return nil, fmt.Errorf("query ride: %w", err)
	}

	return reconstructRide(
		id, rideNumber, pID, driverID, status, rideType,
		estimatedFare, finalFare, requestedAt, matchedAt, startedAt,
		completedAt, cancelledAt, cancelReason,
		pickupLat, pickupLng, pickupAddr,
		destLat, destLng, destAddr,
	)
}

// FindActiveByPassenger retrieves active rides for a passenger
func (r *PostgresRideRepository) FindActiveByPassenger(ctx context.Context, passengerID string) ([]*domain.Ride, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			r.id, r.ride_number, r.passenger_id, r.driver_id, r.status, r.vehicle_type,
			r.estimated_fare, r.final_fare, r.requested_at, r.matched_at, r.started_at,
			r.completed_at, r.cancelled_at, COALESCE(r.cancellation_reason, ''),
			COALESCE(cp.latitude, 0), COALESCE(cp.longitude, 0), COALESCE(cp.address, ''),
			COALESCE(cd.latitude, 0), COALESCE(cd.longitude, 0), COALESCE(cd.address, '')
		FROM rides r
		LEFT JOIN coordinates cp ON r.pickup_coordinate_id = cp.id
		LEFT JOIN coordinates cd ON r.destination_coordinate_id = cd.id
		WHERE r.passenger_id = $1 AND r.status NOT IN ('COMPLETED', 'CANCELLED')
		ORDER BY r.requested_at DESC
	`, passengerID)
	if err != nil {
		return nil, fmt.Errorf("query active rides: %w", err)
	}
	defer rows.Close()

	var rides []*domain.Ride
	for rows.Next() {
		var (
			id            string
			rideNumber    string
			pID           string
			driverID      *string
			status        string
			rideType      string
			estimatedFare float64
			finalFare     *float64
			requestedAt   interface{}
			matchedAt     *interface{}
			startedAt     *interface{}
			completedAt   *interface{}
			cancelledAt   *interface{}
			cancelReason  string
			pickupLat     float64
			pickupLng     float64
			pickupAddr    string
			destLat       float64
			destLng       float64
			destAddr      string
		)

		err := rows.Scan(
			&id, &rideNumber, &pID, &driverID, &status, &rideType,
			&estimatedFare, &finalFare, &requestedAt, &matchedAt, &startedAt,
			&completedAt, &cancelledAt, &cancelReason,
			&pickupLat, &pickupLng, &pickupAddr,
			&destLat, &destLng, &destAddr,
		)
		if err != nil {
			return nil, fmt.Errorf("scan ride: %w", err)
		}

		ride, err := reconstructRide(
			id, rideNumber, pID, driverID, status, rideType,
			estimatedFare, finalFare, requestedAt, matchedAt, startedAt,
			completedAt, cancelledAt, cancelReason,
			pickupLat, pickupLng, pickupAddr,
			destLat, destLng, destAddr,
		)
		if err != nil {
			return nil, err
		}

		rides = append(rides, ride)
	}

	return rides, nil
}

// FindByStatus retrieves rides by status
func (r *PostgresRideRepository) FindByStatus(ctx context.Context, status domain.RideStatus) ([]*domain.Ride, error) {
	// Implementation similar to FindActiveByPassenger
	// Left as exercise or can be implemented later
	return nil, fmt.Errorf("not implemented")
}

// Delete removes a ride (soft delete recommended in production)
func (r *PostgresRideRepository) Delete(ctx context.Context, rideID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM rides WHERE id = $1`, rideID)
	if err != nil {
		return fmt.Errorf("delete ride: %w", err)
	}
	return nil
}

// Helper function to reconstruct ride from database row
func reconstructRide(
	id, rideNumber, passengerID string,
	driverID *string,
	status, rideType string,
	estimatedFare float64,
	finalFare *float64,
	requestedAt, matchedAt, startedAt, completedAt, cancelledAt interface{},
	cancelReason string,
	pickupLat, pickupLng float64, pickupAddr string,
	destLat, destLng float64, destAddr string,
) (*domain.Ride, error) {
	// Reconstruct coordinates
	pickup, _ := domain.NewCoordinate(pickupLat, pickupLng, pickupAddr)
	dest, _ := domain.NewCoordinate(destLat, destLng, destAddr)

	// Parse timestamps
	reqAt := parseTime(requestedAt)
	matchAt := parseTimeFromInterface(matchedAt)
	startAt := parseTimeFromInterface(startedAt)
	completeAt := parseTimeFromInterface(completedAt)
	cancelAt := parseTimeFromInterface(cancelledAt)

	return domain.ReconstructRide(
		id,
		rideNumber,
		passengerID,
		driverID,
		domain.RideStatus(status),
		domain.RideType(rideType),
		pickup,
		dest,
		estimatedFare,
		finalFare,
		reqAt,
		matchAt,
		startAt,
		completeAt,
		cancelAt,
		cancelReason,
	), nil
}

// Helper functions for time parsing
func parseTime(t interface{}) time.Time {
	if t == nil {
		return time.Time{}
	}
	if tv, ok := t.(time.Time); ok {
		return tv
	}
	return time.Time{}
}

func parseTimeFromInterface(t interface{}) *time.Time {
	if t == nil {
		return nil
	}
	if tv, ok := t.(time.Time); ok {
		return &tv
	}
	return nil
}

// ============================================
// Consumer-friendly methods (for backward compatibility)
// ============================================

// UpdateRideStatus updates only the ride status (used by consumers)
func (r *PostgresRideRepository) UpdateRideStatus(ctx context.Context, rideID string, status string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE rides
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`, status, rideID)
	if err != nil {
		return fmt.Errorf("update ride status: %w", err)
	}
	return nil
}

// AssignDriver assigns a driver to a ride (used by consumers)
func (r *PostgresRideRepository) AssignDriver(ctx context.Context, rideID string, driverID string) error {
	now := time.Now()
	_, err := r.db.Exec(ctx, `
		UPDATE rides
		SET driver_id = $1, matched_at = $2, updated_at = NOW()
		WHERE id = $3
	`, driverID, now, rideID)
	if err != nil {
		return fmt.Errorf("assign driver: %w", err)
	}
	return nil
}
