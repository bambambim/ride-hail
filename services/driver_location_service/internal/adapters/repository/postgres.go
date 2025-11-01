package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"ride-hail/services/driver_location_service/internal/domain"
	"ride-hail/services/driver_location_service/internal/ports"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository implements the Repository interface using PostgreSQL
type PostgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *pgxpool.Pool) ports.Repository {
	return &PostgresRepository{db: db}
}

// GetByID retrieves a driver by ID
func (r *PostgresRepository) GetByID(ctx context.Context, driverID string) (*domain.Driver, error) {
	query := `
		SELECT d.id, d.created_at, d.updated_at, d.license_number, d.vehicle_type,
		       d.vehicle_attrs, d.rating, d.total_rides, d.total_earnings,
		       d.status, d.is_verified, u.email
		FROM drivers d
		JOIN users u ON d.id = u.id
		WHERE d.id = $1
	`

	var driver domain.Driver
	var vehicleAttrsJSON []byte

	err := r.db.QueryRow(ctx, query, driverID).Scan(
		&driver.ID,
		&driver.CreatedAt,
		&driver.UpdatedAt,
		&driver.LicenseNumber,
		&driver.VehicleType,
		&vehicleAttrsJSON,
		&driver.Rating,
		&driver.TotalRides,
		&driver.TotalEarnings,
		&driver.Status,
		&driver.IsVerified,
		&driver.Email,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("driver not found: %s", driverID)
		}
		return nil, fmt.Errorf("failed to get driver: %w", err)
	}

	// Unmarshal vehicle attributes
	if len(vehicleAttrsJSON) > 0 {
		if err := json.Unmarshal(vehicleAttrsJSON, &driver.VehicleAttrs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal vehicle attrs: %w", err)
		}
	}

	return &driver, nil
}

// UpdateStatus updates a driver's status
func (r *PostgresRepository) UpdateStatus(ctx context.Context, driverID string, status domain.DriverStatus) error {
	query := `
		UPDATE drivers
		SET status = $1, updated_at = now()
		WHERE id = $2
	`

	result, err := r.db.Exec(ctx, query, status, driverID)
	if err != nil {
		return fmt.Errorf("failed to update driver status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("driver not found: %s", driverID)
	}

	return nil
}

// UpdateStats updates driver statistics
func (r *PostgresRepository) UpdateStats(ctx context.Context, driverID string, totalRides int, totalEarnings float64) error {
	query := `
		UPDATE drivers
		SET total_rides = $1, total_earnings = $2, updated_at = now()
		WHERE id = $3
	`

	result, err := r.db.Exec(ctx, query, totalRides, totalEarnings, driverID)
	if err != nil {
		return fmt.Errorf("failed to update driver stats: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("driver not found: %s", driverID)
	}

	return nil
}

// GetNearbyDrivers finds available drivers near a location using PostGIS
func (r *PostgresRepository) GetNearbyDrivers(ctx context.Context, lat, lng float64, radiusKm float64, vehicleType string, limit int) ([]*domain.NearbyDriver, error) {
	query := `
		SELECT d.id, u.email, d.rating, d.vehicle_attrs,
		       c.latitude, c.longitude, c.address,
		       ST_Distance(
		         ST_MakePoint(c.longitude, c.latitude)::geography,
		         ST_MakePoint($1, $2)::geography
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
		        ST_MakePoint($1, $2)::geography,
		        $4 * 1000
		      )
		ORDER BY distance_km, d.rating DESC
		LIMIT $5
	`

	rows, err := r.db.Query(ctx, query, lng, lat, vehicleType, radiusKm, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query nearby drivers: %w", err)
	}
	defer rows.Close()

	var drivers []*domain.NearbyDriver
	for rows.Next() {
		var driver domain.NearbyDriver
		var vehicleAttrsJSON []byte
		var address string

		err := rows.Scan(
			&driver.DriverID,
			&driver.Email,
			&driver.Rating,
			&vehicleAttrsJSON,
			&driver.Location.Latitude,
			&driver.Location.Longitude,
			&address,
			&driver.DistanceKm,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan driver row: %w", err)
		}

		driver.Location.Address = address

		// Unmarshal vehicle attributes
		if len(vehicleAttrsJSON) > 0 {
			if err := json.Unmarshal(vehicleAttrsJSON, &driver.Vehicle); err != nil {
				return nil, fmt.Errorf("failed to unmarshal vehicle attrs: %w", err)
			}
		}

		drivers = append(drivers, &driver)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating driver rows: %w", err)
	}

	return drivers, nil
}

// CreateSession creates a new driver session
func (r *PostgresRepository) CreateSession(ctx context.Context, driverID string) (*domain.DriverSession, error) {
	query := `
		INSERT INTO driver_sessions (driver_id, started_at, total_rides, total_earnings)
		VALUES ($1, now(), 0, 0)
		RETURNING id, driver_id, started_at, total_rides, total_earnings
	`

	var session domain.DriverSession
	err := r.db.QueryRow(ctx, query, driverID).Scan(
		&session.ID,
		&session.DriverID,
		&session.StartedAt,
		&session.TotalRides,
		&session.TotalEarnings,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &session, nil
}

// GetActiveSession retrieves the active session for a driver
func (r *PostgresRepository) GetActiveSession(ctx context.Context, driverID string) (*domain.DriverSession, error) {
	query := `
		SELECT id, driver_id, started_at, ended_at, total_rides, total_earnings
		FROM driver_sessions
		WHERE driver_id = $1 AND ended_at IS NULL
		ORDER BY started_at DESC
		LIMIT 1
	`

	var session domain.DriverSession
	var endedAt sql.NullTime

	err := r.db.QueryRow(ctx, query, driverID).Scan(
		&session.ID,
		&session.DriverID,
		&session.StartedAt,
		&endedAt,
		&session.TotalRides,
		&session.TotalEarnings,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No active session
		}
		return nil, fmt.Errorf("failed to get active session: %w", err)
	}

	if endedAt.Valid {
		session.EndedAt = &endedAt.Time
	}

	return &session, nil
}

// EndSession ends a driver session
func (r *PostgresRepository) EndSession(ctx context.Context, sessionID string, totalRides int, totalEarnings float64) (*domain.DriverSession, error) {
	query := `
		UPDATE driver_sessions
		SET ended_at = now(), total_rides = $1, total_earnings = $2
		WHERE id = $3
		RETURNING id, driver_id, started_at, ended_at, total_rides, total_earnings
	`

	var session domain.DriverSession
	var endedAt sql.NullTime

	err := r.db.QueryRow(ctx, query, totalRides, totalEarnings, sessionID).Scan(
		&session.ID,
		&session.DriverID,
		&session.StartedAt,
		&endedAt,
		&session.TotalRides,
		&session.TotalEarnings,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		return nil, fmt.Errorf("failed to end session: %w", err)
	}

	if endedAt.Valid {
		session.EndedAt = &endedAt.Time
	}

	return &session, nil
}

// UpdateSessionStats updates session statistics
func (r *PostgresRepository) UpdateSessionStats(ctx context.Context, sessionID string, totalRides int, totalEarnings float64) error {
	query := `
		UPDATE driver_sessions
		SET total_rides = $1, total_earnings = $2
		WHERE id = $3
	`

	result, err := r.db.Exec(ctx, query, totalRides, totalEarnings, sessionID)
	if err != nil {
		return fmt.Errorf("failed to update session stats: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	return nil
}

// GetCurrentLocation retrieves the current location for a driver
func (r *PostgresRepository) GetCurrentLocation(ctx context.Context, driverID string) (*domain.Coordinate, error) {
	query := `
		SELECT id, created_at, updated_at, entity_id, entity_type,
		       address, latitude, longitude, fare_amount, distance_km, is_current
		FROM coordinates
		WHERE entity_id = $1 AND entity_type = 'driver' AND is_current = true
		LIMIT 1
	`

	var coord domain.Coordinate
	var fareAmount, distanceKm sql.NullFloat64

	err := r.db.QueryRow(ctx, query, driverID).Scan(
		&coord.ID,
		&coord.CreatedAt,
		&coord.UpdatedAt,
		&coord.EntityID,
		&coord.EntityType,
		&coord.Address,
		&coord.Latitude,
		&coord.Longitude,
		&fareAmount,
		&distanceKm,
		&coord.IsCurrent,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No current location
		}
		return nil, fmt.Errorf("failed to get current location: %w", err)
	}

	if fareAmount.Valid {
		val := fareAmount.Float64
		coord.FareAmount = &val
	}
	if distanceKm.Valid {
		val := distanceKm.Float64
		coord.DistanceKm = &val
	}

	return &coord, nil
}

// UpdateLocation updates a driver's current location
func (r *PostgresRepository) UpdateLocation(ctx context.Context, update *domain.LocationUpdate) (*domain.Coordinate, error) {
	// Start a transaction
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Archive old location
	archiveQuery := `
		UPDATE coordinates
		SET is_current = false, updated_at = now()
		WHERE entity_id = $1 AND entity_type = 'driver' AND is_current = true
	`
	_, err = tx.Exec(ctx, archiveQuery, update.DriverID)
	if err != nil {
		return nil, fmt.Errorf("failed to archive old location: %w", err)
	}

	// Insert new location
	insertQuery := `
		INSERT INTO coordinates (entity_id, entity_type, address, latitude, longitude, is_current)
		VALUES ($1, 'driver', '', $2, $3, true)
		RETURNING id, created_at, updated_at, entity_id, entity_type, address, latitude, longitude, is_current
	`

	var coord domain.Coordinate
	err = tx.QueryRow(ctx, insertQuery, update.DriverID, update.Latitude, update.Longitude).Scan(
		&coord.ID,
		&coord.CreatedAt,
		&coord.UpdatedAt,
		&coord.EntityID,
		&coord.EntityType,
		&coord.Address,
		&coord.Latitude,
		&coord.Longitude,
		&coord.IsCurrent,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to insert new location: %w", err)
	}

	// Save to location history
	historyQuery := `
		INSERT INTO location_history (coordinate_id, driver_id, latitude, longitude,
		                              accuracy_meters, speed_kmh, heading_degrees,
		                              recorded_at, ride_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err = tx.Exec(ctx, historyQuery,
		coord.ID,
		update.DriverID,
		update.Latitude,
		update.Longitude,
		update.AccuracyMeters,
		update.SpeedKmh,
		update.HeadingDegrees,
		update.Timestamp,
		update.RideID,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to save location history: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &coord, nil
}

// SaveLocationHistory saves location to history
func (r *PostgresRepository) SaveLocationHistory(ctx context.Context, history *domain.LocationHistory) error {
	query := `
		INSERT INTO location_history (coordinate_id, driver_id, latitude, longitude,
		                              accuracy_meters, speed_kmh, heading_degrees,
		                              recorded_at, ride_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.Exec(ctx, query,
		history.CoordinateID,
		history.DriverID,
		history.Latitude,
		history.Longitude,
		history.AccuracyMeters,
		history.SpeedKmh,
		history.HeadingDegrees,
		history.RecordedAt,
		history.RideID,
	)

	if err != nil {
		return fmt.Errorf("failed to save location history: %w", err)
	}

	return nil
}

// GetLocationHistory retrieves location history for a driver
func (r *PostgresRepository) GetLocationHistory(ctx context.Context, driverID string, rideID *string, limit int) ([]*domain.LocationHistory, error) {
	query := `
		SELECT id, coordinate_id, driver_id, latitude, longitude,
		       accuracy_meters, speed_kmh, heading_degrees, recorded_at, ride_id
		FROM location_history
		WHERE driver_id = $1
	`

	args := []interface{}{driverID}
	argCount := 1

	if rideID != nil {
		argCount++
		query += fmt.Sprintf(" AND ride_id = $%d", argCount)
		args = append(args, *rideID)
	}

	query += " ORDER BY recorded_at DESC"

	if limit > 0 {
		argCount++
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, limit)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query location history: %w", err)
	}
	defer rows.Close()

	var history []*domain.LocationHistory
	for rows.Next() {
		var h domain.LocationHistory
		var coordinateID, rID sql.NullString
		var accuracyMeters, speedKmh, headingDegrees sql.NullFloat64

		err := rows.Scan(
			&h.ID,
			&coordinateID,
			&h.DriverID,
			&h.Latitude,
			&h.Longitude,
			&accuracyMeters,
			&speedKmh,
			&headingDegrees,
			&h.RecordedAt,
			&rID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan location history row: %w", err)
		}

		if coordinateID.Valid {
			h.CoordinateID = &coordinateID.String
		}
		if accuracyMeters.Valid {
			val := accuracyMeters.Float64
			h.AccuracyMeters = &val
		}
		if speedKmh.Valid {
			val := speedKmh.Float64
			h.SpeedKmh = &val
		}
		if headingDegrees.Valid {
			val := headingDegrees.Float64
			h.HeadingDegrees = &val
		}
		if rID.Valid {
			h.RideID = &rID.String
		}

		history = append(history, &h)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating location history rows: %w", err)
	}

	return history, nil
}

// ArchiveOldLocation marks old coordinate as not current
func (r *PostgresRepository) ArchiveOldLocation(ctx context.Context, driverID string) error {
	query := `
		UPDATE coordinates
		SET is_current = false, updated_at = now()
		WHERE entity_id = $1 AND entity_type = 'driver' AND is_current = true
	`

	_, err := r.db.Exec(ctx, query, driverID)
	if err != nil {
		return fmt.Errorf("failed to archive old location: %w", err)
	}

	return nil
}
