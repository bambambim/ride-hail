package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ride-hail/internal/driver_location_service/domain"
	"ride-hail/pkg/config"
	"ride-hail/pkg/db"
	"ride-hail/pkg/logger"
)

type PostgresDriverLocationRepository struct {
	log  logger.Logger
	cfg  *config.Config
	pool *pgxpool.Pool
}

func NewPostgresDriverLocationRepository(log logger.Logger, cfg *config.Config) (*PostgresDriverLocationRepository, error) {
	pool, err := db.NewConnection(cfg, log)
	if err != nil {
		log.Error("db_connection_failed", err)
		return nil, fmt.Errorf("failed to create db connection: %w", err)
	}
	return &PostgresDriverLocationRepository{
		log:  log,
		cfg:  cfg,
		pool: pool,
	}, nil
}

// GetDriver retrieves driver information
func (r *PostgresDriverLocationRepository) GetDriver(ctx context.Context, driverID string) (*domain.Driver, error) {
	query := `
		SELECT d.id, u.email, d.license_number, d.vehicle_type, d.vehicle_attrs, 
		       d.rating, d.total_rides, d.total_earnings, d.status, d.is_verified
		FROM drivers d
		JOIN users u ON d.id = u.id
		WHERE d.id = $1
	`
	var driver domain.Driver
	var vehicleAttrsJSON []byte

	err := r.pool.QueryRow(ctx, query, driverID).Scan(
		&driver.ID, &driver.Email, &driver.LicenseNumber, &driver.VehicleType,
		&vehicleAttrsJSON, &driver.Rating, &driver.TotalRides, &driver.TotalEarnings,
		&driver.Status, &driver.IsVerified,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("driver not found: %s", driverID)
		}
		return nil, fmt.Errorf("failed to get driver: %w", err)
	}

	if len(vehicleAttrsJSON) > 0 {
		err = json.Unmarshal(vehicleAttrsJSON, &driver.VehicleAttrs)
		if err != nil {
			r.log.Error("unmarshal_vehicle_attrs_failed", err)
		}
	}

	return &driver, nil
}

// UpdateDriverStatus changes the driver's status
func (r *PostgresDriverLocationRepository) UpdateDriverStatus(ctx context.Context, driverID string, status string) error {
	if status != domain.DriverStatusOffline && status != domain.DriverStatusAvailable &&
		status != domain.DriverStatusBusy && status != domain.DriverStatusEnRoute {
		return fmt.Errorf("invalid status value: %s", status)
	}

	query := `UPDATE drivers SET status = $1, updated_at = now() WHERE id = $2`
	_, err := r.pool.Exec(ctx, query, status, driverID)
	if err != nil {
		return fmt.Errorf("failed to update driver status: %w", err)
	}
	return nil
}

// UpdateDriverSessionStats updates total rides and earnings for a driver
func (r *PostgresDriverLocationRepository) UpdateDriverSessionStats(ctx context.Context, driverID string, rides int, earnings float64) error {
	query := `
		UPDATE drivers 
		SET total_rides = total_rides + $1, 
		    total_earnings = total_earnings + $2,
		    updated_at = now()
		WHERE id = $3
	`
	_, err := r.pool.Exec(ctx, query, rides, earnings, driverID)
	if err != nil {
		return fmt.Errorf("failed to update driver stats: %w", err)
	}
	return nil
}

// CreateDriverSession creates a new session when driver goes online
func (r *PostgresDriverLocationRepository) CreateDriverSession(ctx context.Context, driverID string) (string, error) {
	query := `
		INSERT INTO driver_sessions (driver_id, started_at, total_rides, total_earnings)
		VALUES ($1, now(), 0, 0)
		RETURNING id
	`
	var sessionID string
	err := r.pool.QueryRow(ctx, query, driverID).Scan(&sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to create driver session: %w", err)
	}
	return sessionID, nil
}

// EndDriverSession ends the current session and returns summary
func (r *PostgresDriverLocationRepository) EndDriverSession(ctx context.Context, sessionID string) (*domain.DriverSession, error) {
	query := `
		UPDATE driver_sessions 
		SET ended_at = now()
		WHERE id = $1 AND ended_at IS NULL
		RETURNING id, driver_id, started_at, ended_at, total_rides, total_earnings
	`
	var session domain.DriverSession
	err := r.pool.QueryRow(ctx, query, sessionID).Scan(
		&session.ID, &session.DriverID, &session.StartedAt, &session.EndedAt,
		&session.TotalRides, &session.TotalEarnings,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("session not found or already ended: %s", sessionID)
		}
		return nil, fmt.Errorf("failed to end driver session: %w", err)
	}
	return &session, nil
}

// GetActiveSession retrieves the active session for a driver
func (r *PostgresDriverLocationRepository) GetActiveSession(ctx context.Context, driverID string) (*domain.DriverSession, error) {
	query := `
		SELECT id, driver_id, started_at, ended_at, total_rides, total_earnings
		FROM driver_sessions
		WHERE driver_id = $1 AND ended_at IS NULL
		ORDER BY started_at DESC
		LIMIT 1
	`
	var session domain.DriverSession
	err := r.pool.QueryRow(ctx, query, driverID).Scan(
		&session.ID, &session.DriverID, &session.StartedAt, &session.EndedAt,
		&session.TotalRides, &session.TotalEarnings,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // No active session
		}
		return nil, fmt.Errorf("failed to get active session: %w", err)
	}
	return &session, nil
}

// SaveDriverLocation saves a new location coordinate for driver
func (r *PostgresDriverLocationRepository) SaveDriverLocation(ctx context.Context, driverID string, latitude, longitude float64, address string) (string, error) {
	// Start transaction to update old location and insert new one
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Mark old location as not current
	updateQuery := `
		UPDATE coordinates 
		SET is_current = false, updated_at = now()
		WHERE entity_id = $1 AND entity_type = 'driver' AND is_current = true
	`
	_, err = tx.Exec(ctx, updateQuery, driverID)
	if err != nil {
		return "", fmt.Errorf("failed to update old location: %w", err)
	}

	// Insert new location
	insertQuery := `
		INSERT INTO coordinates (entity_id, entity_type, address, latitude, longitude, is_current, created_at, updated_at)
		VALUES ($1, 'driver', $2, $3, $4, true, now(), now())
		RETURNING id
	`
	var coordinateID string
	err = tx.QueryRow(ctx, insertQuery, driverID, address, latitude, longitude).Scan(&coordinateID)
	if err != nil {
		return "", fmt.Errorf("failed to insert new location: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return coordinateID, nil
}

// UpdateLocationWithMetrics updates location metrics (not used for primary coordinate table)
func (r *PostgresDriverLocationRepository) UpdateLocationWithMetrics(ctx context.Context, coordinateID string, accuracy, speed, heading float64) error {
	// This could be used if we add these fields to coordinates table
	// For now we store these in location_history
	return nil
}

// ArchiveLocation stores historical location data
func (r *PostgresDriverLocationRepository) ArchiveLocation(ctx context.Context, driverID string, lat, lng, accuracy, speed, heading float64, rideID string) error {
	query := `
		INSERT INTO location_history (driver_id, latitude, longitude, accuracy_meters, speed_kmh, heading_degrees, recorded_at, ride_id)
		VALUES ($1, $2, $3, $4, $5, $6, now(), $7)
	`
	var rideIDPtr *string
	if rideID != "" {
		rideIDPtr = &rideID
	}
	_, err := r.pool.Exec(ctx, query, driverID, lat, lng, accuracy, speed, heading, rideIDPtr)
	if err != nil {
		return fmt.Errorf("failed to archive location: %w", err)
	}
	return nil
}

// GetCurrentLocation retrieves the driver's current location
func (r *PostgresDriverLocationRepository) GetCurrentLocation(ctx context.Context, driverID string) (*domain.Coordinate, error) {
	query := `
		SELECT id, entity_id, entity_type, address, latitude, longitude, is_current, created_at, updated_at
		FROM coordinates
		WHERE entity_id = $1 AND entity_type = 'driver' AND is_current = true
		LIMIT 1
	`
	var coord domain.Coordinate
	err := r.pool.QueryRow(ctx, query, driverID).Scan(
		&coord.ID, &coord.EntityID, &coord.EntityType, &coord.Address,
		&coord.Latitude, &coord.Longitude, &coord.IsCurrent,
		&coord.CreatedAt, &coord.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get current location: %w", err)
	}
	return &coord, nil
}

// GetLastLocationUpdate retrieves timestamp of last location update
func (r *PostgresDriverLocationRepository) GetLastLocationUpdate(ctx context.Context, driverID string) (*time.Time, error) {
	query := `
		SELECT recorded_at
		FROM location_history
		WHERE driver_id = $1
		ORDER BY recorded_at DESC
		LIMIT 1
	`
	var lastUpdate time.Time
	err := r.pool.QueryRow(ctx, query, driverID).Scan(&lastUpdate)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get last location update: %w", err)
	}
	return &lastUpdate, nil
}

// FindNearbyDrivers finds drivers within radius using PostGIS
func (r *PostgresDriverLocationRepository) FindNearbyDrivers(ctx context.Context, latitude, longitude float64, vehicleType string, radiusMeters float64, limit int) ([]*domain.NearbyDriver, error) {
	query := `
		SELECT d.id, u.email, d.rating, c.latitude, c.longitude,
       ST_Distance(
         ST_MakePoint(c.longitude, c.latitude)::geography,
         ST_MakePoint($2, $1)::geography
       ) / 1000 as distance_km
FROM drivers d
JOIN users u ON d.id = u.id
JOIN coordinates c ON c.entity_id = d.id
  AND c.entity_type = 'driver'
  AND c.is_current = true
WHERE d.status = 'AVAILABLE'
  AND d.vehicle_type = $3
  AND ST_DWithin(
        ST_MakePoint(c.longitude, c.latitude)::geography,
        ST_MakePoint( $2, $1)::geography,
        5000  -- 5km radius
      )
ORDER BY distance_km, d.rating DESC
LIMIT 10
	`
	fmt.Println(longitude, latitude)
	rows, err := r.pool.Query(ctx, query, latitude, longitude, vehicleType)
	if err != nil {
		return nil, fmt.Errorf("failed to find nearby drivers: %w", err)
	}
	defer rows.Close()

	var drivers []*domain.NearbyDriver
	for rows.Next() {
		var driver domain.NearbyDriver
		var vehicleAttrsJSON []byte

		err := rows.Scan(
			&driver.DriverID, &driver.Email, &driver.Rating,
			&driver.Latitude, &driver.Longitude, &driver.DistanceKm,
		)
		if err != nil {
			r.log.Error("scan_nearby_driver_failed", err)
			continue
		}

		if len(vehicleAttrsJSON) > 0 {
			err = json.Unmarshal(vehicleAttrsJSON, &driver.VehicleInfo)
			if err != nil {
				r.log.Error("unmarshal_vehicle_info_failed", err)
			}
		}

		drivers = append(drivers, &driver)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating nearby drivers: %w", err)
	}
	fmt.Println(drivers)
	return drivers, nil
}

// SetDriverCurrentRide assigns a ride to a driver
func (r *PostgresDriverLocationRepository) SetDriverCurrentRide(ctx context.Context, driverID string, rideID string) error {
	// We can store this in driver state or use a separate tracking table
	// For now, we'll just update the driver status to BUSY
	return r.UpdateDriverStatus(ctx, driverID, domain.DriverStatusBusy)
}

// ClearDriverCurrentRide clears the ride assignment
func (r *PostgresDriverLocationRepository) ClearDriverCurrentRide(ctx context.Context, driverID string) error {
	// Reset driver to AVAILABLE
	return r.UpdateDriverStatus(ctx, driverID, domain.DriverStatusAvailable)
}

// Close releases the underlying database pool.
func (r *PostgresDriverLocationRepository) Close() {
	if r.pool != nil {
		r.pool.Close()
	}
}
